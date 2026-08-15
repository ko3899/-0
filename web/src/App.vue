<template>
  <el-container class="app-container">
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
        <el-menu-item index="/home">
          <span>首页</span>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="app-header">
        <span>连锁多门店 · 总部统一管理</span>
      </el-header>
      <el-main class="app-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
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
  color: #666;
}
.app-main {
  background: #f5f7fa;
}
</style>
