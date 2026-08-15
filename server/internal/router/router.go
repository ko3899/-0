package router

import (
	"net/http"

	"hotel-management/server/internal/handler"
)

// New 构建 HTTP 路由。
// 第一期先挂载健康检查，后续按模块（预订/前台/房态/房价/客户/收银）扩展。
func New() http.Handler {
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/api/v1/ping", handler.Ping)

	// TODO: 业务路由（第一期）
	// mux.HandleFunc("/api/v1/reservations", ...)
	// mux.HandleFunc("/api/v1/checkins", ...)

	return mux
}
