package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"hotel-management/server/internal/db"
)

// ============================================================
// 夜审（Night Audit）
// 营业日规则：当前应审营业日 = 上次已完成夜审日的次日；从未夜审则为今天；不超过今天。
// 核心动作：
//   1. 对在住且已超期（预计退房日期 < 当前营业日）的账单，补过账 1 晚房费（幂等，按 biz_date 去重）。
//   2. 汇总当日营收/入住/退房/在住，生成夜审报表。
//   3. 检测异常：超期未退房、账单余额未结。
//   4. 写入 night_audit_log，锁定该营业日（每营业日仅一次）。
// ============================================================

// currentBizDay 计算当前应夜审的营业日。
func currentBizDay(ctx context.Context, pool *pgxpool.Pool) (time.Time, error) {
	var lastAudited *time.Time
	if err := pool.QueryRow(ctx, `SELECT MAX(biz_date) FROM night_audit_log WHERE status = 1`).Scan(&lastAudited); err != nil {
		return time.Now(), err
	}
	today := time.Now()
	if lastAudited == nil {
		return today, nil
	}
	next := lastAudited.AddDate(0, 0, 1)
	if next.After(today) {
		return today, nil
	}
	return next, nil
}

// NightAuditCurrent 当前营业日状态（GET /api/v1/night-audit/current）。
func NightAuditCurrent(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	ctx := r.Context()

	bizDay, err := currentBizDay(ctx, pool)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var audited bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM night_audit_log WHERE biz_date = $1 AND status = 1)`, bizDay).Scan(&audited)

	var lastAuditAt *time.Time
	var lastAuditBy string
	_ = pool.QueryRow(ctx, `SELECT completed_at, COALESCE(u.name,'') FROM night_audit_log n LEFT JOIN users u ON u.id = n.started_by WHERE n.status = 1 ORDER BY n.biz_date DESC LIMIT 1`).Scan(&lastAuditAt, &lastAuditBy)

	resp := map[string]any{
		"biz_date":      bizDay.Format("2006-01-02"),
		"audited":       audited,
		"today":         time.Now().Format("2006-01-02"),
		"last_audit_by": lastAuditBy,
		"last_audit_at": nil,
	}
	if lastAuditAt != nil {
		resp["last_audit_at"] = lastAuditAt.Format("2006-01-02 15:04:05")
	}
	writeJSON(w, http.StatusOK, resp)
}

// NightAuditPreview 夜审预览（GET /api/v1/night-audit/preview）。
// 不实际过账，仅展示将要补过账的房间、金额、异常清单。
func NightAuditPreview(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	ctx := r.Context()

	bizDay, err := currentBizDay(ctx, pool)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var audited bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM night_audit_log WHERE biz_date = $1 AND status = 1)`, bizDay).Scan(&audited)
	if audited {
		writeJSON(w, http.StatusConflict, map[string]string{"error": fmt.Sprintf("营业日 %s 已完成夜审，不可重复", bizDay.Format("2006-01-02"))})
		return
	}

	preview, err := buildAuditPreview(ctx, pool, r, bizDay)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

