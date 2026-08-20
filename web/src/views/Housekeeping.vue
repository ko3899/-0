<template>
  <div class="page housekeeping">
    <div class="page-header">
      <span class="page-title">客房清洁管理</span>
      <el-button type="primary" @click="openCreate">新建清洁任务</el-button>
    </div>

    <!-- 统计概览 -->
    <div class="stat-grid">
      <div class="stat-card" v-for="s in statusCards" :key="s.status" @click="filterStatus = s.status; load()">
        <div class="stat-num" :style="{ color: s.color }">{{ statCounts[s.status] || 0 }}</div>
        <div class="stat-label">{{ s.label }}</div>
      </div>
    </div>

    <!-- 工具栏 -->
    <el-card shadow="never" class="toolbar-card">
      <div class="toolbar">
        <el-select v-model="storeId" placeholder="全部门店" clearable style="width: 180px" @change="load">
          <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
        </el-select>
        <el-select v-model="filterStatus" placeholder="全部状态" clearable style="width: 130px" @change="load">
          <el-option v-for="s in statusCards" :key="s.status" :label="s.label" :value="s.status" />
        </el-select>
        <el-input v-model="filterRoomNo" placeholder="房号搜索" clearable style="width: 130px" @clear="load" @keyup.enter="load" />
        <el-button type="primary" @click="load">查询</el-button>
        <el-button @click="resetFilter">重置</el-button>
      </div>
    </el-card>

    <!-- 任务列表 -->
    <el-card shadow="never" v-loading="loading">
      <el-table :data="tasks" border stripe>
        <el-table-column prop="room_no" label="房号" width="80" />
        <el-table-column prop="store_name" label="门店" width="140" />
        <el-table-column prop="floor" label="楼层" width="70" />
        <el-table-column prop="room_type" label="房型" width="100" />
        <el-table-column label="类型" width="90">
          <template #default="{ row }">{{ taskTypeMap[row.task_type] }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)">{{ statusMap[row.status] }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="assigned_to" label="服务员" width="90" />
        <el-table-column label="创建时间" width="140">
          <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="120" show-overflow-tooltip />
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 0" link type="primary" @click="openAssign(row)">分配</el-button>
            <el-button v-if="row.status === 1" link type="success" @click="doStart(row)">开始清洁</el-button>
            <el-button v-if="row.status === 2" link type="warning" @click="doSubmit(row)">提交查房</el-button>
            <el-button v-if="row.status === 3" link type="success" @click="openInspect(row, true)">查房通过</el-button>
            <el-button v-if="row.status === 3" link type="danger" @click="openInspect(row, false)">不通过</el-button>
            <span v-if="row.status === 4" style="color: #67c23a; font-size: 12px">已完成</span>
            <span v-if="row.status === 5" style="color: #909399; font-size: 12px">需维修</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 分配弹窗 -->
    <el-dialog v-model="assignVisible" title="分配服务员" width="420px" destroy-on-close>
      <el-form label-width="80px">
        <el-form-item label="房间">{{ current?.room_no }}</el-form-item>
        <el-form-item label="服务员" required>
          <el-select v-model="assignStaffId" placeholder="选择服务员" filterable style="width: 100%">
            <el-option v-for="s in staffList" :key="s.id" :label="`${s.name}（${s.role}）`" :value="s.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="assignVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="doAssign">确认分配</el-button>
      </template>
    </el-dialog>

    <!-- 查房弹窗 -->
    <el-dialog v-model="inspectVisible" :title="inspectPass ? '查房通过' : '查房不通过'" width="420px" destroy-on-close>
      <el-alert
        :title="inspectPass ? '通过后房间将转为空净状态' : '不通过后房间将转为维修状态'"
        :type="inspectPass ? 'success' : 'warning'"
        :closable="false" show-icon
        style="margin-bottom: 16px"
      />
      <el-form label-width="80px">
        <el-form-item label="房间">{{ current?.room_no }}</el-form-item>
        <el-form-item label="备注">
          <el-input v-model="inspectRemark" type="textarea" :rows="3" placeholder="查房备注" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="inspectVisible = false">取消</el-button>
        <el-button :type="inspectPass ? 'success' : 'danger'" :loading="submitting" @click="doInspect">确认</el-button>
      </template>
    </el-dialog>

    <!-- 新建任务弹窗 -->
    <el-dialog v-model="createVisible" title="新建清洁任务" width="460px" destroy-on-close>
      <el-form :model="createForm" label-width="80px">
        <el-form-item label="门店" required>
          <el-select v-model="createForm.store_id" placeholder="选择门店" style="width: 100%" @change="onCreateStoreChange">
            <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="房间" required>
          <el-select v-model="createForm.room_id" placeholder="选择房间" filterable style="width: 100%">
            <el-option v-for="r in createRooms" :key="r.id" :label="`${r.room_no}（${r.room_type_name}，${r.floor}楼）`" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="createForm.task_type" style="width: 100%">
            <el-option label="退房清洁" :value="0" />
            <el-option label="日常清洁" :value="1" />
            <el-option label="深层清洁" :value="2" />
          </el-select>
        </el-form-item>
        <el-form-item label="优先级">
          <el-radio-group v-model="createForm.priority">
            <el-radio :value="0">普通</el-radio>
            <el-radio :value="1">紧急</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="createForm.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="doCreate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'

const stores = ref([])
const tasks = ref([])
const staffList = ref([])
const createRooms = ref([])
const loading = ref(false)
const submitting = ref(false)

