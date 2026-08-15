package handler

import (
	"encoding/json"
	"net/http"

	"hotel-management/server/internal/db"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login 登录接口：校验用户名密码，返回会话令牌与用户信息。
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
		id       int64
		name     string
		hash     string
		status   int
		roleName string
	)
	err := pool.QueryRow(r.Context(),
		`SELECT u.id, COALESCE(u.name,''), u.password_hash, u.status, COALESCE(r.name,'')
		 FROM users u LEFT JOIN roles r ON r.id = u.role_id
		 WHERE u.username = $1`,
		req.Username,
	).Scan(&id, &name, &hash, &status, &roleName)
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

	writeJSON(w, http.StatusOK, map[string]any{
		"token": db.NewToken(),
		"user": map[string]any{
			"id":       id,
			"username": req.Username,
			"name":     name,
			"role":     roleName,
		},
	})
}
