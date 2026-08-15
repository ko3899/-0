package handler

import (
	"net/http"

	"hotel-management/server/internal/db"
)

// ListRooms 房态查询接口：返回房间列表（房号/楼层/房型/状态），供前台房态图展示。
func ListRooms(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	rows, err := pool.Query(r.Context(),
		`SELECT r.id, r.room_no, COALESCE(r.floor,''), r.status,
		        COALESCE(rt.name,''), COALESCE(rt.bed_type,'')
		 FROM room r
		 JOIN room_type rt ON rt.id = r.room_type_id
		 ORDER BY r.floor, r.room_no`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type room struct {
		ID           int64  `json:"id"`
		RoomNo       string `json:"room_no"`
		Floor        string `json:"floor"`
		Status       int    `json:"status"`
		RoomTypeName string `json:"room_type_name"`
		BedType      string `json:"bed_type"`
	}

	list := make([]room, 0)
	for rows.Next() {
		var item room
		if err := rows.Scan(&item.ID, &item.RoomNo, &item.Floor, &item.Status, &item.RoomTypeName, &item.BedType); err != nil {
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
