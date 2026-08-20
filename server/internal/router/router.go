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
	mux.HandleFunc("POST /api/v1/rooms", handler.CreateRoom)
	mux.HandleFunc("PUT /api/v1/rooms/{id}", handler.UpdateRoom)
	mux.HandleFunc("DELETE /api/v1/rooms/{id}", handler.DeleteRoom)

	// 楼层管理
	mux.HandleFunc("GET /api/v1/floors", handler.ListFloors)
	mux.HandleFunc("POST /api/v1/floors", handler.CreateFloor)
	mux.HandleFunc("PUT /api/v1/floors/{id}", handler.UpdateFloor)
	mux.HandleFunc("DELETE /api/v1/floors/{id}", handler.DeleteFloor)

	// 入住 / 退房 / 账单
	mux.HandleFunc("POST /api/v1/checkins", handler.CreateCheckIn)
	mux.HandleFunc("GET /api/v1/checkins", handler.ListCheckIns)
	mux.HandleFunc("POST /api/v1/checkins/{id}/checkout", handler.CheckOut)
	mux.HandleFunc("GET /api/v1/folios/{id}", handler.GetFolio)
	// 前台增值：换房 / 续住 / 附加消费 / 中途收款
	mux.HandleFunc("POST /api/v1/checkins/{id}/change-room", handler.ChangeRoom)
	mux.HandleFunc("POST /api/v1/checkins/{id}/extend", handler.ExtendStay)
	mux.HandleFunc("POST /api/v1/checkins/{id}/charges", handler.AddCharge)
	mux.HandleFunc("POST /api/v1/checkins/{id}/payments", handler.AddPayment)

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

	// 操作日志
	mux.HandleFunc("GET /api/v1/operation-logs", handler.ListOperationLogs)

	// 权限管理（用户/角色，写操作仅管理员）
	mux.HandleFunc("GET /api/v1/roles", handler.ListRoles)
	mux.HandleFunc("GET /api/v1/users", handler.ListUsers)
	mux.HandleFunc("POST /api/v1/users", handler.CreateUser)
	mux.HandleFunc("PUT /api/v1/users/{id}", handler.UpdateUser)

	// OTA 渠道对接
	mux.HandleFunc("GET /api/v1/ota/channels", handler.ListOtaChannels)
	mux.HandleFunc("POST /api/v1/ota/channels", handler.CreateOtaChannel)
	mux.HandleFunc("PUT /api/v1/ota/channels/{id}", handler.UpdateOtaChannel)
	mux.HandleFunc("DELETE /api/v1/ota/channels/{id}", handler.DeleteOtaChannel)
	mux.HandleFunc("POST /api/v1/ota/channels/{id}/sync", handler.SyncOtaChannel)
	mux.HandleFunc("GET /api/v1/ota/mappings", handler.ListOtaMappings)
	mux.HandleFunc("POST /api/v1/ota/mappings", handler.CreateOtaMapping)
	mux.HandleFunc("PUT /api/v1/ota/mappings/{id}", handler.UpdateOtaMapping)
	mux.HandleFunc("DELETE /api/v1/ota/mappings/{id}", handler.DeleteOtaMapping)
	mux.HandleFunc("GET /api/v1/ota/inventory", handler.OtaInventoryPreview)
	mux.HandleFunc("GET /api/v1/ota/sync-logs", handler.ListOtaSyncLogs)

	// OTA 同步闭环（库存/价格/配额/订单）
	mux.HandleFunc("GET /api/v1/ota/quotas", handler.ListOtaQuotas)
	mux.HandleFunc("POST /api/v1/ota/quotas", handler.UpsertOtaQuota)
	mux.HandleFunc("POST /api/v1/ota/push-inventory", handler.ManualPushInventory)
	mux.HandleFunc("GET /api/v1/ota/push-logs", handler.ListOtaPushLogs)
	mux.HandleFunc("POST /api/v1/ota/orders/callback", handler.ReceiveOtaOrder)
	mux.HandleFunc("GET /api/v1/ota/orders", handler.ListOtaOrders)
	mux.HandleFunc("POST /api/v1/ota/orders/pull", handler.PullOtaOrders)
	mux.HandleFunc("POST /api/v1/ota/orders/{id}/confirm", handler.ConfirmOtaOrder)
	mux.HandleFunc("POST /api/v1/ota/orders/{id}/reject", handler.RejectOtaOrder)

	// 夜审
	mux.HandleFunc("GET /api/v1/night-audit/current", handler.NightAuditCurrent)
	mux.HandleFunc("GET /api/v1/night-audit/preview", handler.NightAuditPreview)
	mux.HandleFunc("POST /api/v1/night-audit/run", handler.NightAuditRun)
	mux.HandleFunc("GET /api/v1/night-audit/history", handler.NightAuditHistory)

	// 客房清洁管理
	mux.HandleFunc("GET /api/v1/housekeeping/tasks", handler.ListHousekeepingTasks)
	mux.HandleFunc("POST /api/v1/housekeeping/tasks", handler.CreateHousekeepingTask)
	mux.HandleFunc("POST /api/v1/housekeeping/tasks/{id}/assign", handler.AssignHousekeepingTask)
	mux.HandleFunc("POST /api/v1/housekeeping/tasks/{id}/start", handler.StartHousekeepingTask)
	mux.HandleFunc("POST /api/v1/housekeeping/tasks/{id}/submit", handler.SubmitHousekeepingTask)
	mux.HandleFunc("POST /api/v1/housekeeping/tasks/{id}/inspect", handler.InspectHousekeepingTask)
	mux.HandleFunc("GET /api/v1/housekeeping/stats", handler.HousekeepingStats)
	mux.HandleFunc("GET /api/v1/housekeeping/staff", handler.ListHousekeepingStaff)

	// 发票管理
	mux.HandleFunc("GET /api/v1/invoice-titles", handler.ListInvoiceTitles)
	mux.HandleFunc("POST /api/v1/invoice-titles", handler.CreateInvoiceTitle)
	mux.HandleFunc("PUT /api/v1/invoice-titles/{id}", handler.UpdateInvoiceTitle)
	mux.HandleFunc("DELETE /api/v1/invoice-titles/{id}", handler.DeleteInvoiceTitle)
	mux.HandleFunc("GET /api/v1/invoices", handler.ListInvoices)
	mux.HandleFunc("POST /api/v1/invoices", handler.CreateInvoice)
	mux.HandleFunc("GET /api/v1/invoices/summary", handler.InvoiceSummary)
	mux.HandleFunc("GET /api/v1/invoices/{id}", handler.GetInvoice)
	mux.HandleFunc("POST /api/v1/invoices/{id}/void", handler.VoidInvoice)

	return handler.AuthMiddleware(mux)
}
