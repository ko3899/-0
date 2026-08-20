package handler

import (
	"fmt"
	"net/http"
	"time"

	"hotel-management/server/internal/db"
)

// ============================================================
// 客房清洁管理（Housekeeping）
// 任务流转：待分配(0)→已分配(1)→清洁中(2)→待查房(3)→已完成(4)/需维修(5)
//   * 退房时自动生成「退房清洁」任务（见 checkin.go CheckOut）
//   * 查房通过：任务完成 + 房间转空净(0)
//   * 查房不通过（需维修）：任务转需维修 + 房间转维修(3)，并生成新清洁任务待维修结束后
// ============================================================

// ListHousekeepingTasks 清洁任务列表（GET /api/v1/housekeeping/tasks）。
// 过滤：store_id, status, assigned_to, room_no。
func ListHousekeepingTasks(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeID := queryInt64(r, "store_id")
	status := -1
	if s := r.URL.Query().Get("status"); s != "" {
		var st int
		if _, err := fmt.Sscanf(s, "%d", &st); err == nil {
			status = st
		}
	}
	assignedTo := queryInt64(r, "assigned_to")
	roomNo := r.URL.Query().Get("room_no")

	u := currentUser(r)
	query := `SELECT t.id, t.store_id, s.name, t.room_id, r.room_no, r.floor, rt.name,
	                 t.check_in_id, t.task_type, t.status, t.priority,
	                 COALESCE(au.name,''), t.assigned_at, t.started_at, t.submitted_at, t.completed_at,
	                 COALESCE(ins.name,''), COALESCE(t.remark,''), t.created_at
	          FROM housekeeping_task t
	          JOIN store s ON s.id = t.store_id
	          JOIN room r ON r.id = t.room_id
	          LEFT JOIN room_type rt ON rt.id = r.room_type_id
	          LEFT JOIN users au ON au.id = t.assigned_to
	          LEFT JOIN users ins ON ins.id = t.inspector
	          WHERE 1=1`
	args := []any{}
	idx := 1

	if storeID > 0 {
		if u != nil && !u.IsAdmin && !u.canAccessStore(storeID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
			return
		}
		query += fmt.Sprintf(" AND t.store_id = $%d", idx)
		args = append(args, storeID)
		idx++
	} else if u != nil && !u.IsAdmin {
		if len(u.StoreIDs) == 0 {
			query += " AND FALSE"
		} else {
			query += fmt.Sprintf(" AND t.store_id = ANY($%d)", idx)
			args = append(args, u.StoreIDs)
			idx++
		}
	}
	if status >= 0 {
		query += fmt.Sprintf(" AND t.status = $%d", idx)
		args = append(args, status)
		idx++
	}
	if assignedTo > 0 {
		query += fmt.Sprintf(" AND t.assigned_to = $%d", idx)
		args = append(args, assignedTo)
		idx++
	}
	if roomNo != "" {
		query += fmt.Sprintf(" AND r.room_no ILIKE $%d", idx)
		args = append(args, "%"+roomNo+"%")
		idx++
	}
	query += " ORDER BY t.priority DESC, t.created_at DESC LIMIT 300"

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type task struct {
		ID          int64      `json:"id"`
		StoreID     int64      `json:"store_id"`
		StoreName   string     `json:"store_name"`
		RoomID      int64      `json:"room_id"`
		RoomNo      string     `json:"room_no"`
		Floor       string     `json:"floor"`
		RoomType    string     `json:"room_type"`
		CheckInID   int64      `json:"check_in_id"`
		TaskType    int        `json:"task_type"`
		Status      int        `json:"status"`
		Priority    int        `json:"priority"`
		AssignedTo  string     `json:"assigned_to"`
		AssignedAt  *time.Time `json:"assigned_at"`
		StartedAt   *time.Time `json:"started_at"`
		SubmittedAt *time.Time `json:"submitted_at"`
		CompletedAt *time.Time `json:"completed_at"`
		Inspector   string     `json:"inspector"`
		Remark      string     `json:"remark"`
		CreatedAt   time.Time  `json:"created_at"`
	}
	list := make([]task, 0)
	for rows.Next() {
		var t task
		if err := rows.Scan(&t.ID, &t.StoreID, &t.StoreName, &t.RoomID, &t.RoomNo, &t.Floor, &t.RoomType,
			&t.CheckInID, &t.TaskType, &t.Status, &t.Priority, &t.AssignedTo,
			&t.AssignedAt, &t.StartedAt, &t.SubmittedAt, &t.CompletedAt, &t.Inspector, &t.Remark, &t.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": list, "total": len(list)})
}

// CreateHousekeepingTask 手动创建清洁任务（POST /api/v1/housekeeping/tasks）。
func CreateHousekeepingTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		StoreID  int64  `json:"store_id"`
		RoomID   int64  `json:"room_id"`
		TaskType int    `json:"task_type"`
		Priority int    `json:"priority"`
		Remark   string `json:"remark"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.StoreID == 0 || req.RoomID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "门店和房间不能为空"})
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
	if err := pool.QueryRow(r.Context(),
		`INSERT INTO housekeeping_task (store_id, room_id, task_type, status, priority, remark) VALUES ($1, $2, $3, 0, $4, $5) RETURNING id`,
		req.StoreID, req.RoomID, req.TaskType, req.Priority, req.Remark,
	).Scan(&id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
	LogAction(w, r, req.StoreID, "hk_create", itoa64(id), "创建清洁任务")
}

// AssignHousekeepingTask 分配服务员（POST /api/v1/housekeeping/tasks/{id}/assign）。
func AssignHousekeepingTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	taskID := pathValueInt64(r, "id")
	if taskID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少任务 ID"})
		return
	}
	var req struct {
		AssignedTo int64 `json:"assigned_to"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.AssignedTo == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请选择服务员"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	var storeID, status int64
	if err := pool.QueryRow(r.Context(), `SELECT store_id, status FROM housekeeping_task WHERE id = $1`, taskID).Scan(&storeID, &status); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "任务不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅待分配状态可分配"})
		return
	}
	if _, err := pool.Exec(r.Context(),
		`UPDATE housekeeping_task SET assigned_to = $1, status = 1, assigned_at = now(), updated_at = now() WHERE id = $2`,
		req.AssignedTo, taskID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": taskID, "ok": true})
	LogAction(w, r, storeID, "hk_assign", itoa64(taskID), "分配清洁任务")
}

