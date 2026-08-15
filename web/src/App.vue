<template>
  <router-view v-if="$route.path === '/login'" />
  <el-container v-else class="app-container">
    <el-aside width="220px" class="app-aside">
      <div class="logo">酒店管理系统</div>
      <el-menu
        :default-active="$route.path"
        router
        class="app-menu"
        background-color="#001529"
        text-color="#a6adb4"
        active-text-color="#ffffff"
      >
        <el-menu-item index="/rooms"><span>房态图</span></el-menu-item>
        <el-menu-item index="/checkins"><span>在住管理</span></el-menu-item>
        <el-menu-item index="/reservations"><span>预订管理</span></el-menu-item>
        <el-menu-item index="/customers"><span>客户档案</span></el-menu-item>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="app-header">
        <span>连锁多门店 · 总部统一管理</span>
        <div class="header-right">
          <span class="user-name">{{ userName }}</span>
          <el-button link type="primary" @click="logout">退出登录</el-button>
        </div>
      </el-header>
      <el-main class="app-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

const userName = computed(() => {
  try {
    const u = JSON.parse(localStorage.getItem('user') || '{}')
    return u.name || u.username || '未登录'
  } catch {
    return '未登录'
  }
})

function logout() {
  localStorage.removeItem('token')
  localStorage.removeItem('user')
  router.push('/login')
}
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}
html,
body,
#app {
  height: 100%;
}
.app-container {
  height: 100%;
}
.app-aside {
  background: #001529;
}
.logo {
  height: 60px;
  line-height: 60px;
  text-align: center;
  font-size: 18px;
  font-weight: bold;
  color: #fff;
}
.app-menu {
  border-right: none;
}
.app-header {
  background: #fff;
  border-bottom: 1px solid #e5e5e5;
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #666;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.user-name {
  font-size: 14px;
  color: #303133;
}
.app-main {
  background: #f5f7fa;
}
</style>
