package handler

import (
	"net/http"
	"time"

	"hotel-management/server/internal/db"
)

// ListRatePlans 房价方案列表（可限定门店）。
func ListRatePlans(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeID := queryInt64(r, "store_id")
	query := `SELECT id, store_id, name, type, status FROM rate_plan`
	args := make([]any, 0, 1)
	cond, scopeArgs, forbidden := storeCond(r, storeID, "store_id")
	if forbidden {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
		return
	}
	if cond != "" {
		query += " WHERE " + cond
		args = append(args, scopeArgs...)
	}
	query += ` ORDER BY store_id, id`

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type plan struct {
		ID      int64  `json:"id"`
		StoreID int64  `json:"store_id"`
		Name    string `json:"name"`
		Type    string `json:"type"`
		Status  int    `json:"status"`
	}
	list := make([]plan, 0)
	for rows.Next() {
		var p plan
		if err := rows.Scan(&p.ID, &p.StoreID, &p.Name, &p.Type, &p.Status); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, p)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": list, "total": len(list)})
}

// ListRateCalendar 房价日历（按门店 + 日期范围，默认今天起 7 天）。
func ListRateCalendar(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeID := queryInt64(r, "store_id")
	if storeID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少 store_id 参数"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
		return
	}
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	if start == "" {
		start = time.Now().Format("2006-01-02")
	}
	if end == "" {
		end = time.Now().AddDate(0, 0, 6).Format("2006-01-02")
	}

	rows, err := pool.Query(r.Context(), `
		SELECT rc.id, rc.store_id, rc.room_type_id, rt.name, rc.rate_plan_id, rp.name, rc.biz_date, rc.price
		FROM rate_calendar rc
		JOIN room_type rt ON rt.id = rc.room_type_id
		JOIN rate_plan rp ON rp.id = rc.rate_plan_id
		WHERE rc.store_id = $1 AND rc.biz_date BETWEEN $2::date AND $3::date
		ORDER BY rc.biz_date, rc.room_type_id, rc.rate_plan_id`,
		storeID, start, end,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type cal struct {
		ID           int64   `json:"id"`
		StoreID      int64   `json:"store_id"`
		RoomTypeID   int64   `json:"room_type_id"`
		RoomTypeName string  `json:"room_type_name"`
		RatePlanID   int64   `json:"rate_plan_id"`
		RatePlanName string  `json:"rate_plan_name"`
		BizDate      string  `json:"biz_date"`
		Price        float64 `json:"price"`
	}
	list := make([]cal, 0)
	for rows.Next() {
		var c cal
		var bizDate time.Time
		if err := rows.Scan(&c.ID, &c.StoreID, &c.RoomTypeID, &c.RoomTypeName, &c.RatePlanID, &c.RatePlanName, &bizDate, &c.Price); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.BizDate = bizDate.Format("2006-01-02")
		list = append(list, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list, "total": len(list)})
}

// UpdateRateCalendar 修改某天房价（upsert + 价格审计日志）。
func UpdateRateCalendar(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	var req struct {
		StoreID    int64   `json:"store_id"`
		RoomTypeID int64   `json:"room_type_id"`
		RatePlanID int64   `json:"rate_plan_id"`
		BizDate    string  `json:"biz_date"`
		Price      float64 `json:"price"`
		OperatorID int64   `json:"operator_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.StoreID <= 0 || req.RoomTypeID <= 0 || req.RatePlanID <= 0 || req.BizDate == "" || req.Price <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "参数不完整（需 store_id/room_type_id/rate_plan_id/biz_date/price）"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(req.StoreID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	// 查询旧价（无记录则 old=0）
	var oldPrice float64
	_ = pool.QueryRow(r.Context(),
		`SELECT price FROM rate_calendar WHERE store_id=$1 AND room_type_id=$2 AND rate_plan_id=$3 AND biz_date=$4::date`,
		req.StoreID, req.RoomTypeID, req.RatePlanID, req.BizDate,
	).Scan(&oldPrice)

	// upsert 新价
	if _, err := pool.Exec(r.Context(), `
		INSERT INTO rate_calendar (store_id, room_type_id, rate_plan_id, biz_date, price)
		VALUES ($1, $2, $3, $4::date, $5)
		ON CONFLICT (store_id, room_type_id, rate_plan_id, biz_date)
		DO UPDATE SET price = EXCLUDED.price, updated_at = now()`,
		req.StoreID, req.RoomTypeID, req.RatePlanID, req.BizDate, req.Price,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 价格审计（失败不阻断主流程）
	opID := req.OperatorID
	if opID <= 0 {
		opID = 1 // 演示环境默认 admin
	}
	_, _ = pool.Exec(r.Context(),
		`INSERT INTO price_change_log (store_id, operator_id, room_type_id, old_price, new_price)
		 VALUES ($1, $2, $3, $4, $5)`,
		req.StoreID, opID, req.RoomTypeID, oldPrice, req.Price,
	)

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "old_price": oldPrice, "new_price": req.Price})
}