// StartHousekeepingTask 开始清洁（POST /api/v1/housekeeping/tasks/{id}/start）。
func StartHousekeepingTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	taskID := pathValueInt64(r, "id")
	if taskID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少任务 ID"})
		return
	}
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	var storeID, status int64
	if err := pool.QueryRow(r.Context(), `SELECT store_id, status FROM housekeeping_task WHERE id = $1`, taskID).Scan(&storeID, &status); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "任务不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 1 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅已分配状态可开始"})
		return
	}
	if _, err := pool.Exec(r.Context(),
		`UPDATE housekeeping_task SET status = 2, started_at = now(), updated_at = now() WHERE id = $1`, taskID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": taskID, "ok": true})
}

// SubmitHousekeepingTask 提交查房（POST /api/v1/housekeeping/tasks/{id}/submit）。
func SubmitHousekeepingTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	taskID := pathValueInt64(r, "id")
	if taskID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少任务 ID"})
		return
	}
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	var storeID, status int64
	if err := pool.QueryRow(r.Context(), `SELECT store_id, status FROM housekeeping_task WHERE id = $1`, taskID).Scan(&storeID, &status); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "任务不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 2 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅清洁中状态可提交"})
		return
	}
	if _, err := pool.Exec(r.Context(),
		`UPDATE housekeeping_task SET status = 3, submitted_at = now(), updated_at = now() WHERE id = $1`, taskID,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": taskID, "ok": true})
}

