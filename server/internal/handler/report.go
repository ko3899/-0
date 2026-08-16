package handler

import (
	"net/http"

	"hotel-management/server/internal/db"
)

// Dashboard 首页今日概况：门店/房间/在住/今日入住退房/今日营收/待收款/客户会员。
func Dashboard(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	ctx := r.Context()

	var d struct {
		Stores           int64   `json:"stores"`
		Rooms            int64   `json:"rooms"`
		Occupied         int64   `json:"occupied"`
		CleanRooms       int64   `json:"clean_rooms"`
		TodayCheckin     int64   `json:"today_checkin"`
		TodayCheckout    int64   `json:"today_checkout"`
		TodayReservation int64   `json:"today_reservation"`
		TodayRevenue     float64 `json:"today_revenue"`
		PendingBalance   float64 `json:"pending_balance"`
		Customers        int64   `json:"customers"`
		Members          int64   `json:"members"`
	}

	queries := []struct {
		sql  string
		dest any
	}{
		{`SELECT count(*) FROM store WHERE status=1`, &d.Stores},
		{`SELECT count(*) FROM room`, &d.Rooms},
		{`SELECT count(*) FROM check_in WHERE status=0`, &d.Occupied},
		{`SELECT count(*) FROM room WHERE status=0`, &d.CleanRooms},
		{`SELECT count(*) FROM check_in WHERE check_in_time::date = CURRENT_DATE`, &d.TodayCheckin},
		{`SELECT count(*) FROM check_in WHERE status=1 AND updated_at::date = CURRENT_DATE`, &d.TodayCheckout},
		{`SELECT count(*) FROM reservation WHERE status=0 AND check_in_date = CURRENT_DATE`, &d.TodayReservation},
		{`SELECT COALESCE(sum(amount),0) FROM payment WHERE pay_time::date = CURRENT_DATE`, &d.TodayRevenue},
		{`SELECT COALESCE(sum(balance),0) FROM folio WHERE status=0`, &d.PendingBalance},
		{`SELECT count(*) FROM customer`, &d.Customers},
		{`SELECT count(*) FROM member`, &d.Members},
	}

	for _, q := range queries {
		if err := pool.QueryRow(ctx, q.sql).Scan(q.dest); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": d})
}

// RevenueReport 营收汇总（按门店）：今日营收 / 在住待收 / 累计营收 / 在住数。
func RevenueReport(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	rows, err := pool.Query(r.Context(), `
		SELECT s.id, s.name,
			COALESCE((SELECT sum(p.amount) FROM payment p
				JOIN folio f ON f.id = p.folio_id
				JOIN check_in c ON c.id = f.check_in_id
				WHERE c.store_id = s.id AND p.pay_time::date = CURRENT_DATE), 0) AS today_revenue,
			COALESCE((SELECT sum(f.balance) FROM folio f
				JOIN check_in c ON c.id = f.check_in_id
				WHERE c.store_id = s.id AND f.status = 0), 0) AS pending_balance,
			COALESCE((SELECT sum(p.amount) FROM payment p
				JOIN folio f ON f.id = p.folio_id
				JOIN check_in c ON c.id = f.check_in_id
				WHERE c.store_id = s.id), 0) AS total_revenue,
			(SELECT count(*) FROM check_in c WHERE c.store_id = s.id AND c.status = 0) AS in_house
		FROM store s ORDER BY s.id`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type item struct {
		StoreID        int64   `json:"store_id"`
		StoreName      string  `json:"store_name"`
		TodayRevenue   float64 `json:"today_revenue"`
		PendingBalance float64 `json:"pending_balance"`
		TotalRevenue   float64 `json:"total_revenue"`
		InHouse        int64   `json:"in_house"`
	}
	list := make([]item, 0)
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.StoreID, &it.StoreName, &it.TodayRevenue, &it.PendingBalance, &it.TotalRevenue, &it.InHouse); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list, "total": len(list)})
}

// TrendReport 近 14 天营收与入住趋势（供首页折线图）。
func TrendReport(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	rows, err := pool.Query(r.Context(), `
		SELECT to_char(d::date, 'YYYY-MM-DD') AS date,
			COALESCE((SELECT sum(p.amount) FROM payment p WHERE p.pay_time::date = d::date), 0) AS revenue,
			COALESCE((SELECT count(*) FROM check_in c WHERE c.check_in_time::date = d::date), 0) AS checkins,
			COALESCE((SELECT count(*) FROM check_in c WHERE c.status = 1 AND c.updated_at::date = d::date), 0) AS checkouts
		FROM generate_series(CURRENT_DATE - 13, CURRENT_DATE, interval '1 day') AS d
		ORDER BY d::date`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type item struct {
		Date      string  `json:"date"`
		Revenue   float64 `json:"revenue"`
		Checkins  int64   `json:"checkins"`
		Checkouts int64   `json:"checkouts"`
	}
	list := make([]item, 0, 14)
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.Date, &it.Revenue, &it.Checkins, &it.Checkouts); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list, "total": len(list)})
}

// OccupancyReport 房态分布与入住率（按门店）。
func OccupancyReport(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	rows, err := pool.Query(r.Context(), `
		SELECT s.id, s.name,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id) AS total,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id AND r.status = 2) AS occupied,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id AND r.status = 0) AS clean,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id AND r.status = 1) AS dirty,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id AND r.status = 3) AS maint,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id AND r.status = 4) AS reserved
		FROM store s ORDER BY s.id`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type item struct {
		StoreID    int64   `json:"store_id"`
		StoreName  string  `json:"store_name"`
		Total      int64   `json:"total"`
		Occupied   int64   `json:"occupied"`
		Clean      int64   `json:"clean"`
		Dirty      int64   `json:"dirty"`
		Maint      int64   `json:"maint"`
		Reserved   int64   `json:"reserved"`
		Occupancy  float64 `json:"occupancy"`
	}
	list := make([]item, 0)
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.StoreID, &it.StoreName, &it.Total, &it.Occupied, &it.Clean, &it.Dirty, &it.Maint, &it.Reserved); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if it.Total > 0 {
			it.Occupancy = float64(it.Occupied) / float64(it.Total) * 100
		}
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list, "total": len(list)})
}
