<template>
  <div class="checkin">
<div class="toolbar">
      <el-select v-model="storeId" placeholder="全部门店" clearable style="width: 160px" @change="load">
        <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
      </el-select>
      <el-input v-model="filterFloor" placeholder="楼层" clearable style="width: 100px" @clear="load" @keyup.enter="load" />
      <el-input v-model="filterRoomNo" placeholder="房间号" clearable style="width: 120px" @clear="load" @keyup.enter="load" />
      <el-input v-model="filterGuestName" placeholder="客人姓名" clearable style="width: 130px" @clear="load" @keyup.enter="load" />
      <el-input v-model="filterGuestPhone" placeholder="电话" clearable style="width: 130px" @clear="load" @keyup.enter="load" />
      <el-input-number v-model="filterDepositMin" placeholder="押金≥" :min="0" :precision="0" :step="50" style="width: 120px" />
      <el-input-number v-model="filterDepositMax" placeholder="押金≤" :min="0" :precision="0" :step="50" style="width: 120px" />
      <el-button type="primary" @click="load">筛选</el-button>
      <el-button @click="clearFilter">清除</el-button>
    </div>
    <el-table :data="list" border stripe>
      <el-table-column prop="room_no" label="房间" width="80" />
      <el-table-column prop="guest_name" label="客人" width="100" />
      <el-table-column prop="guest_phone" label="电话" width="130" />
      <el-table-column prop="store_name" label="门店" width="150" />
      <el-table-column label="入住时间" width="150">
        <template #default="{ row }">{{ fmtTime(row.check_in_at) }}</template>
      </el-table-column>
      <el-table-column label="预计退房" width="150">
        <template #default="{ row }">{{ fmtTime(row.check_out_at) }}</template>
      </el-table-column>
      <el-table-column prop="deposit" label="押金" width="80" />
      <el-table-column prop="total" label="总额" width="90" />
      <el-table-column prop="paid" label="已付" width="80" />
      <el-table-column label="待付" width="90">
        <template #default="{ row }">
          <span style="color: #f56c6c; font-weight: bold">¥ {{ row.balance }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="380" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openCheckout(row)">退房</el-button>
          <el-button link type="success" @click="openChangeRoom(row)">换房</el-button>
          <el-button link type="warning" @click="openExtend(row)">续住</el-button>
          <el-button link @click="openCharge(row)">消费</el-button>
          <el-button link @click="openPayment(row)">收款</el-button>
          <el-button link @click="openFolio(row)">账单</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 退房结算 -->
    <el-dialog v-model="checkoutVisible" :title="`退房结算 - ${current?.room_no}`" width="480px" destroy-on-close>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="账单总额">¥ {{ current?.total }}</el-descriptions-item>
        <el-descriptions-item label="已付">¥ {{ current?.paid }}</el-descriptions-item>
        <el-descriptions-item label="待付">
          <span style="color: #f56c6c">¥ {{ current?.balance }}</span>
        </el-descriptions-item>
      </el-descriptions>
      <el-form label-width="90px" style="margin-top: 16px">
        <el-form-item label="收款金额">
          <el-input-number v-model="amount" :min="0" :precision="2" :step="50" />
        </el-form-item>
        <el-form-item label="支付方式">
          <el-select v-model="method" style="width: 200px">
            <el-option label="现金" value="cash" />
            <el-option label="银行卡" value="bank_card" />
            <el-option label="微信" value="wechat" />
            <el-option label="支付宝" value="alipay" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="checkoutVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitCheckout">确认退房</el-button>
      </template>
    </el-dialog>

    <!-- 换房 -->
    <el-dialog v-model="changeRoomVisible" title="换房" width="420px" destroy-on-close>
      <el-form label-width="80px">
        <el-form-item label="当前房间">{{ roomCurrent?.room_no }}</el-form-item>
        <el-form-item label="换至房间">
          <el-select v-model="toRoomId" placeholder="请选择目标房间" style="width: 100%" filterable>
            <el-option v-for="r in availRooms" :key="r.id" :label="`${r.room_no} (${r.room_type_name},${r.floor}楼)`" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="换房原因">
          <el-input v-model="changeReason" placeholder="如：空调故障、客人要求等" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="changeRoomVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitChangeRoom">确认换房</el-button>
      </template>
    </el-dialog>

    <!-- 续住 -->
    <el-dialog v-model="extendVisible" title="续住" width="360px" destroy-on-close>
      <el-form label-width="80px">
        <el-form-item label="当前退房">{{ fmtTime(roomCurrent?.check_out_at) }}</el-form-item>
        <el-form-item label="续住天数">
          <el-input-number v-model="extraNights" :min="1" :max="30" :step="1" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="extendVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitExtend">确认续住</el-button>
      </template>
    </el-dialog>

    <!-- 附加消费 -->
    <el-dialog v-model="chargeVisible" title="附加消费" width="400px" destroy-on-close>
      <el-form label-width="80px">
        <el-form-item label="房间">{{ roomCurrent?.room_no }}</el-form-item>
        <el-form-item label="消费项目">
          <el-input v-model="chargeItem" placeholder="如：迷你吧-可乐" />
        </el-form-item>
        <el-form-item label="单价">
          <el-input-number v-model="chargeAmount" :min="0" :precision="2" :step="1" />
        </el-form-item>
        <el-form-item label="数量">
          <el-input-number v-model="chargeQty" :min="1" :max="999" :step="1" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="chargeVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitCharge">确认添加</el-button>
      </template>
    </el-dialog>

    <!-- 中途收款 -->
    <el-dialog v-model="paymentVisible" title="中途收款" width="400px" destroy-on-close>
      <el-form label-width="80px">
        <el-form-item label="房间">{{ roomCurrent?.room_no }}</el-form-item>
        <el-form-item label="收款金额">
          <el-input-number v-model="payAmount" :min="0" :precision="2" :step="50" />
        </el-form-item>
        <el-form-item label="支付方式">
          <el-select v-model="payMethod" style="width: 200px">
            <el-option label="现金" value="cash" />
            <el-option label="银行卡" value="bank_card" />
            <el-option label="微信" value="wechat" />
            <el-option label="支付宝" value="alipay" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="paymentVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitPayment">确认收款</el-button>
      </template>
    </el-dialog>

    <!-- 账单详情 -->
    <el-dialog v-model="folioVisible" title="账单详情" width="560px" destroy-on-close>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="总额">¥ {{ folio.total }}</el-descriptions-item>
        <el-descriptions-item label="已付">¥ {{ folio.paid }}</el-descriptions-item>
        <el-descriptions-item label="待付">¥ {{ folio.balance }}</el-descriptions-item>
      </el-descriptions>
      <h4 class="sub-title">消费明细</h4>
      <el-table :data="folio.items || []" border size="small">
        <el-table-column label="类型" width="110">
          <template #default="{ row }">{{ itemTypeMap[row.item_type] || row.item_type }}</template>
        </el-table-column>
        <el-table-column prop="amount" label="金额" width="110" />
        <el-table-column prop="remark" label="备注" />
      </el-table>
      <h4 class="sub-title">支付记录</h4>
      <el-table :data="folio.payments || []" border size="small">
        <el-table-column label="方式" width="110">
          <template #default="{ row }">{{ payMethodMap[row.method] || row.method }}</template>
        </el-table-column>
        <el-table-column prop="amount" label="金额" width="110" />
        <el-table-column label="时间" width="160">
          <template #default="{ row }">{{ fmtTime(row.pay_time) }}</template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const itemTypeMap = { room_fee: '房费', goods: '商品', other: '其他' }
const payMethodMap = { cash: '现金', bank_card: '银行卡', wechat: '微信', alipay: '支付宝' }

const stores = ref([])
const storeId = ref(null)
const filterFloor = ref('')
const filterRoomNo = ref('')
const filterGuestName = ref('')
const filterGuestPhone = ref('')
const filterDepositMin = ref(null)
const filterDepositMax = ref(null)
const list = ref([])
const checkoutVisible = ref(false)
const folioVisible = ref(false)
const changeRoomVisible = ref(false)
const extendVisible = ref(false)
const chargeVisible = ref(false)
const paymentVisible = ref(false)
const submitting = ref(false)
const current = ref(null)
const roomCurrent = ref(null)
const amount = ref(0)
const method = ref('cash')
const folio = ref({})
const rooms = ref([])
const availRooms = ref([])
const toRoomId = ref(null)
const changeReason = ref('')
const extraNights = ref(1)
const chargeItem = ref('')
const chargeAmount = ref(0)
const chargeQty = ref(1)
const payAmount = ref(0)
const payMethod = ref('cash')

function fmtTime(t) {
  if (!t) return ''
  return String(t).replace('T', ' ').slice(0, 16)
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
    const params = {}
    if (storeId.value) params.store_id = storeId.value
    if (filterFloor.value) params.floor = filterFloor.value
    if (filterRoomNo.value) params.room_no = filterRoomNo.value
    if (filterGuestName.value) params.guest_name = filterGuestName.value
    if (filterGuestPhone.value) params.guest_phone = filterGuestPhone.value
    if (filterDepositMin.value != null) params.deposit_min = filterDepositMin.value
    if (filterDepositMax.value != null) params.deposit_max = filterDepositMax.value
    const d = await api.listCheckIns(params)
    list.value = d.check_ins || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function clearFilter() {
  filterFloor.value = ''
  filterRoomNo.value = ''
  filterGuestName.value = ''
  filterGuestPhone.value = ''
  filterDepositMin.value = null
  filterDepositMax.value = null
  load()
}

function openCheckout(row) {
  current.value = row
  amount.value = row.balance || 0
  method.value = 'cash'
  checkoutVisible.value = true
}

async function submitCheckout() {
  submitting.value = true
  try {
    await api.checkOut(current.value.id, method.value, amount.value)
    ElMessage.success('退房完成')
    checkoutVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function openFolio(row) {
  folio.value = {}
  folioVisible.value = true
  try {
    folio.value = await api.getFolio(row.id)
  } catch (e) {
    ElMessage.error(e.message)
  }
}

// ---- 换房 ----
async function openChangeRoom(row) {
  roomCurrent.value = row
  toRoomId.value = null
  changeReason.value = ''
  // 加载同门店的可用房间（空净/空脏，排除当前房）
  try {
    const d = await api.listRooms(row.store_id)
    availRooms.value = (d.rooms || []).filter(r => r.id !== row.room_id && (r.status === 0 || r.status === 1))
  } catch (e) {
    availRooms.value = []
  }
  changeRoomVisible.value = true
}

async function submitChangeRoom() {
  if (!toRoomId.value) { ElMessage.warning('请选择目标房间'); return }
  submitting.value = true
  try {
    await api.changeRoom(roomCurrent.value.id, { to_room_id: toRoomId.value, reason: changeReason.value })
    ElMessage.success('换房成功')
    changeRoomVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

// ---- 续住 ----
function openExtend(row) {
  roomCurrent.value = row
  extraNights.value = 1
  extendVisible.value = true
}

async function submitExtend() {
  submitting.value = true
  try {
    await api.extendStay(roomCurrent.value.id, { extra_nights: extraNights.value })
    ElMessage.success('续住成功')
    extendVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

// ---- 附加消费 ----
function openCharge(row) {
  roomCurrent.value = row
  chargeItem.value = ''
  chargeAmount.value = 0
  chargeQty.value = 1
  chargeVisible.value = true
}

async function submitCharge() {
  if (!chargeItem.value) { ElMessage.warning('请输入消费项目'); return }
  if (chargeAmount.value <= 0) { ElMessage.warning('请输入金额'); return }
  submitting.value = true
  try {
    await api.addCharge(roomCurrent.value.id, {
      item: chargeItem.value,
      amount: chargeAmount.value,
      quantity: chargeQty.value
    })
    ElMessage.success('已添加消费')
    chargeVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

// ---- 中途收款 ----
function openPayment(row) {
  roomCurrent.value = row
  payAmount.value = row.balance || 0
  payMethod.value = 'cash'
  paymentVisible.value = true
}

async function submitPayment() {
  if (payAmount.value <= 0) { ElMessage.warning('请输入收款金额'); return }
  submitting.value = true
  try {
    await api.addPayment(roomCurrent.value.id, { amount: payAmount.value, method: payMethod.value })
    ElMessage.success('收款成功')
    paymentVisible.value = false
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
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 16px;
  align-items: center;
}
.sub-title {
  margin: 16px 0 8px;
  font-size: 14px;
  color: #303133;
}
</style>
