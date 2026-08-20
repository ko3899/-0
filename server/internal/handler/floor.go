package handler

import (
	"net/http"
	"strconv"

	"hotel-management/server/internal/db"
)

// ListFloors 楼层列表接口（按门店，受数据权限约束）。
func ListFloors(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeID := queryInt64(r, "store_id")
	query := `SELECT f.id, f.store_id, f.name, f.sort_order, (SELECT count(*) FROM room r WHERE r.store_id = f.store_id AND r.floor = f.name) AS room_count
	          FROM floor f`
	args := []any{}

	cond, scopeArgs, forbidden := storeCond(r, storeID, "f.store_id")
	if forbidden {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
		return
	}
	if cond != "" {
		query += " WHERE " + cond
		args = append(args, scopeArgs...)
	}

	query += ` ORDER BY f.sort_order, f.name`

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type floor struct {
		ID        int64  `json:"id"`
		StoreID   int64  `json:"store_id"`
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
		RoomCount int    `json:"room_count"`
	}
	list := make([]floor, 0)
	for rows.Next() {
		var f floor
		if err := rows.Scan(&f.ID, &f.StoreID, &f.Name, &f.SortOrder, &f.RoomCount); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, f)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"floors": list, "total": len(list)})
}

// CreateFloor 新增楼层。
func CreateFloor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		StoreID   int64  `json:"store_id"`
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.StoreID <= 0 || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "门店和楼层名称不能为空"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(req.StoreID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	var id int64
	if err := pool.QueryRow(r.Context(),
		`INSERT INTO floor (store_id, name, sort_order) VALUES ($1, $2, $3) RETURNING id`,
		req.StoreID, req.Name, req.SortOrder,
	).Scan(&id); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "楼层名称已存在或创建失败: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": req.Name})
}

// UpdateFloor 修改楼层名称或排序。
func UpdateFloor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	floorID := pathID(r.URL.Path)
	if floorID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少楼层 ID"})
		return
	}

	var req struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "楼层名称不能为空"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	var storeID int64
	var oldName string
	if err := pool.QueryRow(r.Context(),
		`SELECT store_id, name FROM floor WHERE id = $1`, floorID,
	).Scan(&storeID, &oldName); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "楼层不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	tx, err := pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer tx.Rollback(r.Context())

	if _, err := tx.Exec(r.Context(),
		`UPDATE floor SET name = $1, sort_order = $2, updated_at = now() WHERE id = $3`,
		req.Name, req.SortOrder, floorID,
	); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "楼层名称已存在: " + err.Error()})
		return
	}

	// 同时更新房间的 floor 字段（保持引用一致）
	if oldName != req.Name {
		if _, err := tx.Exec(r.Context(),
			`UPDATE room SET floor = $1, updated_at = now() WHERE store_id = $2 AND floor = $3`,
			req.Name, storeID, oldName,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": floorID, "name": req.Name})
}

// DeleteFloor 删除楼层（仅当该楼层下无房间时允许）。
func DeleteFloor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	floorID := pathID(r.URL.Path)
	if floorID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少楼层 ID"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	var storeID int64
	var name string
	if err := pool.QueryRow(r.Context(),
		`SELECT store_id, name FROM floor WHERE id = $1`, floorID,
	).Scan(&storeID, &name); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "楼层不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	var roomCount int
	if err := pool.QueryRow(r.Context(),
		`SELECT count(*) FROM room WHERE store_id = $1 AND floor = $2`, storeID, name,
	).Scan(&roomCount); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if roomCount > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "该楼层下还有 " + strconv.Itoa(roomCount) + " 个房间，请先删除房间"})
		return
	}

	if _, err := pool.Exec(r.Context(),
		`DELETE FROM floor WHERE id = $1`, floorID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": floorID})
}

// CreateRoom 新增房间。
func CreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		StoreID    int64  `json:"store_id"`
		RoomTypeID int64  `json:"room_type_id"`
		RoomNo     string `json:"room_no"`
		Floor      string `json:"floor"`
		Status     int    `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.StoreID <= 0 || req.RoomTypeID <= 0 || req.RoomNo == "" || req.Floor == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "门店、房型、房号和楼层不能为空"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(req.StoreID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	var id int64
	if err := pool.QueryRow(r.Context(),
		`INSERT INTO room (store_id, room_type_id, room_no, floor, status) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		req.StoreID, req.RoomTypeID, req.RoomNo, req.Floor, req.Status,
	).Scan(&id); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "房号已存在或创建失败: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "room_no": req.RoomNo})
}

// UpdateRoom 修改房间信息。
func UpdateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	roomID := pathID(r.URL.Path)
	if roomID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少房间 ID"})
		return
	}

	var req struct {
		RoomTypeID int64  `json:"room_type_id"`
		RoomNo     string `json:"room_no"`
		Floor      string `json:"floor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	var storeID int64
	if err := pool.QueryRow(r.Context(),
		`SELECT store_id FROM room WHERE id = $1`, roomID,
	).Scan(&storeID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "房间不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	if _, err := pool.Exec(r.Context(),
		`UPDATE room SET room_type_id = COALESCE(NULLIF($1, 0), room_type_id), room_no = COALESCE(NULLIF($2, ''), room_no), floor = COALESCE(NULLIF($3, ''), floor), updated_at = now() WHERE id = $4`,
		req.RoomTypeID, req.RoomNo, req.Floor, roomID,
	); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "更新失败: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": roomID, "updated": true})
}

// DeleteRoom 删除房间（仅当无进行中的入住时允许）。
func DeleteRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	roomID := pathID(r.URL.Path)
	if roomID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少房间 ID"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	var storeID int64
	if err := pool.QueryRow(r.Context(),
		`SELECT store_id FROM room WHERE id = $1`, roomID,
	).Scan(&storeID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "房间不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	var activeCheckIn int
	if err := pool.QueryRow(r.Context(),
		`SELECT count(*) FROM check_in WHERE room_id = $1 AND status = 0`, roomID,
	).Scan(&activeCheckIn); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if activeCheckIn > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "该房间有在住客人，请先退房"})
		return
	}

	if _, err := pool.Exec(r.Context(),
		`DELETE FROM room WHERE id = $1`, roomID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": roomID})
}
