package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"hotel-management/server/internal/db"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// 鉴权上下文键与用户信息。
type contextKey string

const authUserKey contextKey = "authUser"

// AuthUser 鉴权后注入到请求上下文的当前用户。
type AuthUser struct {
	ID        int64
	Username  string
	Name      string
	Role      string
	RoleLevel int
	IsAdmin   bool    // 集团管理员（level>=9），不受门店数据限制
	StoreIDs  []int64 // 数据权限门店；IsAdmin 时为 nil 表示全部
}

// currentUser 从请求上下文取当前用户（未鉴权返回 nil）。
func currentUser(r *http.Request) *AuthUser {
	u, _ := r.Context().Value(authUserKey).(*AuthUser)
	return u
}

// canAccessStore 判断用户能否访问指定门店。
func (u *AuthUser) canAccessStore(storeID int64) bool {
	if u == nil || u.IsAdmin {
		return true
	}
	for _, id := range u.StoreIDs {
		if id == storeID {
			return true
		}
	}
	return false
}

// storeCond 构建门店级列表查询的过滤条件（不含 WHERE/AND 关键字）。
// col：门店列名（如 "store_id" 或 "r.store_id"）；storeID：请求指定的门店（0=未指定）。
// 返回 cond（可为空；"FALSE" 表示无任何权限）、args；forbidden=true 表示无权访问指定门店。
func storeCond(r *http.Request, storeID int64, col string) (cond string, args []any, forbidden bool) {
	u := currentUser(r)
	if storeID > 0 {
		if u != nil && !u.IsAdmin && !u.canAccessStore(storeID) {
			return "", nil, true
		}
		return col + " = $1", []any{storeID}, false
	}
	if u != nil && !u.IsAdmin {
		if len(u.StoreIDs) == 0 {
			return "FALSE", nil, false
		}
		return col + " = ANY($1)", []any{u.StoreIDs}, false
	}
	return "", nil, false
}

// bearerToken 从 Authorization 头提取 Bearer 令牌。
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// isPublicPath 公开接口（无需登录）。
func isPublicPath(path string) bool {
	switch path {
	case "/health", "/api/v1/ping", "/api/v1/auth/login":
		return true
	}
	return false
}

// AuthMiddleware 鉴权中间件：校验令牌，解析用户及其门店数据权限，注入上下文。
// 公开接口直接放行；其余 /api/v1/* 接口必须携带有效令牌。
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		token := bearerToken(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		pool := db.Pool()
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
			return
		}

		uid, err := db.GetSessionUser(r.Context(), pool, token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "会话无效或已过期，请重新登录"})
			return
		}

		// 查询用户 + 角色（同时校验账号未被禁用）
		var (
			u      AuthUser
			status int
		)
		if err := pool.QueryRow(r.Context(),
			`SELECT u.username, COALESCE(u.name,''), COALESCE(r.name,''), COALESCE(r.level,0), u.status
			 FROM users u LEFT JOIN roles r ON r.id = u.role_id
			 WHERE u.id = $1`, uid,
		).Scan(&u.Username, &u.Name, &u.Role, &u.RoleLevel, &status); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "用户不存在"})
			return
		}
		if status != 1 {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "账号已禁用"})
			return
		}
		u.ID = uid
		u.IsAdmin = u.RoleLevel >= 9

		// 查询数据权限门店
		u.StoreIDs = make([]int64, 0)
		rows, err := pool.Query(r.Context(),
			`SELECT store_id FROM user_store WHERE user_id = $1 ORDER BY store_id`, uid,
		)
		if err == nil {
			for rows.Next() {
				var sid int64
				if rows.Scan(&sid) == nil {
					u.StoreIDs = append(u.StoreIDs, sid)
				}
			}
			rows.Close()
		}

		ctx := context.WithValue(r.Context(), authUserKey, &u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Login 登录接口：校验用户名密码，写入会话并返回令牌与用户权限信息。
func Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求体格式错误"})
		return
	}
	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "用户名和密码不能为空"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	var (
		id        int64
		name      string
		hash      string
		status    int
		roleName  string
		roleLevel int
	)
	err := pool.QueryRow(r.Context(),
		`SELECT u.id, COALESCE(u.name,''), u.password_hash, u.status, COALESCE(r.name,''), COALESCE(r.level,0)
		 FROM users u LEFT JOIN roles r ON r.id = u.role_id
		 WHERE u.username = $1`,
		req.Username,
	).Scan(&id, &name, &hash, &status, &roleName, &roleLevel)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
		return
	}
	if status != 1 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "账号已禁用"})
		return
	}
	if db.HashPassword(req.Password) != hash {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
		return
	}

	// 写入会话（有效期 24 小时）
	token, err := db.CreateSession(r.Context(), pool, id, 24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "创建会话失败: " + err.Error()})
		return
	}

	// 查询数据权限门店
	storeIDs := make([]int64, 0)
	if roleLevel < 9 {
		rows, qerr := pool.Query(r.Context(),
			`SELECT store_id FROM user_store WHERE user_id = $1 ORDER BY store_id`, id,
		)
		if qerr == nil {
			for rows.Next() {
				var sid int64
				if rows.Scan(&sid) == nil {
					storeIDs = append(storeIDs, sid)
				}
			}
			rows.Close()
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user": map[string]any{
			"id":         id,
			"username":   req.Username,
			"name":       name,
			"role":       roleName,
			"role_level": roleLevel,
			"is_admin":   roleLevel >= 9,
			"store_ids":  storeIDs,
		},
	})
}

// Logout 登出接口：删除当前会话。
func Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	token := bearerToken(r)
	if token != "" {
		if pool := db.Pool(); pool != nil {
			_ = db.DeleteSession(r.Context(), pool, token)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已登出"})
}
