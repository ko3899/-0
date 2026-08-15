package handler

import (
	"net/http"
	"strings"

	"hotel-management/server/internal/db"
)

// ListCustomers 客户档案列表接口，支持 keyword 模糊搜索（姓名/手机号）。
func ListCustomers(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	query := `SELECT c.id, c.name, COALESCE(c.gender,0), COALESCE(c.id_type,''), COALESCE(c.id_no,''),
	                 COALESCE(c.phone,''), COALESCE(c.tags,''), COALESCE(m.member_no,''), COALESCE(m.level,0), COALESCE(m.points,0)
	          FROM customer c
	          LEFT JOIN member m ON m.customer_id = c.id`
	args := []any{}
	if keyword != "" {
		query += ` WHERE c.name ILIKE $1 OR c.phone ILIKE $1`
		args = append(args, "%"+keyword+"%")
	}
	query += ` ORDER BY c.id DESC LIMIT 200`

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type customer struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Gender   int    `json:"gender"`
		IDType   string `json:"id_type"`
		IDNo     string `json:"id_no"`
		Phone    string `json:"phone"`
		Tags     string `json:"tags"`
		MemberNo string `json:"member_no"`
		Level    int    `json:"level"`
		Points   int    `json:"points"`
	}
	list := make([]customer, 0)
	for rows.Next() {
		var c customer
		if err := rows.Scan(&c.ID, &c.Name, &c.Gender, &c.IDType, &c.IDNo, &c.Phone, &c.Tags, &c.MemberNo, &c.Level, &c.Points); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"customers": list, "total": len(list)})
}

// CreateCustomer 新建客户档案接口（按手机号去重）。
func CreateCustomer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Name   string `json:"name"`
		Gender int    `json:"gender"`
		IDType string `json:"id_type"`
		IDNo   string `json:"id_no"`
		Phone  string `json:"phone"`
		Tags   string `json:"tags"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "客户姓名不能为空"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	// 按手机号去重（手机号为空则不去重）
	if req.Phone != "" {
		var existID int64
		err := pool.QueryRow(r.Context(),
			`SELECT id FROM customer WHERE phone = $1`, req.Phone,
		).Scan(&existID)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"id": existID, "existed": true})
			return
		}
	}

	var id int64
	if err := pool.QueryRow(r.Context(),
		`INSERT INTO customer (name, gender, id_type, id_no, phone, tags)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		req.Name, req.Gender, req.IDType, req.IDNo, req.Phone, req.Tags,
	).Scan(&id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "existed": false})
}
