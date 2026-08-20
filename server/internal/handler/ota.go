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
// OTA 渠道管理
// ============================================================

// ListOtaChannels 列出 OTA 渠道（GET /api/v1/ota/channels?store_id=）
func ListOtaChannels(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	storeID := queryInt64(r, "store_id")
	cond, args, forbidden := storeCond(r, storeID, "c.store_id")
	if forbidden {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
		return
	}

	where := "WHERE c.status >= 0"
	if cond != "" {
		where += " AND " + cond
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := fmt.Sprintf(`SELECT c.id, c.store_id, COALESCE(s.name,'') AS store_name, c.name, c.channel_code,
		COALESCE(c.api_url,''), COALESCE(c.hotel_id,''), c.status, c.synced_at, c.created_at
		FROM ota_channel c
		LEFT JOIN store s ON s.id = c.store_id
		%s
		ORDER BY c.store_id, c.id`, where)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	type item struct {
		ID          int64  `json:"id"`
		StoreID     int64  `json:"store_id"`
		StoreName   string `json:"store_name"`
		Name        string `json:"name"`
		ChannelCode string `json:"channel_code"`
		APIURL      string `json:"api_url"`
		HotelID     string `json:"hotel_id"`
		Status      int    `json:"status"`
		SyncedAt    string `json:"synced_at"`
		CreatedAt   string `json:"created_at"`
	}

	channels := []item{}
	for rows.Next() {
		var it item
		var syncedAt, createdAt *time.Time
		if err := rows.Scan(&it.ID, &it.StoreID, &it.StoreName, &it.Name, &it.ChannelCode,
			&it.APIURL, &it.HotelID, &it.Status, &syncedAt, &createdAt); err != nil {
			continue
		}
		if syncedAt != nil {
			it.SyncedAt = syncedAt.Format("2006-01-02 15:04:05")
		}
		if createdAt != nil {
			it.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		}
		channels = append(channels, it)
	}

	writeJSON(w, http.StatusOK, map[string]any{"channels": channels})
}

// CreateOtaChannel 创建 OTA 渠道（POST /api/v1/ota/channels）
func CreateOtaChannel(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	var req struct {
		StoreID     int64  `json:"store_id"`
		Name        string `json:"name"`
		ChannelCode string `json:"channel_code"`
		APIURL      string `json:"api_url"`
		AppKey      string `json:"app_key"`
		AppSecret   string `json:"app_secret"`
		HotelID     string `json:"hotel_id"`
		CallbackURL string `json:"callback_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求体格式错误"})
		return
	}
	if req.StoreID == 0 || req.Name == "" || req.ChannelCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "门店、名称和渠道编码不能为空"})
		return
	}
	if !u.IsAdmin && !u.canAccessStore(req.StoreID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int64
	err := pool.QueryRow(ctx,
		`INSERT INTO ota_channel (store_id, name, channel_code, api_url, app_key, app_secret, hotel_id, callback_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		req.StoreID, req.Name, req.ChannelCode, req.APIURL, req.AppKey, req.AppSecret, req.HotelID, req.CallbackURL,
	).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "message": "渠道创建成功"})
	LogAction(w, r, req.StoreID, "create_ota_channel", req.Name, fmt.Sprintf("OTA渠道: %s(%s)", req.Name, req.ChannelCode))
}