const storeId = ref(Number(localStorage.getItem('current_store') || 0))
const filterStatus = ref(null)
const filterRoomNo = ref('')

const statCounts = ref({})

const statusCards = [
  { status: 0, label: '待分配', color: '#909399' },
  { status: 1, label: '已分配', color: '#409eff' },
  { status: 2, label: '清洁中', color: '#e6a23c' },
  { status: 3, label: '待查房', color: '#9c27b0' },
  { status: 4, label: '已完成', color: '#67c23a' },
  { status: 5, label: '需维修', color: '#f56c6c' },
]
const statusMap = { 0: '待分配', 1: '已分配', 2: '清洁中', 3: '待查房', 4: '已完成', 5: '需维修' }
const taskTypeMap = { 0: '退房清洁', 1: '日常清洁', 2: '深层清洁' }

function statusTagType(s) {
  return ['', 'info', 'warning', 'warning', 'success', 'danger'][s] || ''
}

function fmtTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-')
}

async function loadStores() {
  try {
    const r = await api.listStores()
    stores.value = r.stores || []
  } catch (e) { /* 静默 */ }
}

async function load() {
  loading.value = true
  try {
    const params = {}
    if (storeId.value) params.store_id = storeId.value
    if (filterStatus.value !== null && filterStatus.value !== '') params.status = filterStatus.value
    if (filterRoomNo.value) params.room_no = filterRoomNo.value
    const r = await api.listHousekeepingTasks(params)
    tasks.value = r.tasks || []
    // 统计各状态数量（用全量数据，不受状态过滤影响时更准确，这里简化用当前列表）
    loadStats()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    const params = {}
    if (storeId.value) params.store_id = storeId.value
    const r = await api.housekeepingStats(params)
    // 从任务列表统计更直观
    const counts = {}
    tasks.value.forEach(t => { counts[t.status] = (counts[t.status] || 0) + 1 })
    statCounts.value = counts
  } catch (e) { /* 静默 */ }
}

function resetFilter() {
  filterStatus.value = null
  filterRoomNo.value = ''
  load()
}

// 分配
const assignVisible = ref(false)
const assignStaffId = ref(null)
const current = ref(null)

async function openAssign(row) {
  current.value = row
  assignStaffId.value = null
  try {
    const r = await api.listHousekeepingStaff(row.store_id)
    staffList.value = r.staff || []
  } catch (e) {
    ElMessage.error(e.message)
    return
  }
  assignVisible.value = true
}

async function doAssign() {
  if (!assignStaffId.value) {
    ElMessage.warning('请选择服务员')
    return
  }
  submitting.value = true
  try {
    await api.assignHousekeepingTask(current.value.id, assignStaffId.value)
    ElMessage.success('分配成功')
    assignVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

// 开始清洁
async function doStart(row) {
  try {
    await api.startHousekeepingTask(row.id)
    ElMessage.success('已开始清洁')
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

// 提交查房
async function doSubmit(row) {
  try {
    await ElMessageBox.confirm(`确认提交房间 ${row.room_no} 的清洁完成，等待查房？`, '提交查房', { type: 'info' })
    await api.submitHousekeepingTask(row.id)
    ElMessage.success('已提交，等待查房')
    load()
  } catch (e) {
    if (e !== 'cancel' && e.message) ElMessage.error(e.message)
  }
}

// 查房
const inspectVisible = ref(false)
const inspectPass = ref(true)
const inspectRemark = ref('')

function openInspect(row, pass) {
  current.value = row
  inspectPass.value = pass
  inspectRemark.value = ''
  inspectVisible.value = true
}

async function doInspect() {
  submitting.value = true
  try {
    await api.inspectHousekeepingTask(current.value.id, { pass: inspectPass.value, remark: inspectRemark.value })
    ElMessage.success(inspectPass.value ? '查房通过，房间已转空净' : '已标记需维修')
    inspectVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

// 新建任务
const createVisible = ref(false)
const createForm = ref({ store_id: null, room_id: null, task_type: 0, priority: 0, remark: '' })

async function openCreate() {
  createForm.value = { store_id: storeId.value || null, room_id: null, task_type: 0, priority: 0, remark: '' }
  if (createForm.value.store_id) await onCreateStoreChange(createForm.value.store_id)
  createVisible.value = true
}

async function onCreateStoreChange(sid) {
  if (!sid) { createRooms.value = []; return }
  try {
    const r = await api.listRooms(sid)
    createRooms.value = r.rooms || []
  } catch (e) { /* 静默 */ }
}

async function doCreate() {
  if (!createForm.value.store_id || !createForm.value.room_id) {
    ElMessage.warning('请选择门店和房间')
    return
  }
  submitting.value = true
  try {
    await api.createHousekeepingTask(createForm.value)
    ElMessage.success('任务已创建')
    createVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadStores()
  load()
})
</script>

<style scoped>
.housekeeping { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; }
.page-title { font-size: 18px; font-weight: 600; color: var(--text-1); }

.stat-grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: 12px; }
.stat-card {
  background: #fff; border-radius: var(--radius-md); padding: 16px; text-align: center;
  box-shadow: var(--shadow-card); cursor: pointer; transition: transform .15s;
}
.stat-card:hover { transform: translateY(-2px); }
.stat-num { font-size: 24px; font-weight: 700; }
.stat-label { font-size: 12px; color: var(--text-2); margin-top: 4px; }

.toolbar { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }

@media (max-width: 1000px) { .stat-grid { grid-template-columns: repeat(3, 1fr); } }
</style>
