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
	mux.HandleFunc("POST /api/v1/auth/logout", handler.Logout)

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
	mux.HandleFunc("PUT /api/v1/reservations/{id}", handler.UpdateReservation)
	mux.HandleFunc("POST /api/v1/reservations/{id}/checkin", handler.ReservationCheckIn)
	mux.HandleFunc("POST /api/v1/reservations/{id}/cancel", handler.CancelReservation)
	mux.HandleFunc("POST /api/v1/reservations/{id}/noshow", handler.ReservationNoShow)

	// 客户
	mux.HandleFunc("GET /api/v1/customers", handler.ListCustomers)
	mux.HandleFunc("POST /api/v1/customers", handler.CreateCustomer)

	// 报表
	mux.HandleFunc("GET /api/v1/dashboard", handler.Dashboard)
	mux.HandleFunc("GET /api/v1/reports/revenue", handler.RevenueReport)
	mux.HandleFunc("GET /api/v1/reports/occupancy", handler.OccupancyReport)
	mux.HandleFunc("GET /api/v1/reports/trend", handler.TrendReport)

	// 房价政策
	mux.HandleFunc("GET /api/v1/rate-plans", handler.ListRatePlans)
	mux.HandleFunc("GET /api/v1/rate-calendar", handler.ListRateCalendar)
	mux.HandleFunc("PUT /api/v1/rate-calendar", handler.UpdateRateCalendar)

	// 会员
	mux.HandleFunc("GET /api/v1/members", handler.ListMembers)
	mux.HandleFunc("POST /api/v1/members/{id}/recharge", handler.RechargeMember)
	mux.HandleFunc("POST /api/v1/members/{id}/points", handler.AdjustMemberPoints)

	// 权限管理（用户/角色，写操作仅管理员）
	mux.HandleFunc("GET /api/v1/roles", handler.ListRoles)
	mux.HandleFunc("GET /api/v1/users", handler.ListUsers)
	mux.HandleFunc("POST /api/v1/users", handler.CreateUser)
	mux.HandleFunc("PUT /api/v1/users/{id}", handler.UpdateUser)

	return handler.AuthMiddleware(mux)
}
