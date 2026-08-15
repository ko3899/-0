package handler

import (
	"net/http"

	"hotel-management/server/internal/db"
)

// ListStores 门店列表接口（总部视角可看全部门店）。
func ListStores(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	rows, err := pool.Query(r.Context(),
		`SELECT id, name, COALESCE(address,''), COALESCE(phone,''), COALESCE(manager,''), status
		 FROM store ORDER BY id`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type store struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Address string `json:"address"`
		Phone   string `json:"phone"`
		Manager string `json:"manager"`
		Status  int    `json:"status"`
	}
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
