package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"hotel-management/server/internal/db"
)

// ============================================================
// OTA 订单同步
//   ReceiveOtaOrder  接收 OTA 平台订单回调（POST /api/v1/ota/orders/callback）
//   PullOtaOrders    手动触发拉取（POST /api/v1/ota/orders/pull）— 模拟生成若干订单
//   ListOtaOrders     订单列表（GET /api/v1/ota/orders）
//   ConfirmOtaOrder   确认订单 → 自动创建 PMS 预订 + 扣配额 + 推库存
//   RejectOtaOrder    拒单 → 释放配额
// 超卖防护：确认订单时原子扣配额，扣不到则拒单
// ============================================================

// otaOrderCallbackReq OTA 回调报文（演示格式，各平台实际格式不同）
type otaOrderCallbackReq struct {
	ChannelCode   string  `json:"channel_code"`
	OtaOrderNo    string  `json:"ota_order_no"`
	HotelID       string  `json:"hotel_id"`
	OtaRoomID     string  `json:"ota_room_type_id"`
	CustomerName  string  `json:"customer_name"`
	CustomerPhone string  `json:"customer_phone"`
	CheckInDate   string  `json:"check_in_date"`
	CheckOutDate  string  `json:"check_out_date"`
	Price         float64 `json:"price"`
	Nights        int     `json:"nights"`
}

// ReceiveOtaOrder 接收 OTA 平台订单回调（公开接口，无需登录鉴权）。
// 实际生产中各平台回调格式不同，需在 callChannelAPI 层之上做适配。
func ReceiveOtaOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req otaOrderCallbackReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.OtaOrderNo == "" || req.ChannelCode == "" || req.CustomerName == "" || req.CheckInDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "订单号/渠道/客户/入住日期不能为空"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	// 按渠道编码 + OTA房型ID 找到门店/房型
	var storeID, channelID, roomTypeID int64
	err := pool.QueryRow(r.Context(), `
		SELECT c.store_id, c.id, m.room_type_id
		FROM ota_channel c
		JOIN ota_room_mapping m ON m.channel_id = c.id
		WHERE c.channel_code = $1 AND m.ota_room_type_id = $2 AND c.status=1 AND m.status=1
		LIMIT 1`, req.ChannelCode, req.OtaRoomID,
	).Scan(&storeID, &channelID, &roomTypeID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "找不到对应的渠道或房型映射"})
		return
	}

	rawBytes, _ := json.Marshal(req)
	// 幂等：同渠道同订单号已存在则直接返回
	var existID int64
	_ = pool.QueryRow(r.Context(),
		`SELECT id FROM ota_order WHERE channel_id=$1 AND ota_order_no=$2`, channelID, req.OtaOrderNo,
	).Scan(&existID)
	if existID > 0 {
		writeJSON(w, http.StatusOK, map[string]any{"id": existID, "message": "订单已存在，跳过", "duplicate": true})
		return
	}

	nights := req.Nights
	if nights <= 0 {
		nights = 1
	}
	var id int64
	if err := pool.QueryRow(r.Context(),
		`INSERT INTO ota_order (store_id, channel_id, ota_order_no, customer_name, customer_phone,
		                        check_in_date, check_out_date, room_type_id, price, nights, status, source, raw_data)
		 VALUES ($1,$2,$3,$4,$5,$6::date,$7::date,$8,$9,$10,0,'callback',$11) RETURNING id`,
		storeID, channelID, req.OtaOrderNo, req.CustomerName, req.CustomerPhone,
		req.CheckInDate, req.CheckOutDate, roomTypeID, req.Price, nights, string(rawBytes),
	).Scan(&id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": 0, "message": "订单已接收，待确认"})
}

