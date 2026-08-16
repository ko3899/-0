package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"hotel-management/server/internal/db"
)

// CreateReservation 新建预订：查/建客户 → 创建预订 + 预订明细。
func CreateReservation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		StoreID       int64   `json:"store_id"`
		CustomerName  string  `json:"customer_name"`
		CustomerPhone string  `json:"customer_phone"`
		Channel       string  `json:"channel"`
		CheckInDate   string  `json:"check_in_date"`
		CheckOutDate  string  `json:"check_out_date"`
		RoomTypeID    int64   `json:"room_type_id"`
		Contact       string  `json:"contact"`
		Remark        string  `json:"remark"`
		Deposit       float64 `json:"deposit"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.CustomerName = strings.TrimSpace(req.CustomerName)
	if req.StoreID == 0 || req.CustomerName == "" || req.CheckInDate == "" || req.CheckOutDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "门店/客户/入住离店日期不能为空"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(req.StoreID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if req.Channel == "" {
		req.Channel = "walk_in"
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

	// 1. 查/建客户
	var customerID int64
	if req.CustomerPhone != "" {
		err := tx.QueryRow(r.Context(),
			`SELECT id FROM customer WHERE phone = $1`, req.CustomerPhone,
		).Scan(&customerID)
		if err != nil {
			if err := tx.QueryRow(r.Context(),
				`INSERT INTO customer (name, phone) VALUES ($1, $2) RETURNING id`,
				req.CustomerName, req.CustomerPhone,
			).Scan(&customerID); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
	} else {
		if err := tx.QueryRow(r.Context(),
			`INSERT INTO customer (name) VALUES ($1) RETURNING id`, req.CustomerName,
		).Scan(&customerID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	// 1.5 房型库存冲突检测
	if req.RoomTypeID > 0 {
		ok, err := checkRoomTypeAvailable(r.Context(), tx, req.StoreID, req.RoomTypeID, req.CheckInDate, req.CheckOutDate, 0)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !ok {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "该房型在所选日期已订满，请调整日期或房型"})
			return
		}
	}

	// 2. 创建预订
	var reservationID int64
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO reservation (store_id, customer_id, channel, status, check_in_date, check_out_date, deposit, contact, remark)
		 VALUES ($1, $2, $3, 0, $4::date, $5::date, $6, $7, $8) RETURNING id`,
		req.StoreID, customerID, req.Channel, req.CheckInDate, req.CheckOutDate, req.Deposit, req.Contact, req.Remark,
	).Scan(&reservationID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 3. 预订明细（房型 + 首晚门市价）
	if req.RoomTypeID > 0 {
		var price float64
		_ = tx.QueryRow(r.Context(),
			`SELECT COALESCE(rc.price, 0)
			 FROM room_type rt
			 JOIN rate_plan rp ON rp.store_id = $1 AND rp.type = 'rack'
			 LEFT JOIN rate_calendar rc ON rc.room_type_id = rt.id AND rc.rate_plan_id = rp.id AND rc.biz_date = $2::date
			 WHERE rt.id = $3`,
			req.StoreID, req.CheckInDate, req.RoomTypeID,
		).Scan(&price)
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO reservation_item (reservation_id, room_type_id, price) VALUES ($1, $2, $3)`,
			reservationID, req.RoomTypeID, price,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": reservationID})
}

// ListReservations 预订列表接口（按门店/状态过滤）。
func ListReservations(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeID := queryInt64(r, "store_id")
	status := -1
	if s := r.URL.Query().Get("status"); s != "" {
		status, _ = strconv.Atoi(s)
	}
	u := currentUser(r)

	query := `SELECT r.id, r.store_id, s.name, c.name, COALESCE(c.phone,''), r.channel, r.status,
	                 r.check_in_date, r.check_out_date, r.deposit, COALESCE(r.contact,''),
	                 COALESCE((SELECT rt.name FROM reservation_item ri JOIN room_type rt ON rt.id = ri.room_type_id WHERE ri.reservation_id = r.id LIMIT 1), ''),
	                 COALESCE((SELECT rm.room_no FROM reservation_item ri LEFT JOIN room rm ON rm.id = ri.room_id WHERE ri.reservation_id = r.id LIMIT 1), ''),
	                 COALESCE((SELECT ri.room_type_id FROM reservation_item ri WHERE ri.reservation_id = r.id LIMIT 1), 0),
	                 COALESCE(r.remark, '')
	          FROM reservation r
	          JOIN store s ON s.id = r.store_id
	          JOIN customer c ON c.id = r.customer_id
	          WHERE 1=1`
	args := []any{}
	argIdx := 1
	if storeID > 0 {
		if u != nil && !u.IsAdmin && !u.canAccessStore(storeID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
			return
		}
		query += ` AND r.store_id = $` + itoa(argIdx)
		args = append(args, storeID)
		argIdx++
	} else if u != nil && !u.IsAdmin {
		if len(u.StoreIDs) == 0 {
			query += ` AND FALSE`
		} else {
			query += ` AND r.store_id = ANY($` + itoa(argIdx) + `)`
			args = append(args, u.StoreIDs)
			argIdx++
		}
	}
	if status >= 0 {
		query += ` AND r.status = $` + itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	query += ` ORDER BY r.check_in_date DESC, r.id DESC LIMIT 200`

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type reservation struct {
		ID           int64     `json:"id"`
		StoreID      int64     `json:"store_id"`
		StoreName    string    `json:"store_name"`
		GuestName    string    `json:"guest_name"`
		GuestPhone   string    `json:"guest_phone"`
		Channel      string    `json:"channel"`
		Status       int       `json:"status"`
		CheckInDate  time.Time `json:"check_in_date"`
		CheckOutDate time.Time `json:"check_out_date"`
		Deposit      float64   `json:"deposit"`
		Contact      string    `json:"contact"`
		RoomTypeName string    `json:"room_type_name"`
		RoomNo       string    `json:"room_no"`
		RoomTypeID   int64     `json:"room_type_id"`
		Remark       string    `json:"remark"`
	}
	list := make([]reservation, 0)
	for rows.Next() {
		var rv reservation
		if err := rows.Scan(&rv.ID, &rv.StoreID, &rv.StoreName, &rv.GuestName, &rv.GuestPhone, &rv.Channel, &rv.Status,
			&rv.CheckInDate, &rv.CheckOutDate, &rv.Deposit, &rv.Contact, &rv.RoomTypeName, &rv.RoomNo, &rv.RoomTypeID, &rv.Remark); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, rv)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reservations": list, "total": len(list)})
}

// ReservationCheckIn 预订转入住：校验预订/房间 → 创建入住+账单 → 更新预订状态。
func ReservationCheckIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		RoomID int64 `json:"room_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.RoomID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请选择入住房间"})
		return
	}

	reservationID := pathID(r.URL.Path)
	if reservationID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少预订 ID"})
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

	// 1. 查预订 + 房型
	var (
		storeID    int64
		customerID int64
		status     int
		deposit    float64
	)
	if err := tx.QueryRow(r.Context(),
		`SELECT store_id, customer_id, status, deposit FROM reservation WHERE id = $1 FOR UPDATE`, reservationID,
	).Scan(&storeID, &customerID, &status, &deposit); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "预订不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "该预订已入住或已取消"})
		return
	}
	var roomTypeID int64
	if err := tx.QueryRow(r.Context(),
		`SELECT COALESCE(room_type_id, 0) FROM reservation_item WHERE reservation_id = $1 LIMIT 1`, reservationID,
	).Scan(&roomTypeID); err != nil {
		roomTypeID = 0
	}

	// 2. 校验房间（门店匹配、房型匹配、可入住）
	var roomStoreID, roomRTID int64
	var roomStatus int
	if err := tx.QueryRow(r.Context(),
		`SELECT store_id, room_type_id, status FROM room WHERE id = $1 FOR UPDATE`, req.RoomID,
	).Scan(&roomStoreID, &roomRTID, &roomStatus); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "房间不存在"})
		return
	}
	if roomStoreID != storeID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "房间不属于预订门店"})
		return
	}
	if roomTypeID > 0 && roomRTID != roomTypeID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "房间房型与预订不符"})
		return
	}
	if roomStatus != 0 && roomStatus != 1 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "当前房间状态不可入住"})
		return
	}

	// 3. 房价（当日门市价）
	var price float64
	_ = tx.QueryRow(r.Context(),
		`SELECT COALESCE(rc.price, 0)
		 FROM room r
		 JOIN room_type rt ON rt.id = r.room_type_id
		 JOIN rate_plan rp ON rp.store_id = r.store_id AND rp.type = 'rack'
		 LEFT JOIN rate_calendar rc ON rc.room_type_id = rt.id AND rc.rate_plan_id = rp.id AND rc.biz_date = CURRENT_DATE
		 WHERE r.id = $1`, req.RoomID,
	).Scan(&price)

	// 4. 创建入住 + 账单（1 晚）
	var checkInID int64
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO check_in (store_id, reservation_id, customer_id, room_id, check_in_time, expected_checkout_time, deposit, status)
		 VALUES ($1, $2, $3, $4, now(), now() + interval '1 day', $5, 0) RETURNING id`,
		storeID, reservationID, customerID, req.RoomID, deposit,
	).Scan(&checkInID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	balance := price - deposit
	folioStatus := 0
	if balance <= 0 {
		folioStatus = 1
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO folio (check_in_id, total_amount, paid_amount, balance, status)
		 VALUES ($1, $2, $3, $4, $5)`,
		checkInID, price, deposit, balance, folioStatus,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 5. 更新预订状态=已入住 + 关联房间
	if _, err := tx.Exec(r.Context(),
		`UPDATE reservation SET status = 1, updated_at = now() WHERE id = $1`, reservationID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE reservation_item SET room_id = $1 WHERE reservation_id = $2`, req.RoomID, reservationID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 6. 房间转住客
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
	writeJSON(w, http.StatusOK, map[string]any{"check_in_id": checkInID, "total_amount": price})
}

// itoa 整数转字符串。
func itoa(n int) string {
	return strconv.Itoa(n)
}

// checkRoomTypeAvailable 房型库存冲突检测：同门店同房型在日期区间内
// 已有的预订(0)+已入住(1)数量是否已达可用房间数。excludeResID 用于修改时排除自身。
func checkRoomTypeAvailable(ctx context.Context, tx pgx.Tx, storeID, roomTypeID int64, checkIn, checkOut string, excludeResID int64) (bool, error) {
	var roomCount int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM room WHERE store_id = $1 AND room_type_id = $2 AND status != 3`,
		storeID, roomTypeID,
	).Scan(&roomCount); err != nil {
		return false, err
	}
	var overlap int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM reservation r
		 JOIN reservation_item ri ON ri.reservation_id = r.id
		 WHERE r.store_id = $1 AND ri.room_type_id = $2
		   AND r.status IN (0, 1)
		   AND r.id != $3
		   AND r.check_in_date < $5::date
		   AND r.check_out_date > $4::date`,
		storeID, roomTypeID, excludeResID, checkIn, checkOut,
	).Scan(&overlap); err != nil {
		return false, err
	}
	return overlap < roomCount, nil
}

// UpdateReservation 修改预订：仅状态=预订(0)可改，日期/房型/渠道/联系人/备注/定金。
func UpdateReservation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Channel      string   `json:"channel"`
		CheckInDate  string   `json:"check_in_date"`
		CheckOutDate string   `json:"check_out_date"`
		RoomTypeID   int64    `json:"room_type_id"`
		Contact      *string  `json:"contact"`
		Remark       *string  `json:"remark"`
		Deposit      *float64 `json:"deposit"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	reservationID := pathID(r.URL.Path)
	if reservationID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少预订 ID"})
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

	var (
		storeID     int64
		status      int
		curIn       time.Time
		curOut      time.Time
		curRoomType int64
	)
	if err := tx.QueryRow(r.Context(),
		`SELECT store_id, status, check_in_date, check_out_date,
		        COALESCE((SELECT room_type_id FROM reservation_item WHERE reservation_id = r.id LIMIT 1), 0)
		 FROM reservation r WHERE id = $1 FOR UPDATE`, reservationID,
	).Scan(&storeID, &status, &curIn, &curOut, &curRoomType); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "预订不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅预订状态可修改"})
		return
	}

	newIn := curIn
	newOut := curOut
	if req.CheckInDate != "" {
		newIn, err = time.Parse("2006-01-02", req.CheckInDate)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "入住日期格式应为 YYYY-MM-DD"})
			return
		}
	}
	if req.CheckOutDate != "" {
		newOut, err = time.Parse("2006-01-02", req.CheckOutDate)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "离店日期格式应为 YYYY-MM-DD"})
			return
		}
	}
	if !newOut.After(newIn) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "离店日期必须晚于入住日期"})
		return
	}

	newRoomType := curRoomType
	if req.RoomTypeID > 0 {
		newRoomType = req.RoomTypeID
	}
	if newRoomType > 0 {
		ok, err := checkRoomTypeAvailable(r.Context(), tx, storeID, newRoomType, newIn.Format("2006-01-02"), newOut.Format("2006-01-02"), reservationID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !ok {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "该房型在所选日期已订满，请调整日期或房型"})
			return
		}
	}

	if _, err := tx.Exec(r.Context(),
		`UPDATE reservation SET check_in_date = $1, check_out_date = $2,
		        channel = COALESCE(NULLIF($3,''), channel),
		        contact = COALESCE($4, contact), remark = COALESCE($5, remark),
		        deposit = COALESCE($6, deposit), updated_at = now()
		 WHERE id = $7`,
		newIn, newOut, req.Channel, req.Contact, req.Remark, req.Deposit, reservationID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if req.RoomTypeID > 0 && req.RoomTypeID != curRoomType {
		if _, err := tx.Exec(r.Context(),
			`UPDATE reservation_item SET room_type_id = $1, updated_at = now() WHERE reservation_id = $2`,
			req.RoomTypeID, reservationID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": reservationID, "ok": true})
}

// CancelReservation 取消预订：状态 0→2。
func CancelReservation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	reservationID := pathID(r.URL.Path)
	if reservationID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少预订 ID"})
		return
	}
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	var (
		storeID int64
		status  int
	)
	if err := pool.QueryRow(r.Context(),
		`SELECT store_id, status FROM reservation WHERE id = $1`, reservationID,
	).Scan(&storeID, &status); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "预订不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅预订状态可取消"})
		return
	}
	if _, err := pool.Exec(r.Context(),
		`UPDATE reservation SET status = 2, updated_at = now() WHERE id = $1`, reservationID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": reservationID, "status": 2})
}

// ReservationNoShow 预订未到（No-show）：状态 0→4。
func ReservationNoShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	reservationID := pathID(r.URL.Path)
	if reservationID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少预订 ID"})
		return
	}
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	var (
		storeID int64
		status  int
	)
	if err := pool.QueryRow(r.Context(),
		`SELECT store_id, status FROM reservation WHERE id = $1`, reservationID,
	).Scan(&storeID, &status); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "预订不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅预订状态可标记 No-show"})
		return
	}
	if _, err := pool.Exec(r.Context(),
		`UPDATE reservation SET status = 4, updated_at = now() WHERE id = $1`, reservationID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": reservationID, "status": 4})
}
