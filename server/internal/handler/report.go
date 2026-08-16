package handler

import (
	"net/http"

	"hotel-management/server/internal/db"
)

// storeIDsOf 返回当前用户的数据权限门店集合。
// 管理员返回 nil（表示全部）；无任何权限返回空切片。
func storeIDsOf(r *http.Request) []int64 {
	u := currentUser(r)
	if u == nil || u.IsAdmin {
		return nil
	}
	return u.StoreIDs
}

// storeWhere 为「含 store_id 列」的查询构建门店过滤子句与参数。
// 管理员返回空子句；无权限门店返回 " AND FALSE"；有权限返回 " AND store_id = ANY($1)"。
func storeWhere(storeIDs []int64, col string) (clause string, args []any) {
	if storeIDs == nil {
		return "", nil
	}
	if len(storeIDs) == 0 {
		return " AND FALSE", nil
	}
	return " AND " + col + " = ANY($1)", []any{storeIDs}
}

// Dashboard 首页今日概况（按用户数据权限限定门店）。
func Dashboard(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	ctx := r.Context()

	storeIDs := storeIDsOf(r)
	if storeIDs != nil && len(storeIDs) == 0 {
		// 无任何门店权限：门店/房间/营收等指标归零（客户会员为集团级仍返回）
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
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM customer`).Scan(&d.Customers)
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM member`).Scan(&d.Members)
		writeJSON(w, http.StatusOK, map[string]any{"summary": d})
		return
	}

	// 门店过滤子句（有 store_id 列的表）
	storeClause, storeArgs := storeWhere(storeIDs, "store_id")
	// 门店表用 id 列过滤
	storeIDClause, storeIDArgs := storeWhere(storeIDs, "id")
	// 通过 check_in 关联门店（payment/folio 无 store_id 列）
	joinClause := ""
	joinArgs := []any{}
	if storeIDs != nil {
		joinClause = " AND c.store_id = ANY($1)"
		joinArgs = []any{storeIDs}
	}

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
		args []any
		dest any
	}{
		{`SELECT count(*) FROM store WHERE status=1` + storeIDClause, storeIDArgs, &d.Stores},
		{`SELECT count(*) FROM room WHERE 1=1` + storeClause, storeArgs, &d.Rooms},
		{`SELECT count(*) FROM check_in WHERE status=0` + storeClause, storeArgs, &d.Occupied},
		{`SELECT count(*) FROM room WHERE status=0` + storeClause, storeArgs, &d.CleanRooms},
		{`SELECT count(*) FROM check_in WHERE check_in_time::date = CURRENT_DATE` + storeClause, storeArgs, &d.TodayCheckin},
		{`SELECT count(*) FROM check_in WHERE status=1 AND updated_at::date = CURRENT_DATE` + storeClause, storeArgs, &d.TodayCheckout},
		{`SELECT count(*) FROM reservation WHERE status=0 AND check_in_date = CURRENT_DATE` + storeClause, storeArgs, &d.TodayReservation},
		{`SELECT COALESCE(sum(p.amount),0) FROM payment p JOIN folio f ON f.id = p.folio_id JOIN check_in c ON c.id = f.check_in_id WHERE p.pay_time::date = CURRENT_DATE` + joinClause, joinArgs, &d.TodayRevenue},
		{`SELECT COALESCE(sum(f.balance),0) FROM folio f JOIN check_in c ON c.id = f.check_in_id WHERE f.status=0` + joinClause, joinArgs, &d.PendingBalance},
		{`SELECT count(*) FROM customer`, nil, &d.Customers},
		{`SELECT count(*) FROM member`, nil, &d.Members},
	}

	for _, q := range queries {
		if err := pool.QueryRow(ctx, q.sql, q.args...).Scan(q.dest); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": d})
}

// RevenueReport 营收汇总（按门店，受数据权限约束）：今日营收 / 在住待收 / 累计营收 / 在住数。
func RevenueReport(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeIDs := storeIDsOf(r)
	clause, args := storeWhere(storeIDs, "s.id")

	query := `
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
		FROM store s WHERE 1=1` + clause + ` ORDER BY s.id`

	rows, err := pool.Query(r.Context(), query, args...)
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

// TrendReport 近 14 天营收与入住趋势（受数据权限约束，供首页折线图）。
func TrendReport(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeIDs := storeIDsOf(r)
	// check_in 直接按 store_id 过滤；payment 通过 check_in 关联过滤
	ciClause, _ := storeWhere(storeIDs, "c.store_id")
	payClause := ""
	if storeIDs != nil {
		if len(storeIDs) == 0 {
			payClause = " AND FALSE"
		} else {
			payClause = " AND c.store_id = ANY($1)"
		}
	}

	query := `
		SELECT to_char(d::date, 'YYYY-MM-DD') AS date,
			COALESCE((SELECT sum(p.amount) FROM payment p
				JOIN folio f ON f.id = p.folio_id
				JOIN check_in c ON c.id = f.check_in_id
				WHERE p.pay_time::date = d::date` + payClause + `), 0) AS revenue,
			COALESCE((SELECT count(*) FROM check_in c WHERE c.check_in_time::date = d::date` + ciClause + `), 0) AS checkins,
			COALESCE((SELECT count(*) FROM check_in c WHERE c.status = 1 AND c.updated_at::date = d::date` + ciClause + `), 0) AS checkouts
		FROM generate_series(CURRENT_DATE - 13, CURRENT_DATE, interval '1 day') AS d
		ORDER BY d::date`

	// 同一 $1 参数（storeIDs）在多个子查询中复用。
	var args []any
	if storeIDs != nil && len(storeIDs) > 0 {
		args = append(args, storeIDs)
	}

	rows, err := pool.Query(r.Context(), query, args...)
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

// OccupancyReport 房态分布与入住率（按门店，受数据权限约束）。
func OccupancyReport(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeIDs := storeIDsOf(r)
	clause, args := storeWhere(storeIDs, "s.id")

	query := `
		SELECT s.id, s.name,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id) AS total,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id AND r.status = 2) AS occupied,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id AND r.status = 0) AS clean,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id AND r.status = 1) AS dirty,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id AND r.status = 3) AS maint,
			(SELECT count(*) FROM room r WHERE r.store_id = s.id AND r.status = 4) AS reserved
		FROM store s WHERE 1=1` + clause + ` ORDER BY s.id`

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type item struct {
		StoreID   int64   `json:"store_id"`
		StoreName string  `json:"store_name"`
		Total     int64   `json:"total"`
		Occupied  int64   `json:"occupied"`
		Clean     int64   `json:"clean"`
		Dirty     int64   `json:"dirty"`
		Maint     int64   `json:"maint"`
		Reserved  int64   `json:"reserved"`
		Occupancy float64 `json:"occupancy"`
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