// ListOtaOrders OTA 订单列表（GET /api/v1/ota/orders?store_id=&channel_id=&status=）。
func ListOtaOrders(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	storeID := queryInt64(r, "store_id")
	channelID := queryInt64(r, "channel_id")
	status := -1
	if s := r.URL.Query().Get("status"); s != "" {
		fmt.Sscanf(s, "%d", &status)
	}
	u := currentUser(r)

	query := `SELECT o.id, o.store_id, s.name, o.channel_id, c.name, c.channel_code, o.ota_order_no,
	                 o.customer_name, COALESCE(o.customer_phone,''), o.check_in_date, o.check_out_date,
	                 o.room_type_id, rt.name, o.price, o.nights, o.status, COALESCE(o.reservation_id,0),
	                 o.source, o.created_at
	          FROM ota_order o
	          JOIN store s ON s.id = o.store_id
	          JOIN ota_channel c ON c.id = o.channel_id
	          JOIN room_type rt ON rt.id = o.room_type_id
	          WHERE 1=1`
	args := []any{}
	idx := 1
	if storeID > 0 {
		if u != nil && !u.IsAdmin && !u.canAccessStore(storeID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
			return
		}
		query += fmt.Sprintf(" AND o.store_id = $%d", idx)
		args = append(args, storeID)
		idx++
	} else if u != nil && !u.IsAdmin {
		if len(u.StoreIDs) == 0 {
			query += " AND FALSE"
		} else {
			query += fmt.Sprintf(" AND o.store_id = ANY($%d)", idx)
			args = append(args, u.StoreIDs)
			idx++
		}
	}
	if channelID > 0 {
		query += fmt.Sprintf(" AND o.channel_id = $%d", idx)
		args = append(args, channelID)
		idx++
	}
	if status >= 0 {
		query += fmt.Sprintf(" AND o.status = $%d", idx)
		args = append(args, status)
		idx++
	}
	query += " ORDER BY o.created_at DESC LIMIT 200"

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type orderItem struct {
		ID            int64     `json:"id"`
		StoreID       int64     `json:"store_id"`
		StoreName     string    `json:"store_name"`
		ChannelID     int64     `json:"channel_id"`
		ChannelName   string    `json:"channel_name"`
		ChannelCode   string    `json:"channel_code"`
		OtaOrderNo    string    `json:"ota_order_no"`
		CustomerName  string    `json:"customer_name"`
		CustomerPhone string    `json:"customer_phone"`
		CheckInDate   time.Time `json:"check_in_date"`
		CheckOutDate  time.Time `json:"check_out_date"`
		RoomTypeID    int64     `json:"room_type_id"`
		RoomTypeName  string    `json:"room_type_name"`
		Price         float64   `json:"price"`
		Nights        int       `json:"nights"`
		Status        int       `json:"status"`
		ReservationID int64     `json:"reservation_id"`
		Source        string    `json:"source"`
		CreatedAt     time.Time `json:"created_at"`
	}
	list := make([]orderItem, 0)
	for rows.Next() {
		var o orderItem
		if err := rows.Scan(&o.ID, &o.StoreID, &o.StoreName, &o.ChannelID, &o.ChannelName, &o.ChannelCode, &o.OtaOrderNo,
			&o.CustomerName, &o.CustomerPhone, &o.CheckInDate, &o.CheckOutDate,
			&o.RoomTypeID, &o.RoomTypeName, &o.Price, &o.Nights, &o.Status, &o.ReservationID, &o.Source, &o.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, o)
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": list, "total": len(list)})
}

// ConfirmOtaOrder 确认 OTA 订单 → 原子扣配额 → 创建 PMS 预订 → 推库存。
// 扣不到配额则拒单（超卖防护）。
func ConfirmOtaOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := pathValueInt64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少订单 ID"})
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

	// 锁定订单
	var storeID, channelID, roomTypeID, status int64
	var custName, custPhone string
	var checkIn, checkOut time.Time
	var price float64
	var nights int
	if err := tx.QueryRow(r.Context(),
		`SELECT store_id, channel_id, room_type_id, customer_name, COALESCE(customer_phone,''),
		        check_in_date, check_out_date, price, nights, status
		 FROM ota_order WHERE id=$1 FOR UPDATE`, id,
	).Scan(&storeID, &channelID, &roomTypeID, &custName, &custPhone, &checkIn, &checkOut, &price, &nights, &status); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "订单不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅待处理订单可确认"})
		return
	}

	// 原子扣配额（超卖防护）
	if !ConsumeQuota(r.Context(), pool, channelID, roomTypeID) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "配额不足，无法接单（超卖防护）"})
		return
	}

	// 查/建客户
	var customerID int64
	if custPhone != "" {
		err := tx.QueryRow(r.Context(), `SELECT id FROM customer WHERE phone=$1`, custPhone).Scan(&customerID)
		if err != nil {
			if err := tx.QueryRow(r.Context(),
				`INSERT INTO customer (name, phone) VALUES ($1,$2) RETURNING id`, custName, custPhone,
			).Scan(&customerID); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
	} else {
		if err := tx.QueryRow(r.Context(),
			`INSERT INTO customer (name) VALUES ($1) RETURNING id`, custName,
		).Scan(&customerID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	// 创建 PMS 预订
	var reservationID int64
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO reservation (store_id, customer_id, channel, status, check_in_date, check_out_date, deposit, contact, remark)
		 VALUES ($1,$2,'ota',0,$3,$4,$5,$6,$7) RETURNING id`,
		storeID, customerID, checkIn, checkOut, price, custPhone, fmt.Sprintf("OTA订单 %d", id),
	).Scan(&reservationID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// 预订明细
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO reservation_item (reservation_id, room_type_id, price) VALUES ($1,$2,$3)`,
		reservationID, roomTypeID, price,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// 更新订单状态
	if _, err := tx.Exec(r.Context(),
		`UPDATE ota_order SET status=1, reservation_id=$1, updated_at=now() WHERE id=$2`,
		reservationID, id,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 异步推库存（扣配额后可售减少）
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		PushInventory(ctx, pool, storeID, roomTypeID)
	}()

	writeJSON(w, http.StatusOK, map[string]any{
		"order_id":       id,
		"reservation_id": reservationID,
		"status":         1,
		"ok":             true,
	})
	LogAction(w, r, storeID, "ota_order_confirm", itoa64(id), fmt.Sprintf("确认OTA订单→预订%d", reservationID))
}

// RejectOtaOrder 拒单 → 释放配额（若已扣）→ 订单状态置为已取消。
func RejectOtaOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := pathValueInt64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少订单 ID"})
		return
	}
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	var storeID, channelID, roomTypeID, status int64
	if err := pool.QueryRow(r.Context(),
		`SELECT store_id, channel_id, room_type_id, status FROM ota_order WHERE id=$1`, id,
	).Scan(&storeID, &channelID, &roomTypeID, &status); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "订单不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 0 && status != 1 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "当前状态不可拒单"})
		return
	}
	// 已确认(status=1)的拒单需释放配额
	if status == 1 {
		ReleaseQuota(r.Context(), pool, channelID, roomTypeID)
	}
	if _, err := pool.Exec(r.Context(),
		`UPDATE ota_order SET status=2, updated_at=now() WHERE id=$1`, id,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// 拒单后可售增加，推库存
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		PushInventory(ctx, pool, storeID, roomTypeID)
	}()
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": 2, "ok": true})
	LogAction(w, r, storeID, "ota_order_reject", itoa64(id), "拒单OTA订单")
}

