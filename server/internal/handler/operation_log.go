package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hotel-management/server/internal/db"
)

// LogAction 记录一条操作日志（内部调用，不暴露为 HTTP handler）。
// 调用方需传入 *http.Request 以获取用户和 IP 信息。
func LogAction(w http.ResponseWriter, r *http.Request, storeID int64, action, target, detail string) {
	u := currentUser(r)
	if u == nil {
		return
	}
	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.Split(fwd, ",")[0]
	}

	pool := db.Pool()
	if pool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var storeIDArg any = storeID
	if storeID == 0 {
		storeIDArg = nil
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO operation_log (store_id, user_id, username, action, target, detail, ip)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		storeIDArg, u.ID, u.Username, action, target, detail, ip,
	)
	if err != nil {
		return
	}
}

// logRaw 直接写操作日志（不依赖 AuthUser，用于登录等场景）。
func logRaw(w http.ResponseWriter, r *http.Request, storeID int64, userID int64, username, action, target, detail string) {
	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.Split(fwd, ",")[0]
	}
	pool := db.Pool()
	if pool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var storeIDArg any = storeID
	if storeID == 0 {
		storeIDArg = nil
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO operation_log (store_id, user_id, username, action, target, detail, ip)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		storeIDArg, userID, username, action, target, detail, ip,
	)
	if err != nil {
		return
	}
}

// ListOperationLogs 查询操作日志（GET /api/v1/operation-logs）。
// 支持筛选：store_id, action, user_id, keyword(搜索target+detail), start_date, end_date。
// 分页：page, page_size（默认 1/50，最大 200）。
func ListOperationLogs(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先登录"})
		return
	}

	q := r.URL.Query()
	storeID := queryInt64(r, "store_id")
	action := q.Get("action")
	userID := queryInt64(r, "user_id")
	keyword := q.Get("keyword")
	startDate := q.Get("start_date")
	endDate := q.Get("end_date")
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

	if !u.IsAdmin {
		where += fmt.Sprintf(" AND ol.store_id IN (%s)", storePlaceholders(u.StoreIDs, &idx, &args))
	} else if storeID > 0 {
		idx++
		where += fmt.Sprintf(" AND ol.store_id = $%d", idx-1)
		args = append(args, storeID)
	}

	if action != "" {
		idx++
		where += fmt.Sprintf(" AND ol.action = $%d", idx-1)
		args = append(args, action)
	}
	if userID > 0 {
		idx++
		where += fmt.Sprintf(" AND ol.user_id = $%d", idx-1)
		args = append(args, userID)
	}
	if keyword != "" {
		idx++
		where += fmt.Sprintf(" AND (ol.target ILIKE $%d OR ol.detail ILIKE $%d)", idx-1, idx)
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
		idx++
	}
	if startDate != "" {
		idx++
		where += fmt.Sprintf(" AND ol.created_at >= $%d::date", idx-1)
		args = append(args, startDate)
	}
	if endDate != "" {
		idx++
		where += fmt.Sprintf(" AND ol.created_at < ($%d::date + INTERVAL '1 day')", idx-1)
		args = append(args, endDate)
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库未就绪"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int64
	countSQL := "SELECT COUNT(*) FROM operation_log ol " + where
	if err := pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	offset := (page - 1) * pageSize
	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2

	querySQL := fmt.Sprintf(`SELECT ol.id, COALESCE(ol.store_id,0) AS store_id, COALESCE(s.name,'') AS store_name, ol.user_id, ol.username,
		ol.action, ol.target, ol.detail, ol.ip, ol.created_at
		FROM operation_log ol
		LEFT JOIN store s ON s.id = ol.store_id
		%s
		ORDER BY ol.created_at DESC
		LIMIT $%d OFFSET $%d`, where, limitIdx, offsetIdx)

	rows, err := pool.Query(ctx, querySQL, append(args, pageSize, offset)...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	type logItem struct {
		ID        int64  `json:"id"`
		StoreID   int64  `json:"store_id"`
		StoreName string `json:"store_name"`
		UserID    int64  `json:"user_id"`
		Username  string `json:"username"`
		Action    string `json:"action"`
		Target    string `json:"target"`
		Detail    string `json:"detail"`
		IP        string `json:"ip"`
		CreatedAt string `json:"created_at"`
	}

	logs := []logItem{}
	for rows.Next() {
		var item logItem
		var t time.Time
		if err := rows.Scan(&item.ID, &item.StoreID, &item.StoreName, &item.UserID, &item.Username,
			&item.Action, &item.Target, &item.Detail, &item.IP, &t); err != nil {
			continue
		}
		item.CreatedAt = t.Format("2006-01-02 15:04:05")
		logs = append(logs, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"logs":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// storePlaceholders 生成 IN ($1,$2,...) 占位符并在 args 中追加门店 ID。
func storePlaceholders(storeIDs []int64, idx *int, args *[]any) string {
	parts := make([]string, len(storeIDs))
	for i, id := range storeIDs {
		*idx++
		parts[i] = fmt.Sprintf("$%d", *idx-1)
		*args = append(*args, id)
	}
	return strings.Join(parts, ",")
}
