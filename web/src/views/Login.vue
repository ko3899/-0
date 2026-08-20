<template>
  <div class="login-wrap">
    <el-card class="login-card" shadow="always">
      <div class="login-logo">酒店管理系统</div>
      <p class="login-sub">连锁多门店 · 总部统一管理</p>
      <el-form @submit.prevent="onLogin">
        <el-form-item>
          <el-input v-model="username" placeholder="用户名" size="large" clearable />
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="password"
            type="password"
            placeholder="密码"
            size="large"
            show-password
            @keyup.enter="onLogin"
          />
        </el-form-item>
        <el-button type="primary" size="large" class="login-btn" :loading="loading" @click="onLogin">
          登 录
        </el-button>
        <el-button size="large" class="demo-btn" @click="enterDemo">
          进入演示模式
        </el-button>
      </el-form>
      <p class="login-tip">演示账号：admin / admin123（需连接后端）</p>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

const router = useRouter()
const username = ref('admin')
const password = ref('admin123')
const loading = ref(false)

async function onLogin() {
  if (!username.value || !password.value) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    const res = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username.value, password: password.value })
    })
    const data = await res.json()
    if (!res.ok) {
      ElMessage.error(data.error || '登录失败')
      return
    }
    localStorage.setItem('token', data.token)
    localStorage.setItem('user', JSON.stringify(data.user))
    ElMessage.success('登录成功')
    router.push('/rooms')
  } catch (e) {
    ElMessage.error('网络错误，请检查后端服务是否启动')
  } finally {
    loading.value = false
  }
}

function enterDemo() {
  localStorage.setItem('token', 'demo-token')
  localStorage.setItem('user', JSON.stringify({ id: 0, username: 'demo', name: '演示模式', role: '集团管理员', role_level: 9, is_admin: true, store_ids: [] }))
  ElMessage.success('已进入演示模式（界面可浏览，数据需连接后端）')
  router.push('/dashboard')
}
</script>

<style scoped>
.login-wrap {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1f2d3d 0%, #324157 100%);
}
.login-card {
  width: 380px;
  padding: 12px 8px;
  border-radius: 8px;
}
.login-logo {
  font-size: 24px;
  font-weight: 700;
  text-align: center;
  color: #303133;
}
.login-sub {
  text-align: center;
  color: #909399;
  font-size: 13px;
  margin: 8px 0 24px;
}
.login-btn {
  width: 100%;
}
.demo-btn {
  width: 100%;
  margin-top: 10px;
}
.login-tip {
  text-align: center;
  color: #c0c4cc;
  font-size: 12px;
  margin-top: 16px;
}
</style>
