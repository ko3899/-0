package handler

import (
	"net/http"

	"hotel-management/server/internal/db"
)

// ListMembers 会员列表（关联客户，支持姓名/手机/会员号搜索）。
func ListMembers(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	keyword := r.URL.Query().Get("keyword")
	query := `
		SELECT m.id, m.customer_id, m.member_no, m.level, m.points, m.balance,
			COALESCE(m.join_date::text, ''), c.name, COALESCE(c.phone, ''), COALESCE(c.id_no, '')
		FROM member m JOIN customer c ON c.id = m.customer_id`
	args := make([]any, 0, 1)
	if keyword != "" {
		query += ` WHERE c.name ILIKE '%' || $1 || '%' OR c.phone LIKE '%' || $1 || '%' OR m.member_no LIKE '%' || $1 || '%'`
		args = append(args, keyword)
	}
	query += ` ORDER BY m.id`

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type member struct {
		ID         int64   `json:"id"`
		CustomerID int64   `json:"customer_id"`
		MemberNo   string  `json:"member_no"`
		Level      int     `json:"level"`
		Points     int     `json:"points"`
		Balance    float64 `json:"balance"`
		JoinDate   string  `json:"join_date"`
		Name       string  `json:"name"`
		Phone      string  `json:"phone"`
		IDNo       string  `json:"id_no"`
	}
	list := make([]member, 0)
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.ID, &m.CustomerID, &m.MemberNo, &m.Level, &m.Points, &m.Balance, &m.JoinDate, &m.Name, &m.Phone, &m.IDNo); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, m)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": list, "total": len(list)})
}

// RechargeMember 会员储值充值（余额累加）。
func RechargeMember(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	id := pathID(r.URL.Path)
	var req struct {
		Amount float64 `json:"amount"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "充值金额必须大于 0"})
		return
	}

	var balance float64
	if err := pool.QueryRow(r.Context(),
		`UPDATE member SET balance = balance + $2, updated_at = now() WHERE id = $1 RETURNING balance`,
		id, req.Amount,
	).Scan(&balance); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "balance": balance})
}

// AdjustMemberPoints 会员积分调整（delta 可正可负）。
func AdjustMemberPoints(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	id := pathID(r.URL.Path)
	var req struct {
		Delta int `json:"delta"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	var points int
	if err := pool.QueryRow(r.Context(),
		`UPDATE member SET points = points + $2, updated_at = now() WHERE id = $1 RETURNING points`,
		id, req.Delta,
	).Scan(&points); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "points": points})
}
