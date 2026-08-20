<template>
  <router-view v-if="$route.path === '/login'" />
  <el-container v-else class="app-container">
    <!-- 侧边栏 -->
    <el-aside width="232px" class="app-aside">
      <div class="brand">
        <div class="brand-icon">
          <svg viewBox="0 0 24 24" width="26" height="26" fill="none">
            <rect x="4" y="9" width="16" height="11" rx="1.5" fill="#c9a45c" />
            <path d="M6 9V7a6 6 0 0 1 12 0v2" stroke="#c9a45c" stroke-width="1.8" fill="none" />
            <rect x="9" y="12.5" width="6" height="3" rx="1" fill="#0e1f33" />
          </svg>
        </div>
        <div class="brand-text">
          <div class="brand-name">云端酒店</div>
          <div class="brand-sub">连锁管理平台</div>
        </div>
      </div>

      <el-menu
        :default-active="$route.path"
        router
        class="app-menu"
        background-color="transparent"
        text-color="#9fb0c3"
        active-text-color="#ffffff"
      >
        <el-menu-item-group title="经营分析">
          <el-menu-item index="/dashboard">
            <el-icon><Odometer /></el-icon><span>首页仪表盘</span>
          </el-menu-item>
        </el-menu-item-group>

        <el-menu-item-group title="前台业务">
          <el-menu-item index="/rooms">
            <el-icon><Grid /></el-icon><span>房态图</span>
          </el-menu-item>
          <el-menu-item index="/checkins">
            <el-icon><UserFilled /></el-icon><span>在住管理</span>
          </el-menu-item>
          <el-menu-item index="/reservations">
            <el-icon><Calendar /></el-icon><span>预订管理</span>
          </el-menu-item>
          <el-menu-item index="/housekeeping">
            <el-icon><Brush /></el-icon><span>客房清洁</span>
          </el-menu-item>
          <el-menu-item index="/night-audit">
            <el-icon><Moon /></el-icon><span>夜审管理</span>
          </el-menu-item>
        </el-menu-item-group>

        <el-menu-item-group title="客户管理">
          <el-menu-item index="/customers">
            <el-icon><Avatar /></el-icon><span>客户档案</span>
          </el-menu-item>
          <el-menu-item index="/members">
            <el-icon><Medal /></el-icon><span>会员管理</span>
          </el-menu-item>
        </el-menu-item-group>

        <el-menu-item-group title="财务管理">
          <el-menu-item index="/invoice">
            <el-icon><Tickets /></el-icon><span>发票管理</span>
          </el-menu-item>
        </el-menu-item-group>

        <el-menu-item-group title="系统设置">
          <el-menu-item index="/rates">
            <el-icon><Coin /></el-icon><span>房价管理</span>
          </el-menu-item>
          <el-menu-item v-if="isAdmin" index="/users">
            <el-icon><Setting /></el-icon><span>用户管理</span>
          </el-menu-item>
          <el-menu-item v-if="isAdmin" index="/ota">
            <el-icon><Connection /></el-icon><span>OTA 渠道对接</span>
          </el-menu-item>
          <el-menu-item v-if="isAdmin" index="/operation-logs">
            <el-icon><Document /></el-icon><span>操作日志</span>
          </el-menu-item>
        </el-menu-item-group>
      </el-menu>
    </el-aside>

    <el-container class="main-wrap">
      <!-- 顶部栏 -->
      <el-header class="app-header">
        <div class="header-left">
          <span class="page-crumb">{{ currentTitle }}</span>
        </div>
        <div class="header-right">
          <el-select
            v-model="currentStoreId"
            class="store-switch"
            placeholder="当前门店"
            size="default"
            @change="onStoreChange"
          >
            <el-option v-if="isAdmin" label="全部门店" :value="0" />
            <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>

          <el-dropdown trigger="click" @command="onUserCommand">
            <div class="user-chip">
              <div class="avatar">{{ avatarChar }}</div>
              <span class="user-name">{{ userName }}</span>
              <el-icon class="caret"><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item disabled>
                  <span class="role-tag">{{ roleName }}</span>
                </el-dropdown-item>
                <el-dropdown-item divided command="logout">
                  <el-icon><SwitchButton /></el-icon>退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="app-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  Odometer, Grid, UserFilled, Calendar, Avatar, Medal, Coin, ArrowDown, SwitchButton, Setting, Document, Connection, Brush, Moon, Tickets
} from '@element-plus/icons-vue'
import { api } from './api'

