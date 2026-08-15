<template>
  <div class="reservation">
    <div class="toolbar">
      <el-select v-model="storeId" placeholder="全部门店" clearable style="width: 200px" @change="load">
        <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
      </el-select>
      <el-select v-model="status" placeholder="全部状态" clearable style="width: 150px" @change="load">
        <el-option v-for="(c, k) in statusMap" :key="k" :label="c.label" :value="Number(k)" />
      </el-select>
      <el-button type="primary" @click="openCreate">新建预订</el-button>
    </div>

    <el-table :data="list" border stripe>
      <el-table-column prop="id" label="预订号" width="80" />
      <el-table-column prop="store_name" label="门店" width="150" />
      <el-table-column prop="guest_name" label="客人" width="100" />
      <el-table-column prop="guest_phone" label="电话" width="125" />
      <el-table-column label="渠道" width="80">
        <template #default="{ row }">{{ channelMap[row.channel] || row.channel }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="statusMap[row.status].type">{{ statusMap[row.status].label }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="入住" width="105">
        <template #default="{ row }">{{ fmtDate(row.check_in_date) }}</template>
      </el-table-column>
      <el-table-column label="离店" width="105">
        <template #default="{ row }">{{ fmtDate(row.check_out_date) }}</template>
      </el-table-column>
      <el-table-column prop="room_type_name" label="房型" width="100" />
      <el-table-column prop="room_no" label="房间" width="75" />
      <el-table-column prop="deposit" label="定金" width="80" />
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.status === 0" link type="primary" @click="openCheckin(row)">办理入住</el-button>
          <span v-else style="color:#c0c4cc">-</span>
        </template>
      </el-table-column>
    </el-table>

    <!-- 新建预订 -->
    <el-dialog v-model="createVisible" title="新建预订" width="560px" destroy-on-close>
      <el-form :model="form" label-width="90px">
        <el-form-item label="门店" required>
          <el-select v-model="form.store_id" style="width: 100%" @change="loadRoomTypes">
            <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="客人姓名" required>
          <el-input v-model="form.customer_name" />
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="form.customer_phone" />
        </el-form-item>
        <el-form-item label="渠道">
          <el-select v-model="form.channel" style="width: 100%">
            <el-option label="到店" value="walk_in" />
            <el-option label="电话" value="phone" />
            <el-option label="线上" value="online" />
            <el-option label="OTA" value="ota" />
          </el-select>
        </el-form-item>
        <el-form-item label="入住日期" required>
          <el-date-picker v-model="form.check_in_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="离店日期" required>
          <el-date-picker v-model="form.check_out_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="房型">
          <el-select v-model="form.room_type_id" clearable placeholder="选填" style="width: 100%">
            <el-option v-for="t in roomTypes" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="定金">
          <el-input-number v-model="form.deposit" :min="0" :precision="2" :step="50" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitCreate">提交</el-button>
      </template>
    </el-dialog>

    <!-- 预订转入住 -->
    <el-dialog v-model="checkinVisible" title="办理入住 - 选择房间" width="480px" destroy-on-close>
      <p class="tip-line">预订号 #{{ currentRes?.id }} · 客人 {{ currentRes?.guest_name }} · 房型 {{ currentRes?.room_type_name || '未指定' }}</p>
      <el-select v-model="selectedRoomId" placeholder="选择空净房间" style="width: 100%">
        <el-option v-for="r in availableRooms" :key="r.id" :label="`${r.room_no}（${r.room_type_name}）`" :value="r.id" />
      </el-select>
      <template #footer>
        <el-button @click="checkinVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitCheckin">确认入住</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const statusMap = {
  0: { label: '预订', type: 'primary' },
  1: { label: '已入住', type: 'success' },
  2: { label: '已取消', type: 'info' },
  3: { label: '已退房', type: 'warning' },
  4: { label: 'No-show', type: 'danger' }
}
const channelMap = { walk_in: '到店', phone: '电话', online: '线上', ota: 'OTA' }

const stores = ref([])
const storeId = ref(null)
const status = ref(null)
const list = ref([])
const createVisible = ref(false)
const checkinVisible = ref(false)
const submitting = ref(false)
const roomTypes = ref([])
const currentRes = ref(null)
const availableRooms = ref([])
const selectedRoomId = ref(null)

const form = ref({
  store_id: null,
  customer_name: '',
  customer_phone: '',
  channel: 'walk_in',
  check_in_date: '',
  check_out_date: '',
  room_type_id: null,
  deposit: 0,
  remark: ''
})

function fmtDate(d) {
  if (!d) return ''
  return String(d).slice(0, 10)
}

async function loadStores() {
  try {
    const d = await api.listStores()
    stores.value = d.stores || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function load() {
  try {
    const d = await api.listReservations({ store_id: storeId.value || '', status: status.value ?? '' })
    list.value = d.reservations || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openCreate() {
  form.value = {
    store_id: storeId.value || (stores.value[0]?.id || null),
    customer_name: '',
    customer_phone: '',
    channel: 'walk_in',
    check_in_date: '',
    check_out_date: '',
    room_type_id: null,
    deposit: 0,
    remark: ''
  }
  roomTypes.value = []
  createVisible.value = true
  if (form.value.store_id) loadRoomTypes()
}

async function loadRoomTypes() {
  if (!form.value.store_id) return
  try {
    const d = await api.listRoomTypes(form.value.store_id)
    roomTypes.value = d.room_types || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function submitCreate() {
  const f = form.value
  if (!f.store_id || !f.customer_name.trim() || !f.check_in_date || !f.check_out_date) {
    ElMessage.warning('请填写门店、客人、入住/离店日期')
    return
  }
  submitting.value = true
  try {
    await api.createReservation({ ...f, customer_name: f.customer_name.trim() })
    ElMessage.success('预订创建成功')
    createVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function openCheckin(row) {
  currentRes.value = row
  selectedRoomId.value = null
  checkinVisible.value = true
  try {
    const d = await api.listRooms(row.store_id)
    availableRooms.value = (d.rooms || []).filter((r) => r.status === 0 || r.status === 1)
    if (!availableRooms.value.length) ElMessage.warning('该门店暂无空净房间')
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function submitCheckin() {
  if (!selectedRoomId.value) {
    ElMessage.warning('请选择房间')
    return
  }
  submitting.value = true
  try {
    await api.reservationCheckIn(currentRes.value.id, selectedRoomId.value)
    ElMessage.success('入住成功')
    checkinVisible.value = false
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
.toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}
.tip-line {
  margin-bottom: 14px;
  color: #606266;
}
</style>
