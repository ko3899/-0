package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"hotel-management/server/internal/db"
)

// CreateCheckIn 办理入住（散客 walk-in）：
// 校验房间可入住 → 查/建客户 → 创建 check_in + folio + 房费明细 + 押金支付 → 房间转住客。
func CreateCheckIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		RoomID        int64   `json:"room_id"`
		CustomerName  string  `json:"customer_name"`
		CustomerPhone string  `json:"customer_phone"`
		IDNo          string  `json:"id_no"`
		Price         float64 `json:"price"`
		Deposit       float64 `json:"deposit"`
		Nights        int     `json:"nights"`
		Method        string  `json:"method"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.CustomerName = strings.TrimSpace(req.CustomerName)
	if req.RoomID == 0 || req.CustomerName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "房间和客户姓名不能为空"})
		return
	}
	if req.Nights <= 0 {
		req.Nights = 1
	}
	if req.Method == "" {
		req.Method = "cash"
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

	// 1. 锁定房间，校验可入住（0空净/1空脏）
	var storeID int64
	var status int
	if err := tx.QueryRow(r.Context(),
		`SELECT store_id, status FROM room WHERE id = $1 FOR UPDATE`, req.RoomID,
	).Scan(&storeID, &status); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "房间不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 0 && status != 1 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "当前房间状态不可入住"})
		return
	}

	// 2. 房价：未传则查当日门市价
	price := req.Price
	if price <= 0 {
		_ = tx.QueryRow(r.Context(),
			`SELECT COALESCE(rc.price, 0)
			 FROM room r
			 JOIN room_type rt ON rt.id = r.room_type_id
			 JOIN rate_plan rp ON rp.store_id = r.store_id AND rp.type = 'rack'
			 LEFT JOIN rate_calendar rc ON rc.room_type_id = rt.id AND rc.rate_plan_id = rp.id AND rc.biz_date = CURRENT_DATE
			 WHERE r.id = $1`, req.RoomID,
		).Scan(&price)
	}

	// 3. 查/建客户（按手机号）
	var customerID int64
	if req.CustomerPhone != "" {
		err := tx.QueryRow(r.Context(),
			`SELECT id FROM customer WHERE phone = $1`, req.CustomerPhone,
		).Scan(&customerID)
		if err != nil {
			if err := tx.QueryRow(r.Context(),
				`INSERT INTO customer (name, phone, id_no, id_type) VALUES ($1, $2, $3, 'id_card') RETURNING id`,
				req.CustomerName, req.CustomerPhone, req.IDNo,
			).Scan(&customerID); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
	} else {
		if err := tx.QueryRow(r.Context(),
			`INSERT INTO customer (name, id_no, id_type) VALUES ($1, $2, 'id_card') RETURNING id`,
			req.CustomerName, req.IDNo,
		).Scan(&customerID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	// 4. 创建 check_in
	var checkInID int64
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO check_in (store_id, customer_id, room_id, check_in_time, expected_checkout_time, deposit, status)
		 VALUES ($1, $2, $3, now(), now() + make_interval(days => $4), $5, 0) RETURNING id`,
		storeID, customerID, req.RoomID, req.Nights, req.Deposit,
	).Scan(&checkInID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 5. 创建账单 folio + 房费明细
	total := price * float64(req.Nights)
	balance := total - req.Deposit
	folioStatus := 0
	if balance <= 0 {
		folioStatus = 1
	}
	var folioID int64
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO folio (check_in_id, total_amount, paid_amount, balance, status)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		checkInID, total, req.Deposit, balance, folioStatus,
	).Scan(&folioID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO folio_item (folio_id, item_type, amount, remark) VALUES ($1, 'room_fee', $2, $3)`,
		folioID, total, fmt.Sprintf("房费 %d 晚", req.Nights),
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 6. 押金支付
	if req.Deposit > 0 {
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO payment (folio_id, method, amount) VALUES ($1, $2, $3)`,
			folioID, req.Method, req.Deposit,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	// 7. 房间转住客
	if _, err := tx.Exec(r.Context(),
		`UPDATE room SET status = 2, updated_at = now() WHERE id = $1`, req.RoomID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 异步推送 OTA 库存（入住后可售减少）
	PushInventoryByRoom(pool, req.RoomID)

	writeJSON(w, http.StatusOK, map[string]any{
		"check_in_id":  checkInID,
		"folio_id":     folioID,
		"total_amount": total,
		"deposit":      req.Deposit,
		"balance":      balance,
	})
	LogAction(w, r, storeID, "checkin", itoa64(checkInID), req.CustomerName+" 入住")
}

// CheckOut 办理退房结账：记录支付 → 更新账单 → 退房 → 房间转空脏。
func CheckOut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Method string  `json:"method"`
		Amount float64 `json:"amount"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Method == "" {
		req.Method = "cash"
	}

	checkInID := pathID(r.URL.Path)
	if checkInID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少入住记录 ID"})
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

	var roomID, storeID int64
	var status int
	if err := tx.QueryRow(r.Context(),
		`SELECT room_id, store_id, status FROM check_in WHERE id = $1 FOR UPDATE`, checkInID,
	).Scan(&roomID, &storeID, &status); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "入住记录不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status == 1 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "该房间已退房"})
		return
	}

	var folioID int64
	var total, paid float64
	if err := tx.QueryRow(r.Context(),
		`SELECT id, total_amount, paid_amount FROM folio WHERE check_in_id = $1`, checkInID,
	).Scan(&folioID, &total, &paid); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 记录支付
	if req.Amount > 0 {
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO payment (folio_id, method, amount) VALUES ($1, $2, $3)`,
			folioID, req.Method, req.Amount,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	newPaid := paid + req.Amount
	balance := total - newPaid
	folioStatus := 0
	if balance <= 0 {
		folioStatus = 1
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE folio SET paid_amount = $1, balance = $2, status = $3, updated_at = now() WHERE id = $4`,
		newPaid, balance, folioStatus, folioID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if _, err := tx.Exec(r.Context(),
		`UPDATE check_in SET status = 1, updated_at = now() WHERE id = $1`, checkInID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 房间转空脏
	if _, err := tx.Exec(r.Context(),
		`UPDATE room SET status = 1, updated_at = now() WHERE id = $1`, roomID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 自动生成退房清洁任务（housekeeping）
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO housekeeping_task (store_id, room_id, check_in_id, task_type, status) VALUES ($1, $2, $3, 0, 0)`,
		storeID, roomID, checkInID,
	); err != nil {
		// 清洁任务创建失败不阻断退房主流程，仅记录
		_ = err
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 异步推送 OTA 库存（退房后房间转空脏，待清洁后转空净会再推一次）
	PushInventoryByRoom(pool, roomID)

	writeJSON(w, http.StatusOK, map[string]any{
		"check_in_id": checkInID,
		"total":       total,
		"paid":        newPaid,
		"balance":     balance,
		"settled":     folioStatus == 1,
	})
	LogAction(w, r, storeID, "checkout", itoa64(checkInID), fmt.Sprintf("结账 total=%.2f paid=%.2f", total, newPaid))
}

// ListCheckIns 在住列表接口（按门店），含账单汇总。
func ListCheckIns(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeID := queryInt64(r, "store_id")
	floor := r.URL.Query().Get("floor")
	roomNo := r.URL.Query().Get("room_no")
	guestName := r.URL.Query().Get("guest_name")
	guestPhone := r.URL.Query().Get("guest_phone")
	depositMin := queryFloat(r, "deposit_min")
	depositMax := queryFloat(r, "deposit_max")

	query := `SELECT ci.id, ci.store_id, s.name, r.room_no, c.name, COALESCE(c.phone,''),
	                 ci.check_in_time, ci.expected_checkout_time, ci.deposit,
	                 f.total_amount, f.paid_amount, f.balance, f.status
	          FROM check_in ci
	          JOIN store s ON s.id = ci.store_id
	          JOIN room r ON r.id = ci.room_id
	          JOIN customer c ON c.id = ci.customer_id
	          JOIN folio f ON f.check_in_id = ci.id
	          WHERE ci.status = 0`
	args := []any{}
	cond, scopeArgs, forbidden := storeCond(r, storeID, "ci.store_id")
	if forbidden {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
		return
	}
	if cond != "" {
		query += ` AND ` + cond
		args = append(args, scopeArgs...)
	}
	if floor != "" {
		query += ` AND r.floor = $` + itoa(len(args)+1)
		args = append(args, floor)
	}
	if roomNo != "" {
		query += ` AND r.room_no ILIKE $` + itoa(len(args)+1)
		args = append(args, "%"+roomNo+"%")
	}
	if guestName != "" {
		query += ` AND c.name ILIKE $` + itoa(len(args)+1)
		args = append(args, "%"+guestName+"%")
	}
	if guestPhone != "" {
		query += ` AND c.phone ILIKE $` + itoa(len(args)+1)
		args = append(args, "%"+guestPhone+"%")
	}
	if depositMin != nil {
		query += ` AND ci.deposit >= $` + itoa(len(args)+1)
		args = append(args, *depositMin)
	}
	if depositMax != nil {
		query += ` AND ci.deposit <= $` + itoa(len(args)+1)
		args = append(args, *depositMax)
	}
	query += ` ORDER BY ci.check_in_time DESC`

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type checkIn struct {
		ID          int64      `json:"id"`
		StoreID     int64      `json:"store_id"`
		StoreName   string     `json:"store_name"`
		RoomNo      string     `json:"room_no"`
		GuestName   string     `json:"guest_name"`
		GuestPhone  string     `json:"guest_phone"`
		CheckInAt   time.Time  `json:"check_in_at"`
		CheckOutAt  *time.Time `json:"check_out_at"`
		Deposit     float64    `json:"deposit"`
		Total       float64    `json:"total"`
		Paid        float64    `json:"paid"`
		Balance     float64    `json:"balance"`
		FolioStatus int        `json:"folio_status"`
	}
	list := make([]checkIn, 0)
	for rows.Next() {
		var c checkIn
		var expected *time.Time
		if err := rows.Scan(&c.ID, &c.StoreID, &c.StoreName, &c.RoomNo, &c.GuestName, &c.GuestPhone,
			&c.CheckInAt, &expected, &c.Deposit, &c.Total, &c.Paid, &c.Balance, &c.FolioStatus); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.CheckOutAt = expected
		list = append(list, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"check_ins": list, "total": len(list)})
}

// GetFolio 账单详情接口：返回账单 + 明细 + 支付记录。
func GetFolio(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	checkInID := pathID(r.URL.Path)
	if checkInID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少入住记录 ID"})
		return
	}

	var storeID int64
	if err := pool.QueryRow(r.Context(),
		`SELECT store_id FROM check_in WHERE id = $1`, checkInID,
	).Scan(&storeID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "入住记录不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
		return
	}

	var (
		folioID     int64
		total       float64
		paid        float64
		balance     float64
		folioStatus int
	)
	if err := pool.QueryRow(r.Context(),
		`SELECT id, total_amount, paid_amount, balance, status FROM folio WHERE check_in_id = $1`, checkInID,
	).Scan(&folioID, &total, &paid, &balance, &folioStatus); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "账单不存在"})
		return
	}

	// 明细
	type item struct {
		ItemType string    `json:"item_type"`
		Amount   float64   `json:"amount"`
		BizTime  time.Time `json:"biz_time"`
		Remark   string    `json:"remark"`
	}
	items := make([]item, 0)
	rows, err := pool.Query(r.Context(),
		`SELECT item_type, amount, biz_time, COALESCE(remark,'') FROM folio_item WHERE folio_id = $1 ORDER BY id`, folioID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ItemType, &it.Amount, &it.BizTime, &it.Remark); err != nil {
			rows.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		items = append(items, it)
	}
	rows.Close()

	// 支付记录
	type pay struct {
		Method  string    `json:"method"`
		Amount  float64   `json:"amount"`
		PayTime time.Time `json:"pay_time"`
	}
	payments := make([]pay, 0)
	prows, err := pool.Query(r.Context(),
		`SELECT method, amount, pay_time FROM payment WHERE folio_id = $1 ORDER BY id`, folioID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for prows.Next() {
		var p pay
		if err := prows.Scan(&p.Method, &p.Amount, &p.PayTime); err != nil {
			prows.Close()
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		payments = append(payments, p)
	}
	prows.Close()

	writeJSON(w, http.StatusOK, map[string]any{
		"folio_id": folioID,
		"total":    total,
		"paid":     paid,
		"balance":  balance,
		"status":   folioStatus,
		"items":    items,
		"payments": payments,
	})
}

// ChangeRoom 换房：同店换房，旧房间转空脏，新房间转住客，记录变更历史。
func ChangeRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		ToRoomID int64  `json:"to_room_id"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ToRoomID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "目标房间不能为空"})
		return
	}

	checkInID := pathID(r.URL.Path)
	if checkInID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少入住记录 ID"})
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

	// 锁定入住记录，校验在住 + 同店权限
	var fromRoomID, storeID int64
	var ciStatus int
	if err := tx.QueryRow(r.Context(),
		`SELECT room_id, store_id, status FROM check_in WHERE id = $1 FOR UPDATE`, checkInID,
	).Scan(&fromRoomID, &storeID, &ciStatus); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "入住记录不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if ciStatus != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅允许在住状态换房"})
		return
	}
	if fromRoomID == req.ToRoomID {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "不能换到相同房间"})
		return
	}

	// 锁定目标房间，校验同店且可入住
	var toRoomStoreID int64
	var toStatus int
	var toRoomNo string
	if err := tx.QueryRow(r.Context(),
		`SELECT store_id, status, room_no FROM room WHERE id = $1 FOR UPDATE`, req.ToRoomID,
	).Scan(&toRoomStoreID, &toStatus, &toRoomNo); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "目标房间不存在"})
		return
	}
	if toRoomStoreID != storeID {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "只能在同一门店内换房"})
		return
	}
	if toStatus != 0 && toStatus != 1 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "目标房间不可入住"})
		return
	}

	// 旧房间转空脏，新房间转住客
	if _, err := tx.Exec(r.Context(), `UPDATE room SET status = 1, updated_at = now() WHERE id = $1`, fromRoomID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if _, err := tx.Exec(r.Context(), `UPDATE room SET status = 2, updated_at = now() WHERE id = $1`, req.ToRoomID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// 更新入住记录的房间
	if _, err := tx.Exec(r.Context(), `UPDATE check_in SET room_id = $1, updated_at = now() WHERE id = $2`, req.ToRoomID, checkInID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// 记录换房历史
	opID := int64(0)
	if u := currentUser(r); u != nil {
		opID = u.ID
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO room_change (check_in_id, from_room_id, to_room_id, reason, operator_id) VALUES ($1, $2, $3, $4, $5)`,
		checkInID, fromRoomID, req.ToRoomID, req.Reason, opID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 异步推送 OTA 库存（换房涉及旧房型+新房型）
	PushInventoryByRoom(pool, fromRoomID)
	PushInventoryByRoom(pool, req.ToRoomID)

	writeJSON(w, http.StatusOK, map[string]any{
		"check_in_id":  checkInID,
		"from_room_id": fromRoomID,
		"to_room_id":   req.ToRoomID,
		"to_room_no":   toRoomNo,
		"ok":           true,
	})
	LogAction(w, r, storeID, "change_room", itoa64(checkInID), fmt.Sprintf("换房 %d→%d(%s)", fromRoomID, req.ToRoomID, toRoomNo))
}

