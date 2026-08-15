package router

import (
	"net/http"

	"hotel-management/server/internal/handler"
)

// New 构建 HTTP 路由。
// 模块：认证 / 门店 / 房态 / 入住退房账单 / 预订 / 客户。
func New() http.Handler {
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/api/v1/ping", handler.Ping)

	// 认证
	mux.HandleFunc("POST /api/v1/auth/login", handler.Login)

	// 门店
	mux.HandleFunc("GET /api/v1/stores", handler.ListStores)

	// 房态
	mux.HandleFunc("GET /api/v1/rooms", handler.ListRooms)
	mux.HandleFunc("GET /api/v1/room-types", handler.ListRoomTypes)
	mux.HandleFunc("POST /api/v1/rooms/{id}/status", handler.UpdateRoomStatus)

	// 入住 / 退房 / 账单
	mux.HandleFunc("POST /api/v1/checkins", handler.CreateCheckIn)
	mux.HandleFunc("GET /api/v1/checkins", handler.ListCheckIns)
	mux.HandleFunc("POST /api/v1/checkins/{id}/checkout", handler.CheckOut)
	mux.HandleFunc("GET /api/v1/folios/{id}", handler.GetFolio)

	// 预订
	mux.HandleFunc("POST /api/v1/reservations", handler.CreateReservation)
	mux.HandleFunc("GET /api/v1/reservations", handler.ListReservations)
	mux.HandleFunc("POST /api/v1/reservations/{id}/checkin", handler.ReservationCheckIn)

	// 客户
	mux.HandleFunc("GET /api/v1/customers", handler.ListCustomers)
	mux.HandleFunc("POST /api/v1/customers", handler.CreateCustomer)

	// 报表
	mux.HandleFunc("GET /api/v1/dashboard", handler.Dashboard)
	mux.HandleFunc("GET /api/v1/reports/revenue", handler.RevenueReport)
	mux.HandleFunc("GET /api/v1/reports/occupancy", handler.OccupancyReport)

	// 房价政策
	mux.HandleFunc("GET /api/v1/rate-plans", handler.ListRatePlans)
	mux.HandleFunc("GET /api/v1/rate-calendar", handler.ListRateCalendar)
	mux.HandleFunc("PUT /api/v1/rate-calendar", handler.UpdateRateCalendar)

	// 会员
	mux.HandleFunc("GET /api/v1/members", handler.ListMembers)
	mux.HandleFunc("POST /api/v1/members/{id}/recharge", handler.RechargeMember)
	mux.HandleFunc("POST /api/v1/members/{id}/points", handler.AdjustMemberPoints)

	return mux
}