// UpdateOtaChannel 更新 OTA 渠道（PUT /api/v1/ota/channels/{id}）
func UpdateOtaChannel(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	id := pathValueInt64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的渠道 ID"})
		return
	}

	var req struct {
		StoreID     int64  `json:"store_id"`
		Name        string `json:"name"`
		ChannelCode string `json:"channel_code"`
		APIURL      string `json:"api_url"`
		AppKey      string `json:"app_key"`
		AppSecret   string `json:"app_secret"`
		HotelID     string `json:"hotel_id"`
		CallbackURL string `json:"callback_url"`
		Status      *int   `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求体格式错误"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 先查门店校验权限
	var storeID int64
	if err := pool.QueryRow(ctx, `SELECT store_id FROM ota_channel WHERE id = $1`, id).Scan(&storeID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "渠道不存在"})
		return
	}
	if !u.IsAdmin && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	setClauses := "updated_at = now()"
	args := []any{id}
	idx := 2

	if req.Name != "" {
		setClauses += fmt.Sprintf(", name = $%d", idx)
		args = append(args, req.Name)
		idx++
	}
	if req.ChannelCode != "" {
		setClauses += fmt.Sprintf(", channel_code = $%d", idx)
		args = append(args, req.ChannelCode)
		idx++
	}
	if req.APIURL != "" {
		setClauses += fmt.Sprintf(", api_url = $%d", idx)
		args = append(args, req.APIURL)
		idx++
	}
	if req.AppKey != "" {
		setClauses += fmt.Sprintf(", app_key = $%d", idx)
		args = append(args, req.AppKey)
		idx++
	}
	if req.AppSecret != "" {
		setClauses += fmt.Sprintf(", app_secret = $%d", idx)
		args = append(args, req.AppSecret)
		idx++
	}
	if req.HotelID != "" {
		setClauses += fmt.Sprintf(", hotel_id = $%d", idx)
		args = append(args, req.HotelID)
		idx++
	}
	if req.CallbackURL != "" {
		setClauses += fmt.Sprintf(", callback_url = $%d", idx)
		args = append(args, req.CallbackURL)
		idx++
	}
	if req.Status != nil {
		setClauses += fmt.Sprintf(", status = $%d", idx)
		args = append(args, *req.Status)
		idx++
	}

	query := fmt.Sprintf("UPDATE ota_channel SET %s WHERE id = $1", setClauses)
	if _, err := pool.Exec(ctx, query, args...); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "渠道更新成功"})
	LogAction(w, r, storeID, "update_ota_channel", req.Name, "更新OTA渠道配置")
}

// DeleteOtaChannel 删除 OTA 渠道（DELETE /api/v1/ota/channels/{id}）
func DeleteOtaChannel(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	id := pathValueInt64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的渠道 ID"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var storeID int64
	var name string
	if err := pool.QueryRow(ctx, `SELECT store_id, name FROM ota_channel WHERE id = $1`, id).Scan(&storeID, &name); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "渠道不存在"})
		return
	}
	if !u.IsAdmin && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	if _, err := pool.Exec(ctx, `DELETE FROM ota_channel WHERE id = $1`, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "渠道已删除"})
	LogAction(w, r, storeID, "delete_ota_channel", name, "删除OTA渠道")
}

// ============================================================
// OTA 房型映射
// ============================================================

// ListOtaMappings 列出房型映射（GET /api/v1/ota/mappings?channel_id=）
func ListOtaMappings(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	channelID := queryInt64(r, "channel_id")

	where := "WHERE m.status >= 0"
	args := []any{}
	idx := 1

	if channelID > 0 {
		where += fmt.Sprintf(" AND m.channel_id = $%d", idx)
		args = append(args, channelID)
		idx++
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := fmt.Sprintf(`SELECT m.id, m.channel_id, c.name AS channel_name, c.channel_code, m.room_type_id,
		rt.name AS room_type_name, m.ota_room_type_id, COALESCE(m.ota_room_name,''), m.price_factor, m.auto_sync, m.status, m.created_at
		FROM ota_room_mapping m
		JOIN ota_channel c ON c.id = m.channel_id
		JOIN room_type rt ON rt.id = m.room_type_id
		%s
		ORDER BY m.channel_id, m.room_type_id`, where)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	type item struct {
		ID            int64   `json:"id"`
		ChannelID     int64   `json:"channel_id"`
		ChannelName   string  `json:"channel_name"`
		ChannelCode   string  `json:"channel_code"`
		RoomTypeID    int64   `json:"room_type_id"`
		RoomTypeName  string  `json:"room_type_name"`
		OtaRoomTypeID string  `json:"ota_room_type_id"`
		OtaRoomName   string  `json:"ota_room_name"`
		PriceFactor   float64 `json:"price_factor"`
		AutoSync      bool    `json:"auto_sync"`
		Status        int     `json:"status"`
		CreatedAt     string  `json:"created_at"`
	}

	mappings := []item{}
	for rows.Next() {
		var it item
		var createdAt *time.Time
		if err := rows.Scan(&it.ID, &it.ChannelID, &it.ChannelName, &it.ChannelCode, &it.RoomTypeID,
			&it.RoomTypeName, &it.OtaRoomTypeID, &it.OtaRoomName, &it.PriceFactor, &it.AutoSync, &it.Status, &createdAt); err != nil {
			continue
		}
		if createdAt != nil {
			it.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		}
		mappings = append(mappings, it)
	}

	writeJSON(w, http.StatusOK, map[string]any{"mappings": mappings})
}

// CreateOtaMapping 创建房型映射（POST /api/v1/ota/mappings）
func CreateOtaMapping(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	var req struct {
		ChannelID     int64   `json:"channel_id"`
		RoomTypeID    int64   `json:"room_type_id"`
		OtaRoomTypeID string  `json:"ota_room_type_id"`
		OtaRoomName   string  `json:"ota_room_name"`
		PriceFactor   float64 `json:"price_factor"`
		AutoSync      *bool   `json:"auto_sync"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求体格式错误"})
		return
	}
	if req.ChannelID == 0 || req.RoomTypeID == 0 || req.OtaRoomTypeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "渠道、房型和OTA房型ID不能为空"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 校验门店权限
	var storeID int64
	if err := pool.QueryRow(ctx, `SELECT store_id FROM ota_channel WHERE id = $1`, req.ChannelID).Scan(&storeID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "渠道不存在"})
		return
	}
	if !u.IsAdmin && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	if req.PriceFactor <= 0 {
		req.PriceFactor = 1.0
	}
	autoSync := true
	if req.AutoSync != nil {
		autoSync = *req.AutoSync
	}

	var id int64
	err := pool.QueryRow(ctx,
		`INSERT INTO ota_room_mapping (channel_id, room_type_id, ota_room_type_id, ota_room_name, price_factor, auto_sync)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		req.ChannelID, req.RoomTypeID, req.OtaRoomTypeID, req.OtaRoomName, req.PriceFactor, autoSync,
	).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "message": "映射创建成功"})
	LogAction(w, r, storeID, "create_ota_mapping", req.OtaRoomTypeID, "创建OTA房型映射")
}

// UpdateOtaMapping 更新房型映射（PUT /api/v1/ota/mappings/{id}）
func UpdateOtaMapping(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	id := pathValueInt64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的映射 ID"})
		return
	}

	var req struct {
		OtaRoomTypeID string   `json:"ota_room_type_id"`
		OtaRoomName   string   `json:"ota_room_name"`
		PriceFactor   *float64 `json:"price_factor"`
		AutoSync      *bool    `json:"auto_sync"`
		Status        *int     `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求体格式错误"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 校验权限
	var channelID, storeID int64
	if err := pool.QueryRow(ctx,
		`SELECT m.channel_id, c.store_id FROM ota_room_mapping m JOIN ota_channel c ON c.id = m.channel_id WHERE m.id = $1`, id,
	).Scan(&channelID, &storeID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "映射不存在"})
		return
	}
	if !u.IsAdmin && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	setClauses := "updated_at = now()"
	args := []any{id}
	idx := 2

	if req.OtaRoomTypeID != "" {
		setClauses += fmt.Sprintf(", ota_room_type_id = $%d", idx)
		args = append(args, req.OtaRoomTypeID)
		idx++
	}
	if req.OtaRoomName != "" {
		setClauses += fmt.Sprintf(", ota_room_name = $%d", idx)
		args = append(args, req.OtaRoomName)
		idx++
	}
	if req.PriceFactor != nil {
		setClauses += fmt.Sprintf(", price_factor = $%d", idx)
		args = append(args, *req.PriceFactor)
		idx++
	}
	if req.AutoSync != nil {
		setClauses += fmt.Sprintf(", auto_sync = $%d", idx)
		args = append(args, *req.AutoSync)
		idx++
	}
	if req.Status != nil {
		setClauses += fmt.Sprintf(", status = $%d", idx)
		args = append(args, *req.Status)
		idx++
	}

	query := fmt.Sprintf("UPDATE ota_room_mapping SET %s WHERE id = $1", setClauses)
	if _, err := pool.Exec(ctx, query, args...); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "映射更新成功"})
	LogAction(w, r, storeID, "update_ota_mapping", req.OtaRoomTypeID, "更新OTA房型映射")
}

