package handler

import (
	"net/http"

	"hotel-management/server/internal/db"
)

// ListStores 门店列表接口（按用户数据权限返回门店；管理员可见全部）。
func ListStores(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	type store struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Address string `json:"address"`
		Phone   string `json:"phone"`
		Manager string `json:"manager"`
		Status  int    `json:"status"`
	}

	query := `SELECT id, name, COALESCE(address,''), COALESCE(phone,''), COALESCE(manager,''), status
	          FROM store`
	args := []any{}
	if u := currentUser(r); u != nil && !u.IsAdmin {
		if len(u.StoreIDs) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{"stores": []store{}, "total": 0})
			return
		}
		query += ` WHERE id = ANY($1)`
		args = append(args, u.StoreIDs)
	}
	query += ` ORDER BY id`

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := make([]store, 0)
	for rows.Next() {
		var s store
		if err := rows.Scan(&s.ID, &s.Name, &s.Address, &s.Phone, &s.Manager, &s.Status); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stores": list, "total": len(list)})
}
