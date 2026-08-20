import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue'), meta: { public: true } },
  { path: '/dashboard', name: 'Dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: '首页仪表盘' } },
  { path: '/rooms', name: 'RoomStatus', component: () => import('../views/RoomStatus.vue'), meta: { title: '房态图' } },
  { path: '/checkins', name: 'Checkin', component: () => import('../views/Checkin.vue'), meta: { title: '在住管理' } },
  { path: '/reservations', name: 'Reservation', component: () => import('../views/Reservation.vue'), meta: { title: '预订管理' } },
  { path: '/housekeeping', name: 'Housekeeping', component: () => import('../views/Housekeeping.vue'), meta: { title: '客房清洁' } },
  { path: '/night-audit', name: 'NightAudit', component: () => import('../views/NightAudit.vue'), meta: { title: '夜审管理' } },
  { path: '/customers', name: 'Customer', component: () => import('../views/Customer.vue'), meta: { title: '客户档案' } },
  { path: '/members', name: 'Member', component: () => import('../views/Member.vue'), meta: { title: '会员管理' } },
  { path: '/invoice', name: 'Invoice', component: () => import('../views/Invoice.vue'), meta: { title: '发票管理' } },
  { path: '/rates', name: 'Rate', component: () => import('../views/Rate.vue'), meta: { title: '房价管理' } },
  { path: '/users', name: 'Users', component: () => import('../views/Users.vue'), meta: { title: '用户管理' } },
  { path: '/operation-logs', name: 'OperationLog', component: () => import('../views/OperationLog.vue'), meta: { title: '操作日志' } },
  { path: '/ota', name: 'OtaConfig', component: () => import('../views/OtaConfig.vue'), meta: { title: 'OTA 渠道对接' } }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

// 登录守卫：未登录访问受保护页面重定向到登录页
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (!to.meta.public && !token) {
    next('/login')
  } else if (to.path === '/login' && token) {
    next('/dashboard')
  } else {
    next()
  }
})

export default router