// DeleteOtaMapping 删除房型映射（DELETE /api/v1/ota/mappings/{id}）
func DeleteOtaMapping(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	id := pathValueInt64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的映射 ID"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var channelID, storeID int64
	if err := pool.QueryRow(ctx,
		`SELECT m.channel_id, c.store_id FROM ota_room_mapping m JOIN ota_channel c ON c.id = m.channel_id WHERE m.id = $1`, id,
	).Scan(&channelID, &storeID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "映射不存在"})
		return
	}
	if !u.IsAdmin && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	if _, err := pool.Exec(ctx, `DELETE FROM ota_room_mapping WHERE id = $1`, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "映射已删除"})
	LogAction(w, r, storeID, "delete_ota_mapping", fmt.Sprintf("%d", channelID), "删除OTA房型映射")
}

// ============================================================
// 房态预览（PMS 当前可售房量 → 准备同步到 OTA）
// ============================================================

// OtaInventoryPreview 预览当前可同步的房态数据（GET /api/v1/ota/inventory?channel_id=）
func OtaInventoryPreview(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	channelID := queryInt64(r, "channel_id")
	if channelID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请指定渠道 ID"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 校验权限
	var storeID int64
	var channelName string
	if err := pool.QueryRow(ctx, `SELECT store_id, name FROM ota_channel WHERE id = $1`, channelID).Scan(&storeID, &channelName); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "渠道不存在"})
		return
	}
	if !u.IsAdmin && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
		return
	}

	// 查询该渠道下所有启用的映射 + 对应房型的可用房数
	rows, err := pool.Query(ctx,
		`SELECT m.id, m.room_type_id, rt.name AS room_type_name, m.ota_room_type_id, COALESCE(m.ota_room_name,''), m.price_factor,
			COUNT(r.id) FILTER (WHERE r.status = 0) AS available,
			COUNT(r.id) AS total
		 FROM ota_room_mapping m
		 JOIN room_type rt ON rt.id = m.room_type_id
		 LEFT JOIN room r ON r.room_type_id = m.room_type_id AND r.store_id = $1
		 WHERE m.channel_id = $2 AND m.status = 1 AND m.auto_sync = true
		 GROUP BY m.id, m.room_type_id, rt.name, m.ota_room_type_id, m.ota_room_name, m.price_factor
		 ORDER BY rt.name`,
		storeID, channelID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	type roomItem struct {
		MappingID    int64   `json:"mapping_id"`
		RoomTypeID   int64   `json:"room_type_id"`
		RoomTypeName string  `json:"room_type_name"`
		OtaRoomID    string  `json:"ota_room_type_id"`
		OtaRoomName  string  `json:"ota_room_name"`
		PriceFactor  float64 `json:"price_factor"`
		Available    int64   `json:"available"`
		Total        int64   `json:"total"`
		SyncStatus   string  `json:"sync_status"`
	}

	items := []roomItem{}
	for rows.Next() {
		var it roomItem
		if err := rows.Scan(&it.MappingID, &it.RoomTypeID, &it.RoomTypeName, &it.OtaRoomID, &it.OtaRoomName,
			&it.PriceFactor, &it.Available, &it.Total); err != nil {
			continue
		}
		if it.Available > 0 {
			it.SyncStatus = "open"
		} else {
			it.SyncStatus = "closed"
		}
		items = append(items, it)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"channel_id":   channelID,
		"channel_name": channelName,
		"store_id":     storeID,
		"rooms":        items,
	})
}

