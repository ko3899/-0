<template>
  <div class="rate">
    <div class="toolbar">
      <el-select v-model="storeId" placeholder="选择门店" style="width: 220px" @change="loadCalendar">
        <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
      </el-select>
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        style="width: 280px"
      />
      <el-button type="primary" @click="loadCalendar">查询</el-button>
      <span class="tip">点击单元格价格可修改（自动记录价格审计日志）</span>
    </div>

    <el-table :data="rows" border stripe>
      <el-table-column label="房型" prop="room_type_name" width="120" fixed />
      <el-table-column label="价格方案" prop="rate_plan_name" width="110" fixed />
      <el-table-column
        v-for="d in dates"
        :key="d"
        :label="d"
        min-width="100"
        align="center"
      >
        <template #default="{ row }">
          <el-link type="primary" :underline="false" @click="openEdit(row, d)">
            {{ row.cells[d] ? '¥' + Number(row.cells[d].price).toFixed(0) : '-' }}
          </el-link>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="editVisible" title="修改房价" width="420px" destroy-on-close>
      <el-form label-width="90px">
        <el-form-item label="门店">
          <span>{{ storeName }}</span>
        </el-form-item>
        <el-form-item label="房型">
          <span>{{ editForm.room_type_name }}</span>
        </el-form-item>
        <el-form-item label="价格方案">
          <span>{{ editForm.rate_plan_name }}</span>
        </el-form-item>
        <el-form-item label="营业日期">
          <span>{{ editForm.biz_date }}</span>
        </el-form-item>
        <el-form-item label="当前价格">
          <span>¥{{ Number(editForm.old_price).toFixed(2) }}</span>
        </el-form-item>
        <el-form-item label="新价格" required>
          <el-input-number v-model="editForm.price" :min="1" :precision="2" :step="10" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const stores = ref([])
const storeId = ref(null)
const dateRange = ref([])
const calendar = ref([])
const editVisible = ref(false)
const submitting = ref(false)
const editForm = ref({ room_type_id: 0, rate_plan_id: 0, biz_date: '', old_price: 0, price: 0, room_type_name: '', rate_plan_name: '' })

const storeName = computed(() => {
  const s = stores.value.find((x) => x.id === storeId.value)
  return s ? s.name : ''
})

// 日期列：从数据中提取去重排序
const dates = computed(() => {
  const set = new Set()
  for (const it of calendar.value) set.add(it.biz_date)
  return Array.from(set).sort()
})

// 行：按 房型+方案 透视，cells = { biz_date: item }
const rows = computed(() => {
  const map = new Map()
  for (const it of calendar.value) {
    const key = it.room_type_id + '-' + it.rate_plan_id
    if (!map.has(key)) {
      map.set(key, {
        room_type_id: it.room_type_id,
        rate_plan_id: it.rate_plan_id,
        room_type_name: it.room_type_name,
        rate_plan_name: it.rate_plan_name,
        cells: {}
      })
    }
    map.get(key).cells[it.biz_date] = it
  }
  return Array.from(map.values())
})

function todayStr(offset) {
  const t = new Date(Date.now() + offset * 86400000)
  const y = t.getFullYear()
  const m = String(t.getMonth() + 1).padStart(2, '0')
  const d = String(t.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

async function loadStores() {
  try {
    const d = await api.listStores()
    stores.value = d.stores || []
    if (stores.value.length) {
      storeId.value = stores.value[0].id
      loadCalendar()
    }
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function loadCalendar() {
  if (!storeId.value) {
    ElMessage.warning('请先选择门店')
    return
  }
  const [s, e] = dateRange.value
  if (!s || !e) {
    ElMessage.warning('请选择日期范围')
    return
  }
  try {
    const d = await api.listRateCalendar(storeId.value, s, e)
    calendar.value = d.items || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openEdit(row, date) {
  const cell = row.cells[date]
  editForm.value = {
    room_type_id: row.room_type_id,
    rate_plan_id: row.rate_plan_id,
    room_type_name: row.room_type_name,
    rate_plan_name: row.rate_plan_name,
    biz_date: date,
    old_price: cell ? cell.price : 0,
    price: cell ? cell.price : 0
  }
  editVisible.value = true
}

async function submitEdit() {
  if (!editForm.value.price || editForm.value.price <= 0) {
    ElMessage.warning('请输入有效价格')
    return
  }
  submitting.value = true
  try {
    await api.updateRateCalendar({
      store_id: storeId.value,
      room_type_id: editForm.value.room_type_id,
      rate_plan_id: editForm.value.rate_plan_id,
      biz_date: editForm.value.biz_date,
      price: editForm.value.price
    })
    ElMessage.success('价格已更新')
    editVisible.value = false
    loadCalendar()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  dateRange.value = [todayStr(0), todayStr(6)]
  loadStores()
})
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}
.tip {
  font-size: 13px;
  color: #909399;
}
</style>