// ExtendStay 续住：延长预计离店时间，追加房费明细，更新账单。
func ExtendStay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		ExtraNights int     `json:"extra_nights"`
		Price       float64 `json:"price"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ExtraNights <= 0 {
		req.ExtraNights = 1
	}

	checkInID := pathID(r.URL.Path)
	if checkInID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少入住记录 ID"})
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

	var storeID int64
	var ciStatus int
	if err := tx.QueryRow(r.Context(),
		`SELECT store_id, status FROM check_in WHERE id = $1 FOR UPDATE`, checkInID,
	).Scan(&storeID, &ciStatus); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "入住记录不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if ciStatus != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅允许在住状态续住"})
		return
	}

	// 房价：未传则查当日门市价
	price := req.Price
	if price <= 0 {
		var roomID int64
		_ = tx.QueryRow(r.Context(), `SELECT room_id FROM check_in WHERE id = $1`, checkInID).Scan(&roomID)
		_ = tx.QueryRow(r.Context(),
			`SELECT COALESCE(rc.price, 0)
			 FROM room r
			 JOIN room_type rt ON rt.id = r.room_type_id
			 JOIN rate_plan rp ON rp.store_id = r.store_id AND rp.type = 'rack'
			 LEFT JOIN rate_calendar rc ON rc.room_type_id = rt.id AND rc.rate_plan_id = rp.id AND rc.biz_date = CURRENT_DATE
			 WHERE r.id = $1`, roomID,
		).Scan(&price)
	}

	extra := price * float64(req.ExtraNights)

	// 延长预计离店时间
	if _, err := tx.Exec(r.Context(),
		`UPDATE check_in SET expected_checkout_time = expected_checkout_time + make_interval(days => $1), updated_at = now() WHERE id = $2`,
		req.ExtraNights, checkInID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 获取 folio
	var folioID int64
	if err := tx.QueryRow(r.Context(),
		`SELECT id FROM folio WHERE check_in_id = $1`, checkInID,
	).Scan(&folioID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 追加房费明细
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO folio_item (folio_id, item_type, amount, remark) VALUES ($1, 'room_fee', $2, $3)`,
		folioID, extra, fmt.Sprintf("续住 %d 晚", req.ExtraNights),
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 更新账单总额/待付
	if _, err := tx.Exec(r.Context(),
		`UPDATE folio SET total_amount = total_amount + $1, balance = balance + $1, updated_at = now() WHERE id = $2`,
		extra, folioID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 查新离店时间
	var newCheckout *time.Time
	_ = tx.QueryRow(r.Context(),
		`SELECT expected_checkout_time FROM check_in WHERE id = $1`, checkInID,
	).Scan(&newCheckout)

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"check_in_id":  checkInID,
		"extra_nights": req.ExtraNights,
		"extra_amount": extra,
		"new_checkout": newCheckout,
		"ok":           true,
	})
	LogAction(w, r, storeID, "extend", itoa64(checkInID), fmt.Sprintf("续住 %d 晚", req.ExtraNights))
}