// buildAuditPreview 构造夜审预览数据：在住清单、待补过账、异常、汇总。
func buildAuditPreview(ctx context.Context, pool *pgxpool.Pool, r *http.Request, bizDay time.Time) (map[string]any, error) {
	storeIDs := storeIDsOf(r)

	// bizDay 始终是 $1，门店过滤（若有）用 $2
	ciClause, ciArgs := storeClauseAt(storeIDs, "ci.store_id", 2)
	revClause, revArgs := storeClauseAt(storeIDs, "c.store_id", 2)

	// 在住清单 + 是否超期 + 当日门市价 + 是否已补过账
	queryArgs := []any{bizDay}
	queryArgs = append(queryArgs, ciArgs...)
	rows, err := pool.Query(ctx, `
		SELECT ci.id, ci.store_id, s.name, r.room_no, c.name, ci.check_in_time, ci.expected_checkout_time,
		       ci.deposit, f.id, f.total_amount, f.paid_amount, f.balance,
		       COALESCE(rc.price, 0) AS today_price,
		       EXISTS(SELECT 1 FROM folio_item fi WHERE fi.folio_id = f.id AND fi.item_type = 'room_fee' AND fi.biz_date = $1) AS posted
		FROM check_in ci
		JOIN store s ON s.id = ci.store_id
		JOIN room r ON r.id = ci.room_id
		JOIN customer c ON c.id = ci.customer_id
		JOIN folio f ON f.check_in_id = ci.id
		LEFT JOIN room_type rt ON rt.id = r.room_type_id
		LEFT JOIN rate_plan rp ON rp.store_id = ci.store_id AND rp.type = 'rack'
		LEFT JOIN rate_calendar rc ON rc.room_type_id = rt.id AND rc.rate_plan_id = rp.id AND rc.biz_date = $1
		WHERE ci.status = 0`+ciClause, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type inHouseItem struct {
		CheckInID   int64   `json:"check_in_id"`
		StoreID     int64   `json:"store_id"`
		StoreName   string  `json:"store_name"`
		RoomNo      string  `json:"room_no"`
		GuestName   string  `json:"guest_name"`
		CheckInAt   string  `json:"check_in_at"`
		ExpectedOut string  `json:"expected_checkout"`
		Overstay    bool    `json:"overstay"`
		TodayPrice  float64 `json:"today_price"`
		AlreadyPost bool    `json:"already_posted"`
		Balance     float64 `json:"balance"`
	}

	var inHouse = make([]inHouseItem, 0)
	var toPost = make([]inHouseItem, 0)
	var overdue = make([]inHouseItem, 0)
	var unpaid = make([]inHouseItem, 0)

	bizDayDate := bizDay.Format("2006-01-02")
	for rows.Next() {
		var it inHouseItem
		var ciTime, expOut time.Time
		var deposit, total, paid, balance, price float64
		var folioID int64
		var posted bool
		if err := rows.Scan(&it.CheckInID, &it.StoreID, &it.StoreName, &it.RoomNo, &it.GuestName,
			&ciTime, &expOut, &deposit, &folioID, &total, &paid, &balance, &price, &posted); err != nil {
			return nil, err
		}
		it.CheckInAt = ciTime.Format("2006-01-02 15:04")
		it.ExpectedOut = expOut.Format("2006-01-02 15:04")
		it.TodayPrice = price
		it.AlreadyPost = posted
		it.Balance = balance

		overstay := expOut.Format("2006-01-02") < bizDayDate
		it.Overstay = overstay

		inHouse = append(inHouse, it)
		if overstay && !posted && price > 0 {
			toPost = append(toPost, it)
		}
		if overstay {
			overdue = append(overdue, it)
		}
		if balance > 0.001 {
			unpaid = append(unpaid, it)
		}
	}

	// 当日营收
	var revenue float64
	revArgsFull := []any{bizDay}
	revArgsFull = append(revArgsFull, revArgs...)
	_ = pool.QueryRow(ctx, `SELECT COALESCE(sum(p.amount),0) FROM payment p JOIN folio f ON f.id=p.folio_id JOIN check_in c ON c.id=f.check_in_id WHERE p.pay_time::date = $1`+revClause,
		revArgsFull...).Scan(&revenue)

	// 当日入住/退房数
	var checkins, checkouts int64
	ciArgsFull := []any{bizDay}
	ciArgsFull = append(ciArgsFull, ciArgs...)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM check_in WHERE check_in_time::date = $1`+ciClause, ciArgsFull...).Scan(&checkins)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM check_in WHERE status=1 AND updated_at::date = $1`+ciClause, ciArgsFull...).Scan(&checkouts)

	var postedAmount float64
	for _, p := range toPost {
		postedAmount += p.TodayPrice
	}

	return map[string]any{
		"biz_date":       bizDayDate,
		"in_house":       inHouse,
		"in_house_count": len(inHouse),
		"to_post":        toPost,
		"to_post_count":  len(toPost),
		"to_post_amount": postedAmount,
		"overdue":        overdue,
		"overdue_count":  len(overdue),
		"unpaid":         unpaid,
		"unpaid_count":   len(unpaid),
		"today_revenue":  revenue,
		"today_checkin":  checkins,
		"today_checkout": checkouts,
	}, nil
}

