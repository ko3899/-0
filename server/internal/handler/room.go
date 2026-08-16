package handler

import (
	"net/http"

	"hotel-management/server/internal/db"
)

// ListRooms 房态查询接口：支持 store_id 过滤，按用户数据权限隔离，返回房间列表。
func ListRooms(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeID := queryInt64(r, "store_id")
	query := `SELECT r.id, r.room_no, COALESCE(r.floor,''), r.status,
	                 COALESCE(rt.name,''), COALESCE(rt.bed_type,''), rt.capacity, r.store_id,
	                 COALESCE(ci.id,0), COALESCE(f.balance,0), COALESCE(f.total_amount,0)
	          FROM room r
	          JOIN room_type rt ON rt.id = r.room_type_id
	          LEFT JOIN check_in ci ON ci.room_id = r.id AND ci.status = 0
	          LEFT JOIN folio f ON f.check_in_id = ci.id`
	args := []any{}

	cond, scopeArgs, forbidden := storeCond(r, storeID, "r.store_id")
	if forbidden {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
		return
	}
	if cond != "" {
		query += " WHERE " + cond
		args = append(args, scopeArgs...)
	}

	query += ` ORDER BY r.store_id, r.floor, r.room_no`

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type room struct {
		ID           int64   `json:"id"`
		RoomNo       string  `json:"room_no"`
		Floor        string  `json:"floor"`
		Status       int     `json:"status"`
		RoomTypeName string  `json:"room_type_name"`
		BedType      string  `json:"bed_type"`
		Capacity     int     `json:"capacity"`
		StoreID      int64   `json:"store_id"`
		CheckInID    int64   `json:"check_in_id"`
		Balance      float64 `json:"balance"`
		TotalAmount  float64 `json:"total_amount"`
	}
	list := make([]room, 0)
	for rows.Next() {
		var item room
		if err := rows.Scan(&item.ID, &item.RoomNo, &item.Floor, &item.Status, &item.RoomTypeName, &item.BedType, &item.Capacity, &item.StoreID, &item.CheckInID, &item.Balance, &item.TotalAmount); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rooms": list, "total": len(list)})
}

// ListRoomTypes 房型列表接口（按门店，含数据权限隔离），供预订/入住选房型。
func ListRoomTypes(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeID := queryInt64(r, "store_id")
	query := `SELECT id, store_id, name, COALESCE(bed_type,''), capacity
	          FROM room_type`
	args := []any{}

	cond, scopeArgs, forbidden := storeCond(r, storeID, "store_id")
	if forbidden {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
		return
	}
	if cond != "" {
		query += " WHERE " + cond
		args = append(args, scopeArgs...)
	}

	query += ` ORDER BY id`

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type roomType struct {
		ID       int64  `json:"id"`
		StoreID  int64  `json:"store_id"`
		Name     string `json:"name"`
		BedType  string `json:"bed_type"`
		Capacity int    `json:"capacity"`
	}
	list := make([]roomType, 0)
	for rows.Next() {
		var t roomType
		if err := rows.Scan(&t.ID, &t.StoreID, &t.Name, &t.BedType, &t.Capacity); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"room_types": list, "total": len(list)})
}

// UpdateRoomStatus 房间状态变更接口：清洁(0)/空脏(1)/维修(3)/预留(4)。
// 住客(2)房间必须先退房才能变更；受门店数据权限约束。
func UpdateRoomStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Status int `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	switch req.Status {
	case 0, 1, 3, 4:
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "非法状态值（允许 0/1/3/4）"})
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

	var (
		storeID int64
		cur     int
	)
	if err := pool.QueryRow(r.Context(),
		`SELECT store_id, status FROM room WHERE id = $1`, roomID,
	).Scan(&storeID, &cur); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "房间不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店房间"})
		return
	}
	if cur == 2 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "住客房间请先办理退房"})
		return
	}

	if _, err := pool.Exec(r.Context(),
		`UPDATE room SET status = $1, updated_at = now() WHERE id = $2`, req.Status, roomID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": roomID, "status": req.Status})
}