// ============================================================
// 手动同步（触发房态推送到 OTA）
// ============================================================

// SyncOtaChannel 手动触发同步（POST /api/v1/ota/channels/{id}/sync）
func SyncOtaChannel(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	channelID := pathValueInt64(r, "id")
	if channelID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的渠道 ID"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var storeID int64
	var channelName, channelCode string
	if err := pool.QueryRow(ctx, `SELECT store_id, name, channel_code FROM ota_channel WHERE id = $1`, channelID).Scan(&storeID, &channelName, &channelCode); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "渠道不存在"})
		return
	}
	if !u.IsAdmin && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	// 查询该渠道下所有启用的映射 + 对应房型的可用房数
	rows, err := pool.Query(ctx,
		`SELECT m.id, m.room_type_id, rt.name, m.ota_room_type_id, m.price_factor,
			COUNT(r.id) FILTER (WHERE r.status = 0) AS available
		 FROM ota_room_mapping m
		 JOIN room_type rt ON rt.id = m.room_type_id
		 LEFT JOIN room r ON r.room_type_id = m.room_type_id AND r.store_id = $1
		 WHERE m.channel_id = $2 AND m.status = 1 AND m.auto_sync = true
		 GROUP BY m.id, m.room_type_id, rt.name, m.ota_room_type_id, m.price_factor`,
		storeID, channelID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	type syncItem struct {
		MappingID   int64   `json:"mapping_id"`
		RoomTypeID  int64   `json:"room_type_id"`
		RoomName    string  `json:"room_name"`
		OtaRoomID   string  `json:"ota_room_type_id"`
		PriceFactor float64 `json:"price_factor"`
		Available   int64   `json:"available"`
	}
	var items []syncItem
	for rows.Next() {
		var it syncItem
		if rows.Scan(&it.MappingID, &it.RoomTypeID, &it.RoomName, &it.OtaRoomID, &it.PriceFactor, &it.Available) == nil {
			items = append(items, it)
		}
	}

	// 模拟推送：构造请求体并记录日志
	reqBody, _ := json.Marshal(map[string]any{
		"channel_code": channelCode,
		"rooms":        items,
	})

	// 记录同步日志
	_, _ = pool.Exec(ctx,
		`INSERT INTO ota_sync_log (channel_id, sync_type, status, request_body, response_body)
		 VALUES ($1, 'inventory', 'success', $2, '{"message":"模拟同步成功"}')`,
		channelID, string(reqBody),
	)

	// 更新最后同步时间
	_, _ = pool.Exec(ctx, `UPDATE ota_channel SET synced_at = now() WHERE id = $1`, channelID)

	writeJSON(w, http.StatusOK, map[string]any{
		"message":      "同步成功",
		"channel":      channelName,
		"synced_rooms": len(items),
		"mode":         "simulated",
		"detail":       items,
	})
	LogAction(w, r, storeID, "sync_ota", channelName, fmt.Sprintf("手动同步OTA渠道，共 %d 个房型", len(items)))
}

