<template>
  <div class="checkin">
    <div class="toolbar">
      <el-select v-model="storeId" placeholder="全部门店" clearable style="width: 200px" @change="load">
        <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
      </el-select>
      <el-button type="primary" @click="load">刷新</el-button>
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
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openCheckout(row)">退房</el-button>
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
const list = ref([])
const checkoutVisible = ref(false)
const folioVisible = ref(false)
const submitting = ref(false)
const current = ref(null)
const amount = ref(0)
const method = ref('cash')
const folio = ref({})

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
    const d = await api.listCheckIns(storeId.value)
    list.value = d.check_ins || []
  } catch (e) {
    ElMessage.error(e.message)
  }
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
.sub-title {
  margin: 16px 0 8px;
  font-size: 14px;
  color: #303133;
}
</style>
