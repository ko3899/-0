package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// decodeJSON 解析 JSON 请求体，失败时已写回 400 响应并返回 false。
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求体格式错误"})
		return false
	}
	return true
}

// pathID 从 /api/v1/{resource}/{id}[/{action}] 路径提取资源 ID（第 4 段）。
func pathID(path string) int64 {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		return 0
	}
	id, _ := strconv.ParseInt(parts[3], 10, 64)
	return id
}

// queryInt64 从 query 参数解析 int64，缺省返回 0。
func queryInt64(r *http.Request, key string) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
	return v
}
