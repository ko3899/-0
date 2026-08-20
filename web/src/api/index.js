const BASE = '/api/v1'

async function request(path, options = {}) {
  const headers = { 'Content-Type': 'application/json' }
  const token = localStorage.getItem('token')
  if (token) headers.Authorization = 'Bearer ' + token
  let res
  try {
    res = await fetch(BASE + path, { ...options, headers })
  } catch (e) {
    throw new Error('网络请求失败，请检查后端服务是否启动')
  }
  if (!res.ok) {
    let msg = '请求失败(' + res.status + ')'
    try {
      const d = await res.json()
      msg = d.error || msg
    } catch {}
    throw new Error(msg)
  }
  return res.json()
}

export const api = {
  login: (data) => request('/auth/login', { method: 'POST', body: JSON.stringify(data) }),
  listStores: () => request('/stores'),
  listRooms: (storeId) => request('/rooms' + (storeId ? `?store_id=${storeId}` : '')),
  listRoomTypes: (storeId) => request('/room-types' + (storeId ? `?store_id=${storeId}` : '')),
  updateRoomStatus: (id, status) => request(`/rooms/${id}/status`, { method: 'POST', body: JSON.stringify({ status }) }),
  createRoom: (data) => request('/rooms', { method: 'POST', body: JSON.stringify(data) }),
  updateRoom: (id, data) => request(`/rooms/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteRoom: (id) => request(`/rooms/${id}`, { method: 'DELETE' }),
  // 楼层管理
  listFloors: (storeId) => request('/floors' + (storeId ? `?store_id=${storeId}` : '')),
  createFloor: (data) => request('/floors', { method: 'POST', body: JSON.stringify(data) }),
  updateFloor: (id, data) => request(`/floors/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteFloor: (id) => request(`/floors/${id}`, { method: 'DELETE' }),
  createCheckIn: (data) => request('/checkins', { method: 'POST', body: JSON.stringify(data) }),
  listCheckIns: (params = {}) => {
    const qs = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => { if (v !== '' && v != null) qs.set(k, v) })
    const s = qs.toString()
    return request('/checkins' + (s ? `?${s}` : ''))
  },
  checkOut: (id, method, amount) => request(`/checkins/${id}/checkout`, { method: 'POST', body: JSON.stringify({ method, amount }) }),
  getFolio: (id) => request(`/folios/${id}`),
  // 前台增值
  changeRoom: (id, data) => request(`/checkins/${id}/change-room`, { method: 'POST', body: JSON.stringify(data) }),
  extendStay: (id, data) => request(`/checkins/${id}/extend`, { method: 'POST', body: JSON.stringify(data) }),
  addCharge: (id, data) => request(`/checkins/${id}/charges`, { method: 'POST', body: JSON.stringify(data) }),
  addPayment: (id, data) => request(`/checkins/${id}/payments`, { method: 'POST', body: JSON.stringify(data) }),
  createReservation: (data) => request('/reservations', { method: 'POST', body: JSON.stringify(data) }),
  listReservations: (params = {}) => {
    const q = new URLSearchParams()
    if (params.store_id) q.set('store_id', params.store_id)
    if (params.status !== undefined && params.status !== null && params.status !== '') q.set('status', params.status)
    const s = q.toString()
    return request('/reservations' + (s ? `?${s}` : ''))
  },
  reservationCheckIn: (id, roomId) => request(`/reservations/${id}/checkin`, { method: 'POST', body: JSON.stringify({ room_id: roomId }) }),
  updateReservation: (id, data) => request(`/reservations/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  cancelReservation: (id) => request(`/reservations/${id}/cancel`, { method: 'POST' }),
  reservationNoShow: (id) => request(`/reservations/${id}/noshow`, { method: 'POST' }),
  listCustomers: (keyword) => request('/customers' + (keyword ? `?keyword=${encodeURIComponent(keyword)}` : '')),
  createCustomer: (data) => request('/customers', { method: 'POST', body: JSON.stringify(data) }),
  dashboard: () => request('/dashboard'),
  revenueReport: () => request('/reports/revenue'),
  occupancyReport: () => request('/reports/occupancy'),
  trendReport: () => request('/reports/trend'),
  listRatePlans: (storeId) => request('/rate-plans' + (storeId ? `?store_id=${storeId}` : '')),
  listRateCalendar: (storeId, start, end) => request(`/rate-calendar?store_id=${storeId}&start=${start}&end=${end}`),
  updateRateCalendar: (data) => request('/rate-calendar', { method: 'PUT', body: JSON.stringify(data) }),
  listMembers: (keyword) => request('/members' + (keyword ? `?keyword=${encodeURIComponent(keyword)}` : '')),
  rechargeMember: (id, amount) => request(`/members/${id}/recharge`, { method: 'POST', body: JSON.stringify({ amount }) }),
  adjustMemberPoints: (id, delta) => request(`/members/${id}/points`, { method: 'POST', body: JSON.stringify({ delta }) }),
  listRoles: () => request('/roles'),
  listUsers: () => request('/users'),
  createUser: (data) => request('/users', { method: 'POST', body: JSON.stringify(data) }),
  updateUser: (id, data) => request(`/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  listOperationLogs: (params) => {
    const qs = new URLSearchParams(params).toString()
    return request('/operation-logs' + (qs ? '?' + qs : ''))
  },
  // OTA 渠道对接
  listOtaChannels: (storeId) => request('/ota/channels' + (storeId ? `?store_id=${storeId}` : '')),
  createOtaChannel: (data) => request('/ota/channels', { method: 'POST', body: JSON.stringify(data) }),
  updateOtaChannel: (id, data) => request(`/ota/channels/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteOtaChannel: (id) => request(`/ota/channels/${id}`, { method: 'DELETE' }),
  syncOtaChannel: (id) => request(`/ota/channels/${id}/sync`, { method: 'POST' }),
  listOtaMappings: (channelId) => request('/ota/mappings' + (channelId ? `?channel_id=${channelId}` : '')),
  createOtaMapping: (data) => request('/ota/mappings', { method: 'POST', body: JSON.stringify(data) }),
  updateOtaMapping: (id, data) => request(`/ota/mappings/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteOtaMapping: (id) => request(`/ota/mappings/${id}`, { method: 'DELETE' }),
  otaInventoryPreview: (channelId) => request(`/ota/inventory?channel_id=${channelId}`),
  listOtaSyncLogs: (params) => {
    const qs = new URLSearchParams(params).toString()
    return request('/ota/sync-logs' + (qs ? '?' + qs : ''))
  },

  // OTA 同步闭环
  listOtaQuotas: (params) => {
    const qs = new URLSearchParams(params).toString()
    return request('/ota/quotas' + (qs ? '?' + qs : ''))
  },
  upsertOtaQuota: (data) => request('/ota/quotas', { method: 'POST', body: JSON.stringify(data) }),
  manualPushInventory: (storeId) => request(`/ota/push-inventory?store_id=${storeId}`, { method: 'POST' }),
  listOtaPushLogs: (params) => {
    const qs = new URLSearchParams(params).toString()
    return request('/ota/push-logs' + (qs ? '?' + qs : ''))
  },
  listOtaOrders: (params) => {
    const qs = new URLSearchParams(params).toString()
    return request('/ota/orders' + (qs ? '?' + qs : ''))
  },
  pullOtaOrders: (storeId) => request(`/ota/orders/pull?store_id=${storeId}`, { method: 'POST' }),
  confirmOtaOrder: (id) => request(`/ota/orders/${id}/confirm`, { method: 'POST' }),
  rejectOtaOrder: (id) => request(`/ota/orders/${id}/reject`, { method: 'POST' }),

  // 夜审
  nightAuditCurrent: () => request('/night-audit/current'),
  nightAuditPreview: () => request('/night-audit/preview'),
  nightAuditRun: () => request('/night-audit/run', { method: 'POST' }),
  nightAuditHistory: (params) => {
    const qs = new URLSearchParams(params).toString()
    return request('/night-audit/history' + (qs ? '?' + qs : ''))
  },

  // 客房清洁管理
  listHousekeepingTasks: (params) => {
    const qs = new URLSearchParams(params).toString()
    return request('/housekeeping/tasks' + (qs ? '?' + qs : ''))
  },
  createHousekeepingTask: (data) => request('/housekeeping/tasks', { method: 'POST', body: JSON.stringify(data) }),
  assignHousekeepingTask: (id, assignedTo) => request(`/housekeeping/tasks/${id}/assign`, { method: 'POST', body: JSON.stringify({ assigned_to: assignedTo }) }),
  startHousekeepingTask: (id) => request(`/housekeeping/tasks/${id}/start`, { method: 'POST' }),
  submitHousekeepingTask: (id) => request(`/housekeeping/tasks/${id}/submit`, { method: 'POST' }),
  inspectHousekeepingTask: (id, data) => request(`/housekeeping/tasks/${id}/inspect`, { method: 'POST', body: JSON.stringify(data) }),
  housekeepingStats: (params) => {
    const qs = new URLSearchParams(params).toString()
    return request('/housekeeping/stats' + (qs ? '?' + qs : ''))
  },
  listHousekeepingStaff: (storeId) => request('/housekeeping/staff' + (storeId ? `?store_id=${storeId}` : '')),

  // 发票管理
  listInvoiceTitles: (customerId) => request('/invoice-titles' + (customerId ? `?customer_id=${customerId}` : '')),
  createInvoiceTitle: (data) => request('/invoice-titles', { method: 'POST', body: JSON.stringify(data) }),
  updateInvoiceTitle: (id, data) => request(`/invoice-titles/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteInvoiceTitle: (id) => request(`/invoice-titles/${id}`, { method: 'DELETE' }),
  listInvoices: (params) => {
    const qs = new URLSearchParams(params).toString()
    return request('/invoices' + (qs ? '?' + qs : ''))
  },
  createInvoice: (data) => request('/invoices', { method: 'POST', body: JSON.stringify(data) }),
  getInvoice: (id) => request(`/invoices/${id}`),
  voidInvoice: (id) => request(`/invoices/${id}/void`, { method: 'POST' }),
  invoiceSummary: (params) => {
    const qs = new URLSearchParams(params).toString()
    return request('/invoices/summary' + (qs ? '?' + qs : ''))
  },
}
