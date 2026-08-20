<template>
  <div class="operation-logs">
    <div class="toolbar">
      <div class="toolbar-title">操作日志</div>
      <div class="filters">
        <el-select v-model="filters.store_id" clearable placeholder="门店" style="width:180px" @change="search">
          <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
        </el-select>
        <el-select v-model="filters.action" clearable placeholder="操作类型" style="width:150px" @change="search">
          <el-option label="登录" value="login" />
          <el-option label="登出" value="logout" />
          <el-option label="入住" value="checkin" />
          <el-option label="退房" value="checkout" />
          <el-option label="换房" value="change_room" />
          <el-option label="续住" value="extend" />
          <el-option label="附加消费" value="charge" />
          <el-option label="收款" value="payment" />
          <el-option label="创建预订" value="reservation_create" />
          <el-option label="修改预订" value="reservation_update" />
          <el-option label="取消预订" value="reservation_cancel" />
          <el-option label="No-show" value="reservation_noshow" />
          <el-option label="预订转入住" value="reservation_checkin" />
          <el-option label="房态变更" value="room_status" />
          <el-option label="创建用户" value="user_create" />
          <el-option label="编辑用户" value="user_edit" />
        </el-select>
        <el-input v-model="filters.keyword" clearable placeholder="搜索关键词" style="width:200px" @clear="search" @keyup.enter="search" />
        <el-date-picker v-model="dateRange" type="daterange" range-separator="至"
          start-placeholder="开始日期" end-placeholder="结束日期"
          value-format="YYYY-MM-DD" style="width:280px" @change="search" />
        <el-button type="primary" @click="search">筛选</el-button>
        <el-button @click="clearFilters">清除</el-button>
      </div>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table :data="list" border stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="created_at" label="时间" width="170" />
      <el-table-column prop="username" label="操作人" width="110" />
      <el-table-column label="操作类型" width="130">
        <template #default="{ row }">
          <el-tag :type="actionTag(row.action)" size="small">{{ actionLabel(row.action) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="target" label="目标" width="120" />
      <el-table-column prop="detail" label="详情" min-width="200" />
      <el-table-column prop="store_name" label="门店" width="140">
        <template #default="{ row }">
          {{ row.store_name || '系统' }}
        </template>
      </el-table-column>
      <el-table-column prop="ip" label="IP" width="140" />
    </el-table>

    <div class="pagination">
      <el-pagination background layout="total, prev, pager, next, sizes" v-model:current-page="page"
        v-model:page-size="pageSize" :total="total" :page-sizes="[20, 50, 100, 200]"
        @current-change="load" @size-change="load" />
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { api } from '../api'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const loading = ref(false)
const stores = ref([])
const dateRange = ref(null)

const filters = reactive({
  store_id: '',
  action: '',
  keyword: '',
  start_date: '',
  end_date: ''
})

const actionMap = {
  login: '登录', logout: '登出', checkin: '入住', checkout: '退房',
  change_room: '换房', extend: '续住', charge: '附加消费', payment: '收款',
  reservation_create: '创建预订', reservation_update: '修改预订',
  reservation_cancel: '取消预订', reservation_noshow: 'No-show',
  reservation_checkin: '预订转入住', room_status: '房态变更',
  user_create: '创建用户', user_edit: '编辑用户'
}

const actionColors = {
  login: 'success', logout: 'info', checkin: 'primary', checkout: 'warning',
  change_room: 'primary', extend: 'primary', charge: 'danger', payment: 'success',
  reservation_create: 'primary', reservation_update: 'warning',
  reservation_cancel: 'danger', reservation_noshow: 'danger',
  reservation_checkin: 'success', room_status: 'info',
  user_create: 'primary', user_edit: 'warning'
}

function actionLabel(action) { return actionMap[action] || action }
function actionTag(action) { return actionColors[action] || 'info' }

async function load() {
  loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize.value }
    if (filters.store_id) params.store_id = filters.store_id
    if (filters.action) params.action = filters.action
    if (filters.keyword) params.keyword = filters.keyword
    if (filters.start_date) params.start_date = filters.start_date
    if (filters.end_date) params.end_date = filters.end_date

    const res = await api.listOperationLogs(params)
    list.value = res.logs || []
    total.value = res.total || 0
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

function search() {
  if (dateRange.value && dateRange.value.length === 2) {
    filters.start_date = dateRange.value[0]
    filters.end_date = dateRange.value[1]
  } else {
    filters.start_date = ''
    filters.end_date = ''
  }
  page.value = 1
  load()
}

function clearFilters() {
  filters.store_id = ''
  filters.action = ''
  filters.keyword = ''
  filters.start_date = ''
  filters.end_date = ''
  dateRange.value = null
  page.value = 1
  load()
}

async function loadStores() {
  try {
    const res = await api.listStores()
    stores.value = res.stores || []
  } catch (e) { /* ignore */ }
}

onMounted(() => {
  loadStores()
  load()
})
</script>

<style scoped>
.operation-logs {
  padding: 20px;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.toolbar-title {
  font-size: 18px;
  font-weight: 600;
  margin-right: auto;
}
.filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
</style>