// ============================================================
// 同步日志
// ============================================================

// ListOtaSyncLogs 查询同步日志（GET /api/v1/ota/sync-logs?channel_id=&page=&page_size=）
func ListOtaSyncLogs(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	channelID := queryInt64(r, "channel_id")
	page := queryInt64(r, "page")
	if page < 1 {
		page = 1
	}
	pageSize := queryInt64(r, "page_size")
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	where := "WHERE 1=1"
	args := []any{}
	idx := 1

	if channelID > 0 {
		where += fmt.Sprintf(" AND l.channel_id = $%d", idx)
		args = append(args, channelID)
		idx++
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int64
	countSQL := "SELECT COUNT(*) FROM ota_sync_log l " + where
	if err := pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	offset := (page - 1) * pageSize
	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2

	querySQL := fmt.Sprintf(`SELECT l.id, l.channel_id, c.name AS channel_name, c.channel_code, l.sync_type, l.status,
		COALESCE(l.error_msg,''), l.created_at
		FROM ota_sync_log l
		JOIN ota_channel c ON c.id = l.channel_id
		%s
		ORDER BY l.created_at DESC
		LIMIT $%d OFFSET $%d`, where, limitIdx, offsetIdx)

	rows, err := pool.Query(ctx, querySQL, append(args, pageSize, offset)...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	type logItem struct {
		ID          int64  `json:"id"`
		ChannelID   int64  `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		ChannelCode string `json:"channel_code"`
		SyncType    string `json:"sync_type"`
		Status      string `json:"status"`
		ErrorMsg    string `json:"error_msg"`
		CreatedAt   string `json:"created_at"`
	}

	logs := []logItem{}
	for rows.Next() {
		var it logItem
		var t time.Time
		if err := rows.Scan(&it.ID, &it.ChannelID, &it.ChannelName, &it.ChannelCode,
			&it.SyncType, &it.Status, &it.ErrorMsg, &t); err != nil {
			continue
		}
		it.CreatedAt = t.Format("2006-01-02 15:04:05")
		logs = append(logs, it)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"logs":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