// storeClauseAt 构建门店过滤子句，占位符序号从 idx 开始（用于已有前置参数的查询）。
func storeClauseAt(storeIDs []int64, col string, idx int) (string, []any) {
	if storeIDs == nil {
		return "", nil
	}
	if len(storeIDs) == 0 {
		return " AND FALSE", nil
	}
	return fmt.Sprintf(" AND %s = ANY($%d)", col, idx), []any{storeIDs}
}

// NightAuditRun 执行夜审（POST /api/v1/night-audit/run）。仅集团管理员可执行。
func NightAuditRun(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	ctx := r.Context()

	bizDay, err := currentBizDay(ctx, pool)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var audited bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM night_audit_log WHERE biz_date = $1 AND status = 1)`, bizDay).Scan(&audited)
	if audited {
		writeJSON(w, http.StatusConflict, map[string]string{"error": fmt.Sprintf("营业日 %s 已完成夜审，不可重复", bizDay.Format("2006-01-02"))})
		return
	}

	// 预览（取异常计数用于日志）
	preview, err := buildAuditPreview(ctx, pool, r, bizDay)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	overdueCount, _ := preview["overdue_count"].(int)
	unpaidCount, _ := preview["unpaid_count"].(int)

	tx, err := pool.Begin(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer tx.Rollback(ctx)

	// 1. 对超期未退房且未补过账的账单，追加 1 晚房费（事务内重新查询保证一致性）
	postedCount := 0
	var postedAmount float64
	rows, err := tx.Query(ctx, `
		SELECT ci.id, f.id, COALESCE(rc.price, 0)
		FROM check_in ci
		JOIN folio f ON f.check_in_id = ci.id
		JOIN room r ON r.id = ci.room_id
		LEFT JOIN room_type rt ON rt.id = r.room_type_id
		LEFT JOIN rate_plan rp ON rp.store_id = ci.store_id AND rp.type = 'rack'
		LEFT JOIN rate_calendar rc ON rc.room_type_id = rt.id AND rc.rate_plan_id = rp.id AND rc.biz_date = $1
		WHERE ci.status = 0 AND ci.expected_checkout_time::date < $1
		  AND NOT EXISTS (SELECT 1 FROM folio_item fi WHERE fi.folio_id = f.id AND fi.item_type='room_fee' AND fi.biz_date = $1)`, bizDay)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for rows.Next() {
		var checkInID, folioID int64
		var price float64
		if err := rows.Scan(&checkInID, &folioID, &price); err != nil {
			rows.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if price <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO folio_item (folio_id, item_type, amount, remark, biz_date) VALUES ($1, 'room_fee', $2, $3, $4)`,
			folioID, price, fmt.Sprintf("夜审补过账 %s", bizDay.Format("2006-01-02")), bizDay,
		); err != nil {
			rows.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if _, err := tx.Exec(ctx,
			`UPDATE folio SET total_amount = total_amount + $1, balance = balance + $1, updated_at = now() WHERE id = $2`,
			price, folioID,
		); err != nil {
			rows.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		postedCount++
		postedAmount += price
	}
	rows.Close()

	// 2. 汇总当日指标（bizDay 为 $1，门店过滤为 $2）
	storeIDs := storeIDsOf(r)
	ciClause, ciArgs := storeClauseAt(storeIDs, "ci.store_id", 2)
	revClause, revArgs := storeClauseAt(storeIDs, "c.store_id", 2)

	var revenue float64
	revArgsFull := []any{bizDay}
	revArgsFull = append(revArgsFull, revArgs...)
	_ = tx.QueryRow(ctx, `SELECT COALESCE(sum(p.amount),0) FROM payment p JOIN folio f ON f.id=p.folio_id JOIN check_in c ON c.id=f.check_in_id WHERE p.pay_time::date = $1`+revClause,
		revArgsFull...).Scan(&revenue)

	var checkins, checkouts, inHouse int64
	ciArgsFull := []any{bizDay}
	ciArgsFull = append(ciArgsFull, ciArgs...)
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM check_in WHERE check_in_time::date = $1`+ciClause, ciArgsFull...).Scan(&checkins)
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM check_in WHERE status=1 AND updated_at::date = $1`+ciClause, ciArgsFull...).Scan(&checkouts)
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM check_in WHERE status=0`+ciClause, ciArgsFull...).Scan(&inHouse)

	// 异常说明
	exceptions := ""
	if overdueCount > 0 {
		exceptions = fmt.Sprintf("超期未退房 %d 间", overdueCount)
	}
	if unpaidCount > 0 {
		if exceptions != "" {
			exceptions += "；"
		}
		exceptions += fmt.Sprintf("账单未结 %d 笔", unpaidCount)
	}

	// 3. 写入夜审日志
	opID := int64(0)
	if u := currentUser(r); u != nil {
		opID = u.ID
	}
	var logID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO night_audit_log (biz_date, status, started_by, started_at, completed_at, revenue, checkins, checkouts, in_house, posted_count, posted_amount, exceptions)
		 VALUES ($1, 1, $2, now(), now(), $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		bizDay, opID, revenue, checkins, checkouts, inHouse, postedCount, postedAmount, exceptions,
	).Scan(&logID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"audit_id":      logID,
		"biz_date":      bizDay.Format("2006-01-02"),
		"posted_count":  postedCount,
		"posted_amount": postedAmount,
		"revenue":       revenue,
		"checkins":      checkins,
		"checkouts":     checkouts,
		"in_house":      inHouse,
		"exceptions":    exceptions,
		"next_biz_date": bizDay.AddDate(0, 0, 1).Format("2006-01-02"),
	})
	LogAction(w, r, 0, "night_audit", bizDay.Format("2006-01-02"), fmt.Sprintf("夜审完成：补过账%d笔¥%.2f 营收¥%.2f", postedCount, postedAmount, revenue))
}

// NightAuditHistory 夜审历史（GET /api/v1/night-audit/history）。
func NightAuditHistory(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	ctx := r.Context()

	page := queryInt64(r, "page")
	if page < 1 {
		page = 1
	}
	pageSize := queryInt64(r, "page_size")
	if pageSize < 1 || pageSize > 200 {
		pageSize = 30
	}

	var total int64
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM night_audit_log WHERE status = 1`).Scan(&total)

	offset := (page - 1) * pageSize
	rows, err := pool.Query(ctx, `
		SELECT n.id, n.biz_date, COALESCE(u.name,''), n.started_at, n.completed_at,
		       n.revenue, n.checkins, n.checkouts, n.in_house, n.posted_count, n.posted_amount, COALESCE(n.exceptions,'')
		FROM night_audit_log n
		LEFT JOIN users u ON u.id = n.started_by
		WHERE n.status = 1
		ORDER BY n.biz_date DESC
		LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type auditItem struct {
		ID           int64   `json:"id"`
		BizDate      string  `json:"biz_date"`
		Operator     string  `json:"operator"`
		StartedAt    string  `json:"started_at"`
		CompletedAt  string  `json:"completed_at"`
		Revenue      float64 `json:"revenue"`
		Checkins     int64   `json:"checkins"`
		Checkouts    int64   `json:"checkouts"`
		InHouse      int64   `json:"in_house"`
		PostedCount  int     `json:"posted_count"`
		PostedAmount float64 `json:"posted_amount"`
		Exceptions   string  `json:"exceptions"`
	}
	list := make([]auditItem, 0)
	for rows.Next() {
		var it auditItem
		var started, completed time.Time
		if err := rows.Scan(&it.ID, &it.BizDate, &it.Operator, &started, &completed,
			&it.Revenue, &it.Checkins, &it.Checkouts, &it.InHouse, &it.PostedCount, &it.PostedAmount, &it.Exceptions); err != nil {
			continue
		}
		it.StartedAt = started.Format("2006-01-02 15:04:05")
		it.CompletedAt = completed.Format("2006-01-02 15:04:05")
		list = append(list, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":     list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
