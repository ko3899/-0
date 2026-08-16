package handler

import (
	"net/http"
	"strings"

	"hotel-management/server/internal/db"
)

// requireAdmin 校验当前用户是否为集团管理员，非管理员返回 false 并写回 403。
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	u := currentUser(r)
	if u == nil || !u.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "仅集团管理员可操作"})
		return false
	}
	return true
}

// ListRoles 角色列表（登录用户可看，用于用户管理下拉）。
func ListRoles(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	rows, err := pool.Query(r.Context(),
		`SELECT id, name, level FROM roles ORDER BY level DESC, id`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type role struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Level int    `json:"level"`
	}
	list := make([]role, 0)
	for rows.Next() {
		var rl role
		if err := rows.Scan(&rl.ID, &rl.Name, &rl.Level); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, rl)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": list, "total": len(list)})
}

// ListUsers 用户列表（仅管理员），含角色与数据权限门店。
func ListUsers(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	rows, err := pool.Query(r.Context(), `
		SELECT u.id, u.username, COALESCE(u.name,''), COALESCE(r.id,0), COALESCE(r.name,''), u.status,
		       COALESCE(array_agg(us.store_id) FILTER (WHERE us.store_id IS NOT NULL), '{}')
		FROM users u
		LEFT JOIN roles r ON r.id = u.role_id
		LEFT JOIN user_store us ON us.user_id = u.id
		GROUP BY u.id, u.username, u.name, r.id, r.name, u.status
		ORDER BY u.id`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type user struct {
		ID       int64   `json:"id"`
		Username string  `json:"username"`
		Name     string  `json:"name"`
		RoleID   int64   `json:"role_id"`
		RoleName string  `json:"role_name"`
		Status   int     `json:"status"`
		StoreIDs []int64 `json:"store_ids"`
	}
	list := make([]user, 0)
	for rows.Next() {
		var it user
		if err := rows.Scan(&it.ID, &it.Username, &it.Name, &it.RoleID, &it.RoleName, &it.Status, &it.StoreIDs); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": list, "total": len(list)})
}

// CreateUser 新增用户（仅管理员）：用户 + 数据权限门店。
func CreateUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req struct {
		Username  string  `json:"username"`
		Password  string  `json:"password"`
		Name      string  `json:"name"`
		RoleID    int64   `json:"role_id"`
		StoreIDs  []int64 `json:"store_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Name = strings.TrimSpace(req.Name)
	if req.Username == "" || req.Password == "" || req.RoleID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "用户名/密码/角色不能为空"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	tx, err := pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer tx.Rollback(r.Context())

	var userID int64
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO users (username, password_hash, name, role_id, status)
		 VALUES ($1, $2, $3, $4, 1) RETURNING id`,
		req.Username, db.HashPassword(req.Password), req.Name, req.RoleID,
	).Scan(&userID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "用户名已存在或参数错误"})
		return
	}

	for _, sid := range req.StoreIDs {
		if sid > 0 {
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO user_store (user_id, store_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				userID, sid,
			); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": userID})
}

// UpdateUser 编辑用户（仅管理员）：姓名/角色/状态/密码/门店权限。
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	userID := pathID(r.URL.Path)
	if userID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少用户 ID"})
		return
	}

	var req struct {
		Name     string  `json:"name"`
		RoleID   int64   `json:"role_id"`
		Status   *int    `json:"status"`
		Password string  `json:"password"`
		StoreIDs []int64 `json:"store_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	tx, err := pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer tx.Rollback(r.Context())

	// 基础信息
	if req.Name != "" || req.RoleID > 0 || req.Status != nil {
		var (
			name   string
			roleID int64
			status int
		)
		// 先取现有值，未传的字段保持原值
		if err := tx.QueryRow(r.Context(),
			`SELECT COALESCE(name,''), COALESCE(role_id,0), status FROM users WHERE id = $1 FOR UPDATE`, userID,
		).Scan(&name, &roleID, &status); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "用户不存在"})
			return
		}
		if req.Name != "" {
			name = req.Name
		}
		if req.RoleID > 0 {
			roleID = req.RoleID
		}
		if req.Status != nil {
			status = *req.Status
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE users SET name = $1, role_id = $2, status = $3, updated_at = now() WHERE id = $4`,
			name, roleID, status, userID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	// 密码（可选）
	if req.Password != "" {
		if _, err := tx.Exec(r.Context(),
			`UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`,
			db.HashPassword(req.Password), userID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	// 门店权限（全量替换）
	if req.StoreIDs != nil {
		if _, err := tx.Exec(r.Context(),
			`DELETE FROM user_store WHERE user_id = $1`, userID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		for _, sid := range req.StoreIDs {
			if sid > 0 {
				if _, err := tx.Exec(r.Context(),
					`INSERT INTO user_store (user_id, store_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
					userID, sid,
				); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
			}
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": userID, "ok": true})
}