// AddCharge 附加消费：入账迷你吧/商品/其他，更新账单。
func AddCharge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		ItemType string  `json:"item_type"`
		Amount   float64 `json:"amount"`
		Remark   string  `json:"remark"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ItemType == "" {
		req.ItemType = "other"
	}
	if req.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "金额必须大于 0"})
		return
	}

	checkInID := pathID(r.URL.Path)
	if checkInID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少入住记录 ID"})
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

	var storeID int64
	var ciStatus int
	if err := tx.QueryRow(r.Context(),
		`SELECT store_id, status FROM check_in WHERE id = $1 FOR UPDATE`, checkInID,
	).Scan(&storeID, &ciStatus); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "入住记录不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if ciStatus != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅允许在住状态入账"})
		return
	}

	var folioID int64
	if err := tx.QueryRow(r.Context(),
		`SELECT id FROM folio WHERE check_in_id = $1`, checkInID,
	).Scan(&folioID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if _, err := tx.Exec(r.Context(),
		`INSERT INTO folio_item (folio_id, item_type, amount, remark) VALUES ($1, $2, $3, $4)`,
		folioID, req.ItemType, req.Amount, req.Remark,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if _, err := tx.Exec(r.Context(),
		`UPDATE folio SET total_amount = total_amount + $1, balance = balance + $1, updated_at = now() WHERE id = $2`,
		req.Amount, folioID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"check_in_id": checkInID,
		"folio_id":    folioID,
		"item_type":   req.ItemType,
		"amount":      req.Amount,
		"ok":          true,
	})
	LogAction(w, r, storeID, "charge", itoa64(checkInID), fmt.Sprintf("%s %.2f 元", req.ItemType, req.Amount))
}

// AddPayment 中途收款：在住期间追加支付，更新账单已付/待付。
func AddPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Method string  `json:"method"`
		Amount float64 `json:"amount"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Method == "" {
		req.Method = "cash"
	}
	if req.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "金额必须大于 0"})
		return
	}

	checkInID := pathID(r.URL.Path)
	if checkInID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少入住记录 ID"})
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

	var storeID int64
	var ciStatus int
	if err := tx.QueryRow(r.Context(),
		`SELECT store_id, status FROM check_in WHERE id = $1 FOR UPDATE`, checkInID,
	).Scan(&storeID, &ciStatus); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "入住记录不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if ciStatus != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅允许在住状态收款"})
		return
	}

	var folioID int64
	var total, paid float64
	if err := tx.QueryRow(r.Context(),
		`SELECT id, total_amount, paid_amount FROM folio WHERE check_in_id = $1`, checkInID,
	).Scan(&folioID, &total, &paid); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	opID := int64(0)
	if u := currentUser(r); u != nil {
		opID = u.ID
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO payment (folio_id, method, amount, operator_id) VALUES ($1, $2, $3, $4)`,
		folioID, req.Method, req.Amount, opID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	newPaid := paid + req.Amount
	balance := total - newPaid
	folioStatus := 0
	if balance <= 0 {
		folioStatus = 1
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE folio SET paid_amount = $1, balance = $2, status = $3, updated_at = now() WHERE id = $4`,
		newPaid, balance, folioStatus, folioID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"check_in_id": checkInID,
		"folio_id":    folioID,
		"paid":        newPaid,
		"balance":     balance,
		"settled":     folioStatus == 1,
		"ok":          true,
	})
	LogAction(w, r, storeID, "payment", itoa64(checkInID), fmt.Sprintf("收款 %.2f 元 (%s)", req.Amount, req.Method))
}