const router = useRouter()

const stores = ref([])
const currentStoreId = ref(Number(localStorage.getItem('current_store') || 0))

const user = computed(() => {
  try {
    return JSON.parse(localStorage.getItem('user') || '{}')
  } catch {
    return {}
  }
})
const userName = computed(() => user.value.name || user.value.username || '未登录')
const roleName = computed(() => user.value.role_name || user.value.role || '员工')
const isAdmin = computed(() => Boolean(user.value.is_admin || user.value.role_level >= 9))
const avatarChar = computed(() => userName.value.slice(0, 1).toUpperCase())

const menuTitles = {
  '/dashboard': '首页仪表盘',
  '/rooms': '房态图',
  '/checkins': '在住管理',
  '/reservations': '预订管理',
  '/housekeeping': '客房清洁',
  '/night-audit': '夜审管理',
  '/customers': '客户档案',
  '/members': '会员管理',
  '/invoice': '发票管理',
  '/rates': '房价管理',
  '/users': '用户管理',
  '/ota': 'OTA 渠道对接',
  '/operation-logs': '操作日志'
}
const currentTitle = computed(() => menuTitles[router.currentRoute.value.path] || '酒店管理系统')

async function loadStores() {
  try {
    const r = await api.listStores()
    stores.value = r.stores || []
    // 非管理员且未选择门店时，默认选中第一个有权限的门店
    if (!isAdmin.value && stores.value.length > 0 && currentStoreId.value === 0) {
      currentStoreId.value = stores.value[0].id
      localStorage.setItem('current_store', String(currentStoreId.value))
    }
  } catch (e) {
    /* 静默失败，不影响主界面 */
  }
}

function onStoreChange(id) {
  localStorage.setItem('current_store', String(id))
  ElMessage.success('已切换门店视角')
}

function onUserCommand(cmd) {
  if (cmd === 'logout') {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    localStorage.removeItem('current_store')
    router.push('/login')
  }
}

onMounted(loadStores)
</script>

<style scoped>
.app-container {
  height: 100%;
}

/* 侧边栏 */
.app-aside {
  background: linear-gradient(180deg, #0e1f33 0%, #13293f 100%);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.brand {
  height: 64px;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}
.brand-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: rgba(201, 164, 92, 0.14);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.brand-name {
  font-size: 17px;
  font-weight: 700;
  color: #fff;
  letter-spacing: 1px;
}
.brand-sub {
  font-size: 11px;
  color: #7d8fa3;
  margin-top: 2px;
}

.app-menu {
  flex: 1;
  border-right: none;
  padding: 8px 10px;
  overflow-y: auto;
}
.app-menu :deep(.el-menu-item-group__title) {
  padding: 14px 12px 6px;
  font-size: 11px;
  color: #5a6d82;
  letter-spacing: 1px;
}
.app-menu :deep(.el-menu-item) {
  height: 42px;
  line-height: 42px;
  border-radius: 8px;
  margin: 2px 0;
}
.app-menu :deep(.el-menu-item:hover) {
  background: rgba(255, 255, 255, 0.06);
}
.app-menu :deep(.el-menu-item.is-active) {
  background: linear-gradient(90deg, #2b5a9c, #3d7bd4);
  box-shadow: 0 4px 12px rgba(43, 90, 156, 0.4);
}
.app-menu :deep(.el-menu-item .el-icon) {
  margin-right: 8px;
}

/* 主区域 */
.main-wrap {
  min-width: 0;
}
.app-header {
  height: 64px;
  background: #fff;
  border-bottom: 1px solid #eef1f5;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  box-shadow: 0 1px 4px rgba(20, 40, 70, 0.04);
  z-index: 10;
}
.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}
.page-crumb {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-1);
}
.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}
.store-switch {
  width: 180px;
}

.user-chip {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 8px;
  transition: background 0.15s;
}
.user-chip:hover {
  background: #f5f7fa;
}
.avatar {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: linear-gradient(135deg, #2b5a9c, #3d7bd4);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  font-size: 15px;
}
.user-name {
  font-size: 14px;
  color: var(--text-1);
  font-weight: 500;
}
.caret {
  color: #8a97a8;
  font-size: 12px;
}
.role-tag {
  color: #8a97a8;
  font-size: 12px;
}

.app-main {
  background: var(--bg-page);
  padding: 20px 24px;
  overflow-y: auto;
}
</style>
