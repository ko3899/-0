<template>
  <div class="dashboard">
    <div class="cards">
      <div v-for="c in cards" :key="c.label" class="card">
        <div class="card-value" :style="{ color: c.color }">{{ c.value }}</div>
        <div class="card-label">{{ c.label }}</div>
      </div>
    </div>

    <el-row :gutter="16">
      <el-col :span="14">
        <el-card shadow="never">
          <template #header>营收汇总（按门店）</template>
          <el-table :data="revenue" border stripe>
            <el-table-column prop="store_name" label="门店" />
            <el-table-column label="今日营收" width="110" align="right">
              <template #default="{ row }">¥{{ row.today_revenue.toFixed(2) }}</template>
            </el-table-column>
            <el-table-column label="在住待收" width="110" align="right">
              <template #default="{ row }">¥{{ row.pending_balance.toFixed(2) }}</template>
            </el-table-column>
            <el-table-column label="累计营收" width="110" align="right">
              <template #default="{ row }">¥{{ row.total_revenue.toFixed(2) }}</template>
            </el-table-column>
            <el-table-column prop="in_house" label="在住" width="70" align="center" />
          </el-table>
        </el-card>
      </el-col>
      <el-col :span="10">
        <el-card shadow="never">
          <template #header>房态分布与入住率</template>
          <el-table :data="occupancy" border stripe>
            <el-table-column prop="store_name" label="门店" />
            <el-table-column label="入住率" width="130">
              <template #default="{ row }">
                <el-progress :percentage="Math.round(row.occupancy)" :stroke-width="14" />
              </template>
            </el-table-column>
            <el-table-column label="在住/总房" width="90" align="center">
              <template #default="{ row }">{{ row.occupied }}/{{ row.total }}</template>
            </el-table-column>
            <el-table-column label="空净/空脏" width="90" align="center">
              <template #default="{ row }">{{ row.clean }}/{{ row.dirty }}</template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const summary = ref({})
const revenue = ref([])
const occupancy = ref([])

const cards = computed(() => [
  { label: '营业门店', value: summary.value.stores ?? 0, color: '#409eff' },
  { label: '房间总数', value: summary.value.rooms ?? 0, color: '#303133' },
  { label: '当前在住', value: summary.value.occupied ?? 0, color: '#67c23a' },
  { label: '空净房', value: summary.value.clean_rooms ?? 0, color: '#909399' },
  { label: '今日入住', value: summary.value.today_checkin ?? 0, color: '#e6a23c' },
  { label: '今日退房', value: summary.value.today_checkout ?? 0, color: '#f56c6c' },
  { label: '今日营收', value: '¥' + Number(summary.value.today_revenue ?? 0).toFixed(2), color: '#e6a23c' },
  { label: '待收款总额', value: '¥' + Number(summary.value.pending_balance ?? 0).toFixed(2), color: '#f56c6c' }
])

async function load() {
  try {
    const [d, r, o] = await Promise.all([
      api.dashboard(),
      api.revenueReport(),
      api.occupancyReport()
    ])
    summary.value = d.summary || {}
    revenue.value = r.items || []
    occupancy.value = o.items || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<style scoped>
.cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 16px;
}
.card {
  background: #fff;
  border-radius: 8px;
  padding: 20px;
  text-align: center;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}
.card-value {
  font-size: 26px;
  font-weight: bold;
  margin-bottom: 6px;
}
.card-label {
  font-size: 13px;
  color: #909399;
}
</style>
