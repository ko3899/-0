package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"hotel-management/server/internal/db"
)

// ============================================================
// OTA 同步引擎（方案C·模拟API层）
// 核心函数：
//   PushInventory  房态变化后推送可售房量（可售>0开房，=0关房）
//   PushRate       房价变化后推送价格
//   callChannelAPI 模拟HTTP调用OTA平台（后续接真实API只改此处）
// 超卖防护：可推房量 = min(物理空净房, 渠道配额-已用)
// ============================================================

// channelInfo 渠道基本信息（推送时使用）
type channelInfo struct {
	ID          int64
	StoreID     int64
	Name        string
	ChannelCode string
	APIURL      string
	AppKey      string
	AppSecret   string
	HotelID     string
}

// roomTypeBrief 房型摘要
type roomTypeBrief struct {
	ID      int64
	StoreID int64
	Name    string
}

// activeMappings 查询某门店某房型所有启用自动同步的渠道映射。
func activeMappings(ctx context.Context, pool *pgxpool.Pool, storeID, roomTypeID int64) ([]struct {
	Channel   channelInfo
	OtaRoomID string
	OtaRoomNm string
	PriceFtr  float64
	Quota     int
	Used      int
}, error) {
	rows, err := pool.Query(ctx, `
		SELECT c.id, c.store_id, c.name, c.channel_code, COALESCE(c.api_url,''),
		       COALESCE(c.app_key,''), COALESCE(c.app_secret,''), COALESCE(c.hotel_id,''),
		       m.ota_room_type_id, COALESCE(m.ota_room_name,''), m.price_factor,
		       COALESCE(q.quota,0), COALESCE(q.used,0)
		FROM ota_room_mapping m
		JOIN ota_channel c ON c.id = m.channel_id
		LEFT JOIN ota_quota q ON q.channel_id = m.channel_id AND q.room_type_id = m.room_type_id
		WHERE m.room_type_id = $1 AND m.status = 1 AND m.auto_sync = true AND c.status = 1
		  AND ($2 = 0 OR c.store_id = $2)`, roomTypeID, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []struct {
		Channel   channelInfo
		OtaRoomID string
		OtaRoomNm string
		PriceFtr  float64
		Quota     int
		Used      int
	}
	for rows.Next() {
		var item struct {
			Channel   channelInfo
			OtaRoomID string
			OtaRoomNm string
			PriceFtr  float64
			Quota     int
			Used      int
		}
		if err := rows.Scan(&item.Channel.ID, &item.Channel.StoreID, &item.Channel.Name, &item.Channel.ChannelCode,
			&item.Channel.APIURL, &item.Channel.AppKey, &item.Channel.AppSecret, &item.Channel.HotelID,
			&item.OtaRoomID, &item.OtaRoomNm, &item.PriceFtr, &item.Quota, &item.Used); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

// physicalAvailable 计算某房型当前物理空净房数（status=0）。
func physicalAvailable(ctx context.Context, pool *pgxpool.Pool, storeID, roomTypeID int64) (int, error) {
	var cnt int
	err := pool.QueryRow(ctx,
		`SELECT count(*) FROM room WHERE store_id=$1 AND room_type_id=$2 AND status=0`,
		storeID, roomTypeID).Scan(&cnt)
	return cnt, err
}

// roomTypeStoreID 查房型所属门店。
func roomTypeStoreID(ctx context.Context, pool *pgxpool.Pool, roomTypeID int64) (int64, error) {
	var storeID int64
	err := pool.QueryRow(ctx, `SELECT store_id FROM room_type WHERE id=$1`, roomTypeID).Scan(&storeID)
	return storeID, err
}

// PushInventory 推送某房型的可售库存到其所有启用渠道。
// 可推房量 = min(物理空净房, 渠道配额-已用)；>0 推 open+房量，=0 推 close。
// 失败不阻断主流程，仅记日志。
func PushInventory(ctx context.Context, pool *pgxpool.Pool, storeID, roomTypeID int64) {
	if pool == nil {
		return
	}
	// 兜底查门店
	if storeID == 0 {
		var err error
		storeID, err = roomTypeStoreID(ctx, pool, roomTypeID)
		if err != nil {
			return
		}
	}

	avail, err := physicalAvailable(ctx, pool, storeID, roomTypeID)
	if err != nil {
		return
	}

	mappings, err := activeMappings(ctx, pool, storeID, roomTypeID)
	if err != nil || len(mappings) == 0 {
		return
	}

	var rtName string
	_ = pool.QueryRow(ctx, `SELECT name FROM room_type WHERE id=$1`, roomTypeID).Scan(&rtName)

	for _, m := range mappings {
		// 可推房量 = min(物理空净, 配额-已用)
		pushable := avail
		if m.Quota > 0 {
			remaining := m.Quota - m.Used
			if remaining < pushable {
				pushable = remaining
			}
		}

		action := "open"
		if pushable <= 0 {
			action = "close"
			pushable = 0
		}

		payload := map[string]any{
			"hotel_id":       m.Channel.HotelID,
			"ota_room_id":    m.OtaRoomID,
			"room_type_name": rtName,
			"available":      pushable,
			"action":         action,
		}

		status, errMsg := callChannelAPI(ctx, m.Channel, "inventory", payload)
		logPush(ctx, pool, m.Channel.StoreID, m.Channel.ID, roomTypeID, "inventory", action, payload, status, errMsg)
	}
}

// PushRate 推送某房型某日价格到其所有启用渠道。
func PushRate(ctx context.Context, pool *pgxpool.Pool, storeID, roomTypeID int64, bizDate time.Time, price float64) {
	if pool == nil || price <= 0 {
		return
	}
	if storeID == 0 {
		var err error
		storeID, err = roomTypeStoreID(ctx, pool, roomTypeID)
		if err != nil {
			return
		}
	}

	mappings, err := activeMappings(ctx, pool, storeID, roomTypeID)
	if err != nil || len(mappings) == 0 {
		return
	}

	var rtName string
	_ = pool.QueryRow(ctx, `SELECT name FROM room_type WHERE id=$1`, roomTypeID).Scan(&rtName)

	for _, m := range mappings {
		otaPrice := price * m.PriceFtr
		payload := map[string]any{
			"hotel_id":    m.Channel.HotelID,
			"ota_room_id": m.OtaRoomID,
			"room_type":   rtName,
			"biz_date":    bizDate.Format("2006-01-02"),
			"price":       otaPrice,
		}
		status, errMsg := callChannelAPI(ctx, m.Channel, "rate", payload)
		logPush(ctx, pool, m.Channel.StoreID, m.Channel.ID, roomTypeID, "rate", "update", payload, status, errMsg)
	}
}

// callChannelAPI 模拟调用 OTA 平台 API。
// ★ 后续接真实 API 只需改本函数：根据 channel.ChannelCode 路由到各平台 SDK。
// 当前实现：返回 success（模拟），并附带 payload 便于日志追溯。
func callChannelAPI(ctx context.Context, ch channelInfo, action string, payload map[string]any) (status string, errMsg string) {
	// 模拟：真实环境下这里会 http.Post(ch.APIURL, ...) 并解析响应
	_ = payload
	_ = action
	return "success", ""
}

// logPush 记录一条推送明细日志（失败不阻断）。
func logPush(ctx context.Context, pool *pgxpool.Pool, storeID, channelID, roomTypeID int64, pushType, action string, payload map[string]any, status, errMsg string) {
	if pool == nil {
		return
	}
	payloadBytes, _ := json.Marshal(payload)
	var errMsgArg any = nil
	if errMsg != "" {
		errMsgArg = errMsg
	}
	_, _ = pool.Exec(ctx,
		`INSERT INTO ota_push_log (store_id, channel_id, room_type_id, push_type, action, payload, status, error_msg)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		storeID, channelID, roomTypeID, pushType, action, string(payloadBytes), status, errMsgArg)
}

// ============================================================
// 配额管理
// ============================================================

// ListOtaQuotas 渠道配额列表（GET /api/v1/ota/quotas?store_id=&channel_id=）。
func ListOtaQuotas(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	storeID := queryInt64(r, "store_id")
	channelID := queryInt64(r, "channel_id")
	u := currentUser(r)

	query := `SELECT q.id, q.store_id, s.name, q.channel_id, c.name, c.channel_code,
	                 q.room_type_id, rt.name, q.quota, q.used
	          FROM ota_quota q
	          JOIN store s ON s.id = q.store_id
	          JOIN ota_channel c ON c.id = q.channel_id
	          JOIN room_type rt ON rt.id = q.room_type_id
	          WHERE 1=1`
	args := []any{}
	idx := 1
	if storeID > 0 {
		if u != nil && !u.IsAdmin && !u.canAccessStore(storeID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
			return
		}
		query += fmt.Sprintf(" AND q.store_id = $%d", idx)
		args = append(args, storeID)
		idx++
	} else if u != nil && !u.IsAdmin {
		if len(u.StoreIDs) == 0 {
			query += " AND FALSE"
		} else {
			query += fmt.Sprintf(" AND q.store_id = ANY($%d)", idx)
			args = append(args, u.StoreIDs)
			idx++
		}
	}
	if channelID > 0 {
		query += fmt.Sprintf(" AND q.channel_id = $%d", idx)
		args = append(args, channelID)
		idx++
	}
	query += " ORDER BY q.store_id, q.channel_id, q.room_type_id"

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type quotaItem struct {
		ID          int64  `json:"id"`
		StoreID     int64  `json:"store_id"`
		StoreName   string `json:"store_name"`
		ChannelID   int64  `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		ChannelCode string `json:"channel_code"`
		RoomTypeID  int64  `json:"room_type_id"`
		RoomTypeNm  string `json:"room_type_name"`
		Quota       int    `json:"quota"`
		Used        int    `json:"used"`
	}
	list := make([]quotaItem, 0)
	for rows.Next() {
		var q quotaItem
		if err := rows.Scan(&q.ID, &q.StoreID, &q.StoreName, &q.ChannelID, &q.ChannelName, &q.ChannelCode,
			&q.RoomTypeID, &q.RoomTypeNm, &q.Quota, &q.Used); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, q)
	}
	writeJSON(w, http.StatusOK, map[string]any{"quotas": list, "total": len(list)})
}

// UpsertOtaQuota 设置/更新渠道配额（POST /api/v1/ota/quotas）。
func UpsertOtaQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		StoreID    int64 `json:"store_id"`
		ChannelID  int64 `json:"channel_id"`
		RoomTypeID int64 `json:"room_type_id"`
		Quota      int   `json:"quota"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.StoreID == 0 || req.ChannelID == 0 || req.RoomTypeID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "门店/渠道/房型不能为空"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(req.StoreID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	var id int64
	if err := pool.QueryRow(r.Context(), `
		INSERT INTO ota_quota (store_id, channel_id, room_type_id, quota) VALUES ($1,$2,$3,$4)
		ON CONFLICT (channel_id, room_type_id) DO UPDATE SET quota = EXCLUDED.quota, updated_at = now()
		RETURNING id`, req.StoreID, req.ChannelID, req.RoomTypeID, req.Quota,
	).Scan(&id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "ok": true})
	LogAction(w, r, req.StoreID, "ota_quota_set", itoa64(id), fmt.Sprintf("设置配额 %d", req.Quota))

	// 配额变化后触发库存推送
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		PushInventory(ctx, pool, req.StoreID, req.RoomTypeID)
	}()
}

// ConsumeQuota 原子扣减配额（OTA下单时调用）。成功返回 true。
func ConsumeQuota(ctx context.Context, pool *pgxpool.Pool, channelID, roomTypeID int64) bool {
	tag, err := pool.Exec(ctx,
		`UPDATE ota_quota SET used = used + 1, updated_at = now()
		 WHERE channel_id=$1 AND room_type_id=$2 AND quota - used >= 1`,
		channelID, roomTypeID)
	if err != nil {
		return false
	}
	return tag.RowsAffected() > 0
}

// ReleaseQuota 释放配额（退房/取消时调用）。
func ReleaseQuota(ctx context.Context, pool *pgxpool.Pool, channelID, roomTypeID int64) {
	_, _ = pool.Exec(ctx,
		`UPDATE ota_quota SET used = GREATEST(used - 1, 0), updated_at = now()
		 WHERE channel_id=$1 AND room_type_id=$2`, channelID, roomTypeID)
}

// ListOtaPushLogs 推送明细日志（GET /api/v1/ota/push-logs?store_id=&channel_id=&push_type=）。
func ListOtaPushLogs(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	storeID := queryInt64(r, "store_id")
	channelID := queryInt64(r, "channel_id")
	pushType := r.URL.Query().Get("push_type")
	u := currentUser(r)

	query := `SELECT l.id, l.store_id, s.name, l.channel_id, c.name, l.room_type_id, rt.name,
	                 l.push_type, l.action, COALESCE(l.payload,''), l.status, COALESCE(l.error_msg,''), l.created_at
	          FROM ota_push_log l
	          JOIN store s ON s.id = l.store_id
	          JOIN ota_channel c ON c.id = l.channel_id
	          JOIN room_type rt ON rt.id = l.room_type_id
	          WHERE 1=1`
	args := []any{}
	idx := 1
	if storeID > 0 {
		if u != nil && !u.IsAdmin && !u.canAccessStore(storeID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
			return
		}
		query += fmt.Sprintf(" AND l.store_id = $%d", idx)
		args = append(args, storeID)
		idx++
	} else if u != nil && !u.IsAdmin {
		if len(u.StoreIDs) == 0 {
			query += " AND FALSE"
		} else {
			query += fmt.Sprintf(" AND l.store_id = ANY($%d)", idx)
			args = append(args, u.StoreIDs)
			idx++
		}
	}
	if channelID > 0 {
		query += fmt.Sprintf(" AND l.channel_id = $%d", idx)
		args = append(args, channelID)
		idx++
	}
	if pushType != "" {
		query += fmt.Sprintf(" AND l.push_type = $%d", idx)
		args = append(args, pushType)
		idx++
	}
	query += " ORDER BY l.created_at DESC LIMIT 200"

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type logItem struct {
		ID          int64     `json:"id"`
		StoreID     int64     `json:"store_id"`
		StoreName   string    `json:"store_name"`
		ChannelID   int64     `json:"channel_id"`
		ChannelName string    `json:"channel_name"`
		RoomTypeID  int64     `json:"room_type_id"`
		RoomTypeNm  string    `json:"room_type_name"`
		PushType    string    `json:"push_type"`
		Action      string    `json:"action"`
		Payload     string    `json:"payload"`
		Status      string    `json:"status"`
		ErrorMsg    string    `json:"error_msg"`
		CreatedAt   time.Time `json:"created_at"`
	}
	list := make([]logItem, 0)
	for rows.Next() {
		var it logItem
		if err := rows.Scan(&it.ID, &it.StoreID, &it.StoreName, &it.ChannelID, &it.ChannelName, &it.RoomTypeID, &it.RoomTypeNm,
			&it.PushType, &it.Action, &it.Payload, &it.Status, &it.ErrorMsg, &it.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": list, "total": len(list)})
}

// ManualPushInventory 手动触发某门店全房型库存推送（POST /api/v1/ota/push-inventory?store_id=）。
func ManualPushInventory(w http.ResponseWriter, r *http.Request) {
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

	// 查该门店所有房型，逐个推送
	rows, err := pool.Query(r.Context(), `SELECT id FROM room_type WHERE store_id=$1`, storeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var rtIDs []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			rtIDs = append(rtIDs, id)
		}
	}
	rows.Close()

	count := 0
	for _, rtID := range rtIDs {
		PushInventory(r.Context(), pool, storeID, rtID)
		count++
	}
	writeJSON(w, http.StatusOK, map[string]any{"pushed_room_types": count, "ok": true})
	LogAction(w, r, storeID, "ota_push_manual", itoa64(storeID), fmt.Sprintf("手动推送 %d 个房型", count))
}

// PushInventoryByRoom 按房间ID异步推送其所属房型的库存。
// 供房态变化 handler（入住/退房/换房/改状态）在事务提交后调用。
func PushInventoryByRoom(pool *pgxpool.Pool, roomID int64) {
	if pool == nil || roomID == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var storeID, roomTypeID int64
		if err := pool.QueryRow(ctx,
			`SELECT r.store_id, r.room_type_id FROM room r WHERE r.id=$1`, roomID,
		).Scan(&storeID, &roomTypeID); err != nil {
			return
		}
		PushInventory(ctx, pool, storeID, roomTypeID)
	}()
}
