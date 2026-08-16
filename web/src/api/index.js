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
  createCheckIn: (data) => request('/checkins', { method: 'POST', body: JSON.stringify(data) }),
  listCheckIns: (storeId) => request('/checkins' + (storeId ? `?store_id=${storeId}` : '')),
  checkOut: (id, method, amount) => request(`/checkins/${id}/checkout`, { method: 'POST', body: JSON.stringify({ method, amount }) }),
  getFolio: (id) => request(`/folios/${id}`),
  createReservation: (data) => request('/reservations', { method: 'POST', body: JSON.stringify(data) }),
  listReservations: (params = {}) => {
    const q = new URLSearchParams()
    if (params.store_id) q.set('store_id', params.store_id)
    if (params.status !== undefined && params.status !== null && params.status !== '') q.set('status', params.status)
    const s = q.toString()
    return request('/reservations' + (s ? `?${s}` : ''))
  },
  reservationCheckIn: (id, roomId) => request(`/reservations/${id}/checkin`, { method: 'POST', body: JSON.stringify({ room_id: roomId }) }),
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
}
