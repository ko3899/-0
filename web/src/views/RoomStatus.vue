<template>
  <div class="room-status">
    <div class="toolbar">
      <el-select v-model="storeId" placeholder="选择门店" style="width: 220px" @change="loadRooms">
        <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
      </el-select>
      <div class="legend">
        <span v-for="(c, k) in statusMap" :key="k" class="legend-item">
          <i :style="{ background: c.color }"></i>{{ c.label }}
        </span>
      </div>
      <el-button type="primary" @click="loadRooms">刷新</el-button>
    </div>

    <div v-if="!rooms.length" class="empty-tip">暂无房间数据，请选择门店后刷新</div>

    <div v-for="(list, floor) in floorGroups" :key="floor" class="floor-block">
      <div class="floor-title">{{ floor }} 楼</div>
      <div class="room-grid">
        <div
          v-for="room in list"
          :key="room.id"
          class="room-cell"
          :style="{ background: statusMap[room.status].color }"
          @click="openAction(room)"
        >
          <div class="room-no">{{ room.room_no }}</div>
          <div class="room-type">{{ room.room_type_name }}</div>
          <div class="room-state">{{ statusMap[room.status].label }}</div>
        </div>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="560px" destroy-on-close>
      <div v-if="currentRoom" class="room-info">
        <el-tag>{{ currentRoom.room_no }}</el-tag>
        <span class="info-text">{{ currentRoom.room_type_name }} · {{ currentRoom.bed_type }}</span>
        <el-tag :type="statusTagType(currentRoom.status)" effect="dark">{{ statusMap[currentRoom.status].label }}</el-tag>
      </div>

      <!-- 操作选择（非住客房间先选操作） -->
      <div v-if="action === ''" class="action-btns">
        <el-button
          v-for="a in availableActions"
          :key="a.key"
          :type="a.type"
          @click="chooseAction(a.key)"
        >{{ a.label }}</el-button>
      </div>

      <!-- 入住表单 -->
      <el-form v-if="action === 'checkin'" :model="checkinForm" label-width="90px">
        <el-form-item label="客人姓名" required>
          <el-input v-model="checkinForm.customer_name" placeholder="必填" />
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="checkinForm.customer_phone" placeholder="选填，按手机号识别老客户" />
        </el-form-item>
        <el-form-item label="证件号">
          <el-input v-model="checkinForm.id_no" placeholder="选填" />
        </el-form-item>
        <el-form-item label="入住晚数">
          <el-input-number v-model="checkinForm.nights" :min="1" :max="30" />
        </el-form-item>
        <el-form-item label="房价/晚">
          <el-input-number v-model="checkinForm.price" :min="0" :precision="2" :step="10" />
          <span class="tip">留 0 按门市价</span>
        </el-form-item>
        <el-form-item label="押金">
          <el-input-number v-model="checkinForm.deposit" :min="0" :precision="2" :step="50" />
        </el-form-item>
      </el-form>

      <!-- 退房结算 -->
      <div v-else-if="action === 'checkout'" class="checkout-box">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="账单总额">¥ {{ currentRoom.total_amount }}</el-descriptions-item>
          <el-descriptions-item label="待付余额">¥ {{ currentRoom.balance }}</el-descriptions-item>
        </el-descriptions>
        <el-form label-width="90px" style="margin-top: 16px">
          <el-form-item label="收款金额">
            <el-input-number v-model="checkoutAmount" :min="0" :precision="2" :step="50" />
          </el-form-item>
          <el-form-item label="支付方式">
            <el-select v-model="checkoutMethod" style="width: 200px">
              <el-option label="现金" value="cash" />
              <el-option label="银行卡" value="bank_card" />
              <el-option label="微信" value="wechat" />
              <el-option label="支付宝" value="alipay" />
            </el-select>
          </el-form-item>
        </el-form>
      </div>

      <!-- 状态变更确认 -->
      <div v-else-if="action === 'change'" class="confirm-box">
        <el-alert :title="changeText" type="warning" :closable="false" show-icon />
      </div>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button v-if="action !== ''" type="primary" :loading="submitting" @click="submit">{{ submitText }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const statusMap = {
  0: { label: '空净', color: '#67C23A' },
  1: { label: '空脏', color: '#E6A23C' },
  2: { label: '住客', color: '#409EFF' },
  3: { label: '维修', color: '#909399' },
  4: { label: '预留', color: '#9254DE' }
}

const stores = ref([])
const storeId = ref(null)
const rooms = ref([])
const dialogVisible = ref(false)
const action = ref('')
const currentRoom = ref(null)
const submitting = ref(false)

const checkinForm = ref({ customer_name: '', customer_phone: '', id_no: '', nights: 1, price: 0, deposit: 0 })
const checkoutAmount = ref(0)
const checkoutMethod = ref('cash')
const changeStatus = ref(0)

const floorGroups = computed(() => {
  const g = {}
  for (const r of rooms.value) {
    const f = r.floor || '其他'
    if (!g[f]) g[f] = []
    g[f].push(r)
  }
  const keys = Object.keys(g).sort((a, b) => {
    const na = parseInt(a, 10)
    const nb = parseInt(b, 10)
    if (!isNaN(na) && !isNaN(nb)) return na - nb
    return a.localeCompare(b)
  })
  const sorted = {}
  for (const k of keys) sorted[k] = g[k]
  return sorted
})

const dialogTitle = computed(() => {
  if (!currentRoom.value) return ''
  const labels = { checkin: '办理入住', checkout: '退房结账', change: '状态变更' }
  return `${labels[action.value] || '选择操作'} - 房间 ${currentRoom.value.room_no}`
})

const submitText = computed(() => ({ checkin: '确认入住', checkout: '确认退房', change: '确认' })[action.value] || '确认')

const changeText = computed(() => {
  const labels = { 0: '将该房间标记为「空净」（清洁完成）', 1: '将该房间标记为「空脏」', 3: '将该房间标记为「维修」', 4: '将该房间标记为「预留」' }
  return labels[changeStatus.value] || ''
})

// 根据房间状态返回可选操作
const availableActions = computed(() => {
  const s = currentRoom.value ? currentRoom.value.status : 0
  const map = {
    0: [
      { key: 'checkin', label: '办理入住', type: 'primary' },
      { key: 'maintain', label: '设为维修', type: 'warning' },
      { key: 'reserve', label: '设为预留', type: 'info' }
    ],
    1: [
      { key: 'checkin', label: '办理入住', type: 'primary' },
      { key: 'clean', label: '清洁完成', type: 'success' },
      { key: 'maintain', label: '设为维修', type: 'warning' }
    ],
    3: [{ key: 'restore', label: '恢复空净', type: 'success' }],
    4: [
      { key: 'checkin', label: '办理入住', type: 'primary' },
      { key: 'unreserve', label: '取消预留', type: 'info' }
    ]
  }
  return map[s] || []
})

function statusTagType(status) {
  return { 0: 'success', 1: 'warning', 2: 'primary', 3: 'info', 4: '' }[status] || 'info'
}

function chooseAction(key) {
  if (key === 'checkin') {
    action.value = 'checkin'
  } else {
    action.value = 'change'
    changeStatus.value = { clean: 0, maintain: 3, reserve: 4, restore: 0, unreserve: 0 }[key] || 0
  }
}

async function loadStores() {
  try {
    const d = await api.listStores()
    stores.value = d.stores || []
    if (stores.value.length && !storeId.value) {
      storeId.value = stores.value[0].id
      loadRooms()
    }
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function loadRooms() {
  if (!storeId.value) return
  try {
    const d = await api.listRooms(storeId.value)
    rooms.value = d.rooms || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openAction(room) {
  currentRoom.value = room
  submitting.value = false
  checkinForm.value = { customer_name: '', customer_phone: '', id_no: '', nights: 1, price: 0, deposit: 0 }
  checkoutMethod.value = 'cash'

  if (room.status === 2) {
    action.value = 'checkout'
    checkoutAmount.value = room.balance || 0
  } else {
    action.value = ''
  }
  dialogVisible.value = true
}

async function submit() {
  if (submitting.value) return
  submitting.value = true
  try {
    if (action.value === 'checkin') {
      await doCheckin()
    } else if (action.value === 'checkout') {
      await doCheckout()
    } else if (action.value === 'change') {
      await doChangeStatus()
    }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function doCheckin() {
  const f = checkinForm.value
  if (!f.customer_name.trim()) {
    ElMessage.warning('请输入客人姓名')
    return
  }
  await api.createCheckIn({
    room_id: currentRoom.value.id,
    customer_name: f.customer_name.trim(),
    customer_phone: f.customer_phone.trim(),
    id_no: f.id_no.trim(),
    nights: f.nights,
    price: f.price,
    deposit: f.deposit
  })
  ElMessage.success('入住成功')
  dialogVisible.value = false
  loadRooms()
}

async function doCheckout() {
  await api.checkOut(currentRoom.value.check_in_id, checkoutMethod.value, checkoutAmount.value)
  ElMessage.success('退房完成')
  dialogVisible.value = false
  loadRooms()
}

async function doChangeStatus() {
  await api.updateRoomStatus(currentRoom.value.id, changeStatus.value)
  ElMessage.success('状态已更新')
  dialogVisible.value = false
  loadRooms()
}

onMounted(loadStores)
</script>

<style scoped>
.room-status {
  padding: 4px;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
}
.legend {
  display: flex;
  gap: 14px;
  flex: 1;
}
.legend-item {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 13px;
  color: #606266;
}
.legend-item i {
  width: 14px;
  height: 14px;
  border-radius: 3px;
  display: inline-block;
}
.empty-tip {
  text-align: center;
  color: #909399;
  padding: 60px 0;
}
.floor-block {
  margin-bottom: 18px;
}
.floor-title {
  font-size: 14px;
  font-weight: bold;
  color: #303133;
  margin-bottom: 8px;
  padding-left: 4px;
  border-left: 3px solid #409eff;
}
.room-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.room-cell {
  width: 118px;
  height: 78px;
  border-radius: 6px;
  color: #fff;
  padding: 8px;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
  transition: transform 0.1s;
}
.room-cell:hover {
  transform: translateY(-2px);
  box-shadow: 0 3px 8px rgba(0, 0, 0, 0.25);
}
.room-no {
  font-size: 18px;
  font-weight: bold;
}
.room-type {
  font-size: 11px;
  opacity: 0.9;
}
.room-state {
  font-size: 12px;
  font-weight: bold;
}
.room-info {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid #f0f0f0;
}
.info-text {
  color: #606266;
  font-size: 14px;
}
.action-btns {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  padding: 10px 0;
}
.tip {
  margin-left: 10px;
  font-size: 12px;
  color: #909399;
}
.confirm-box {
  padding: 10px 0;
}
</style>