// InspectHousekeepingTask 查房（POST /api/v1/housekeeping/tasks/{id}/inspect）。
// pass=true：任务完成 + 房间转空净(0)；pass=false：任务转需维修(5) + 房间转维修(3)。
func InspectHousekeepingTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	taskID := pathValueInt64(r, "id")
	if taskID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少任务 ID"})
		return
	}
	var req struct {
		Pass   bool   `json:"pass"`
		Remark string `json:"remark"`
	}
	if !decodeJSON(w, r, &req) {
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

	var storeID, roomID, status int64
	if err := tx.QueryRow(r.Context(), `SELECT store_id, room_id, status FROM housekeeping_task WHERE id = $1 FOR UPDATE`, taskID).Scan(&storeID, &roomID, &status); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "任务不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 3 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅待查房状态可查房"})
		return
	}

	opID := int64(0)
	if u := currentUser(r); u != nil {
		opID = u.ID
	}

	if req.Pass {
		// 查房通过：任务完成 + 房间转空净
		if _, err := tx.Exec(r.Context(),
			`UPDATE housekeeping_task SET status = 4, completed_at = now(), inspector = $1, remark = $2, updated_at = now() WHERE id = $3`,
			opID, req.Remark, taskID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE room SET status = 0, updated_at = now() WHERE id = $1`, roomID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	} else {
		// 查房不通过：任务转需维修 + 房间转维修
		if _, err := tx.Exec(r.Context(),
			`UPDATE housekeeping_task SET status = 5, inspector = $1, remark = $2, updated_at = now() WHERE id = $3`,
			opID, req.Remark, taskID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE room SET status = 3, updated_at = now() WHERE id = $1`, roomID,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	action := "hk_inspect_pass"
	detail := "查房通过，房间转空净"
	if !req.Pass {
		action = "hk_inspect_fail"
		detail = "查房不通过，房间转维修"
	}
	// 查房通过后房间转空净，可售增加，异步推送 OTA 库存
	if req.Pass {
		PushInventoryByRoom(pool, roomID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": taskID, "ok": true, "pass": req.Pass})
	LogAction(w, r, storeID, action, itoa64(taskID), detail)
}

// HousekeepingStats 清洁工作量统计（GET /api/v1/housekeeping/stats?store_id=&start=&end=）。
func HousekeepingStats(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	storeID := queryInt64(r, "store_id")
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	if start == "" {
		start = time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	}
	if end == "" {
		end = time.Now().Format("2006-01-02")
	}

	u := currentUser(r)
	where := "WHERE t.created_at::date BETWEEN $1::date AND $2::date"
	args := []any{start, end}
	idx := 3
	if storeID > 0 {
		if u != nil && !u.IsAdmin && !u.canAccessStore(storeID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
			return
		}
		where += fmt.Sprintf(" AND t.store_id = $%d", idx)
		args = append(args, storeID)
		idx++
	} else if u != nil && !u.IsAdmin {
		if len(u.StoreIDs) == 0 {
			where += " AND FALSE"
		} else {
			where += fmt.Sprintf(" AND t.store_id = ANY($%d)", idx)
			args = append(args, u.StoreIDs)
			idx++
		}
	}

	// 按服务员统计完成数
	rows, err := pool.Query(r.Context(), `
		SELECT COALESCE(au.name,'未分配'), count(*) FILTER (WHERE t.status = 4),
		       count(*) FILTER (WHERE t.status IN (0,1,2,3)), count(*)
		FROM housekeeping_task t
		LEFT JOIN users au ON au.id = t.assigned_to
		`+where+`
		GROUP BY au.name
		ORDER BY count(*) DESC`, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type staffStat struct {
		Staff     string `json:"staff"`
		Completed int    `json:"completed"`
		Pending   int    `json:"pending"`
		Total     int    `json:"total"`
	}
	list := make([]staffStat, 0)
	var totalCompleted, totalPending, totalAll int
	for rows.Next() {
		var s staffStat
		if err := rows.Scan(&s.Staff, &s.Completed, &s.Pending, &s.Total); err != nil {
			continue
		}
		list = append(list, s)
		totalCompleted += s.Completed
		totalPending += s.Pending
		totalAll += s.Total
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"staff_stats":     list,
		"total_completed": totalCompleted,
		"total_pending":   totalPending,
		"total_all":       totalAll,
		"start":           start,
		"end":             end,
	})
}

// ListHousekeepingStaff 可分配的服务员列表（GET /api/v1/housekeeping/staff?store_id=）。
// 返回对指定门店有数据权限的用户（管理员返回全部门店用户）。
func ListHousekeepingStaff(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	storeID := queryInt64(r, "store_id")
	u := currentUser(r)

	query := `SELECT DISTINCT u.id, u.username, COALESCE(u.name,''), COALESCE(r.name,'')
	          FROM users u
	          LEFT JOIN roles r ON r.id = u.role_id
	          LEFT JOIN user_store us ON us.user_id = u.id
	          WHERE u.status = 1`
	args := []any{}
	if storeID > 0 {
		if u != nil && !u.IsAdmin && !u.canAccessStore(storeID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
			return
		}
		query += " AND us.store_id = $1"
		args = append(args, storeID)
	} else if u != nil && !u.IsAdmin {
		if len(u.StoreIDs) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{"staff": []any{}})
			return
		}
		query += " AND us.store_id = ANY($1)"
		args = append(args, u.StoreIDs)
	}
	query += " ORDER BY u.id"

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type staff struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
		Role     string `json:"role"`
	}
	list := make([]staff, 0)
	for rows.Next() {
		var s staff
		if err := rows.Scan(&s.ID, &s.Username, &s.Name, &s.Role); err != nil {
			continue
		}
		list = append(list, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"staff": list, "total": len(list)})
}