// PullOtaOrders 手动触发拉取（模拟）。
// 演示用：为每个启用渠道随机生成 0-2 个测试订单，模拟从平台拉取。
func PullOtaOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	storeID := queryInt64(r, "store_id")
	if storeID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少 store_id"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	// 查该门店所有渠道映射
	rows, err := pool.Query(r.Context(), `
		SELECT c.id, c.channel_code, m.room_type_id, m.ota_room_type_id
		FROM ota_channel c
		JOIN ota_room_mapping m ON m.channel_id = c.id
		WHERE c.store_id=$1 AND c.status=1 AND m.status=1`, storeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type mapping struct {
		ChannelID, RoomTypeID int64
		ChannelCode, OtaRoom  string
	}
	var mappings []mapping
	for rows.Next() {
		var m mapping
		if rows.Scan(&m.ChannelID, &m.ChannelCode, &m.RoomTypeID, &m.OtaRoom) == nil {
			mappings = append(mappings, m)
		}
	}
	rows.Close()

	// 模拟生成订单（确定性，便于复现）
	now := time.Now()
	pulled := 0
	for i, m := range mappings {
		if i%2 == 0 { // 间隔生成
			continue
		}
		otaNo := fmt.Sprintf("OTA%s%d", m.ChannelCode[:3], now.UnixNano()%100000)
		custName := fmt.Sprintf("客人%d", i+1)
		checkIn := now.AddDate(0, 0, 1)
		checkOut := checkIn.AddDate(0, 0, 1)
		raw := fmt.Sprintf(`{"channel_code":"%s","ota_order_no":"%s","simulated":true}`, m.ChannelCode, otaNo)
		var id int64
		err := pool.QueryRow(r.Context(),
			`INSERT INTO ota_order (store_id, channel_id, ota_order_no, customer_name, customer_phone,
			                        check_in_date, check_out_date, room_type_id, price, nights, status, source, raw_data)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,1,0,'pull',$10) RETURNING id`,
			storeID, m.ChannelID, otaNo, custName, "13900000000",
			checkIn, checkOut, m.RoomTypeID, 300, raw,
		).Scan(&id)
		if err == nil {
			pulled++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"pulled": pulled, "message": "模拟拉取完成"})
	LogAction(w, r, storeID, "ota_order_pull", itoa64(storeID), fmt.Sprintf("模拟拉取 %d 个订单", pulled))
}
