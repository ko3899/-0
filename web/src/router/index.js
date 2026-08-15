import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/', redirect: '/rooms' },
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue'), meta: { public: true } },
  { path: '/rooms', name: 'RoomStatus', component: () => import('../views/RoomStatus.vue'), meta: { title: '房态图' } },
  { path: '/checkins', name: 'Checkin', component: () => import('../views/Checkin.vue'), meta: { title: '在住管理' } },
  { path: '/reservations', name: 'Reservation', component: () => import('../views/Reservation.vue'), meta: { title: '预订管理' } },
  { path: '/customers', name: 'Customer', component: () => import('../views/Customer.vue'), meta: { title: '客户档案' } }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 登录守卫：未登录访问受保护页面重定向到登录页
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (!to.meta.public && !token) {
    next('/login')
  } else if (to.path === '/login' && token) {
    next('/rooms')
  } else {
    next()
  }
})

export default router
