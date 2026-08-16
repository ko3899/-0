<template>
  <div class="dashboard">
    <!-- KPI 卡片 -->
    <div class="kpi-grid">
      <div v-for="k in kpis" :key="k.label" class="kpi-card">
        <div class="kpi-icon" :style="{ background: k.bg }">
          <el-icon :size="24"><component :is="k.icon" /></el-icon>
        </div>
        <div class="kpi-meta">
          <div class="kpi-value">{{ k.value }}</div>
          <div class="kpi-label">{{ k.label }}</div>
          <div class="kpi-sub" v-if="k.sub">{{ k.sub }}</div>
        </div>
      </div>
    </div>

    <!-- 营收趋势 -->
    <el-card class="chart-card" shadow="never">
      <template #header>
        <div class="chart-head"><span class="chart-title">近 14 天营收与入住趋势</span></div>
      </template>
      <div ref="trendRef" class="chart-box"></div>
    </el-card>

    <!-- 双图 -->
    <div class="chart-row">
      <el-card class="chart-card" shadow="never">
        <template #header>
          <div class="chart-head"><span class="chart-title">各门店营收对比</span></div>
        </template>
        <div ref="revenueRef" class="chart-box"></div>
      </el-card>
      <el-card class="chart-card" shadow="never">
        <template #header>
          <div class="chart-head"><span class="chart-title">全集团房态分布</span></div>
        </template>
        <div ref="roomRef" class="chart-box"></div>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { Shop, House, Money, Wallet } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import { api } from '../api'

const summary = ref({})
const revenue = ref([])
const occupancy = ref([])
const trend = ref([])

const trendRef = ref(null)
const revenueRef = ref(null)
const roomRef = ref(null)
let charts = []

const totalRooms = computed(() => occupancy.value.reduce((s, o) => s + (o.total || 0), 0))
const totalOccupied = computed(() => occupancy.value.reduce((s, o) => s + (o.occupied || 0), 0))
const totalRevenue = computed(() => revenue.value.reduce((s, r) => s + Number(r.total_revenue || 0), 0))
const occRate = computed(() => totalRooms.value ? Math.round(totalOccupied.value / totalRooms.value * 100) : 0)

const kpis = computed(() => [
  {
    label: '营业门店', value: summary.value.stores ?? 0,
    sub: `总房间 ${summary.value.rooms ?? 0} 间`,
    icon: Shop, bg: 'linear-gradient(135deg,#2b5a9c,#3d7bd4)'
  },
  {
    label: '当前在住', value: summary.value.occupied ?? 0,
    sub: `入住率 ${occRate.value}%`,
    icon: House, bg: 'linear-gradient(135deg,#0ba360,#3cba92)'
  },
  {
    label: '今日营收', value: '¥' + Number(summary.value.today_revenue ?? 0).toFixed(2),
    sub: `累计 ¥${totalRevenue.value.toFixed(0)}`,
    icon: Money, bg: 'linear-gradient(135deg,#f7971e,#ffd200)'
  },
  {
    label: '待收款总额', value: '¥' + Number(summary.value.pending_balance ?? 0).toFixed(2),
    sub: `今日入住 ${summary.value.today_checkin ?? 0} · 退房 ${summary.value.today_checkout ?? 0}`,
    icon: Wallet, bg: 'linear-gradient(135deg,#eb3349,#f45c43)'
  }
])

async function load() {
  try {
    const [d, r, o, t] = await Promise.all([
      api.dashboard(),
      api.revenueReport(),
      api.occupancyReport(),
      api.trendReport()
    ])
    summary.value = d.summary || {}
    revenue.value = r.items || []
    occupancy.value = o.items || []
    trend.value = t.items || []
    renderCharts()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function renderCharts() {
  renderTrend()
  renderRevenue()
  renderRoom()
}

function renderTrend() {
  const el = trendRef.value
  if (!el) return
  const chart = echarts.init(el)
  const dates = trend.value.map(t => (t.date || '').slice(5))
  const revenues = trend.value.map(t => t.revenue)
  const checkins = trend.value.map(t => t.checkins)
  chart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['营收(元)', '入住(间)'], top: 0 },
    grid: { left: 52, right: 52, top: 42, bottom: 28 },
    xAxis: { type: 'category', data: dates, boundaryGap: false, axisLine: { lineStyle: { color: '#c0c7d1' } } },
    yAxis: [
      { type: 'value', name: '营收(元)', splitLine: { lineStyle: { color: '#eef1f5' } } },
      { type: 'value', name: '入住(间)', splitLine: { show: false } }
    ],
    series: [
      { name: '营收(元)', type: 'line', smooth: true, data: revenues, itemStyle: { color: '#3d7bd4' }, lineStyle: { width: 2.5 }, areaStyle: { color: 'rgba(61,123,212,0.12)' } },
      { name: '入住(间)', type: 'line', smooth: true, yAxisIndex: 1, data: checkins, itemStyle: { color: '#10b981' }, lineStyle: { width: 2 } }
    ]
  })
  charts.push(chart)
}

function renderRevenue() {
  const el = revenueRef.value
  if (!el) return
  const chart = echarts.init(el)
  const names = revenue.value.map(r => r.store_name)
  chart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['今日营收', '在住待收', '累计营收'], top: 0 },
    grid: { left: 52, right: 16, top: 42, bottom: 28 },
    xAxis: { type: 'category', data: names, axisLine: { lineStyle: { color: '#c0c7d1' } } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: '#eef1f5' } } },
    series: [
      { name: '今日营收', type: 'bar', barMaxWidth: 22, data: revenue.value.map(r => r.today_revenue), itemStyle: { color: '#3d7bd4', borderRadius: [4, 4, 0, 0] } },
      { name: '在住待收', type: 'bar', barMaxWidth: 22, data: revenue.value.map(r => r.pending_balance), itemStyle: { color: '#f59e0b', borderRadius: [4, 4, 0, 0] } },
      { name: '累计营收', type: 'bar', barMaxWidth: 22, data: revenue.value.map(r => r.total_revenue), itemStyle: { color: '#10b981', borderRadius: [4, 4, 0, 0] } }
    ]
  })
  charts.push(chart)
}

function renderRoom() {
  const el = roomRef.value
  if (!el) return
  const chart = echarts.init(el)
  const clean = occupancy.value.reduce((s, o) => s + (o.clean || 0), 0)
  const dirty = occupancy.value.reduce((s, o) => s + (o.dirty || 0), 0)
  const occupied = occupancy.value.reduce((s, o) => s + (o.occupied || 0), 0)
  const maint = occupancy.value.reduce((s, o) => s + (o.maint || 0), 0)
  const reserved = occupancy.value.reduce((s, o) => s + (o.reserved || 0), 0)
  chart.setOption({
    tooltip: { trigger: 'item', formatter: '{b}: {c} 间 ({d}%)' },
    legend: { orient: 'vertical', right: 8, top: 'center', itemGap: 12 },
    series: [{
      type: 'pie',
      radius: ['45%', '68%'],
      center: ['38%', '50%'],
      avoidLabelOverlap: false,
      itemStyle: { borderRadius: 6, borderColor: '#fff', borderWidth: 2 },
      label: { show: false },
      data: [
        { name: '空净', value: clean, itemStyle: { color: '#10b981' } },
        { name: '空脏', value: dirty, itemStyle: { color: '#f59e0b' } },
        { name: '住客', value: occupied, itemStyle: { color: '#ef4444' } },
        { name: '维修', value: maint, itemStyle: { color: '#94a3b8' } },
        { name: '预留', value: reserved, itemStyle: { color: '#60a5fa' } }
      ]
    }]
  })
  charts.push(chart)
}

function handleResize() {
  charts.forEach(c => c.resize())
}

onMounted(() => {
  load()
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  charts.forEach(c => c.dispose())
  charts = []
})
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.kpi-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}
.kpi-card {
  background: #fff;
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  transition: transform .2s, box-shadow .2s;
}
.kpi-card:hover {
  transform: translateY(-3px);
  box-shadow: var(--shadow-hover);
}
.kpi-icon {
  width: 52px;
  height: 52px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  flex-shrink: 0;
}
.kpi-meta {
  min-width: 0;
}
.kpi-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-1);
  line-height: 1.2;
  white-space: nowrap;
}
.kpi-label {
  font-size: 13px;
  color: var(--text-2);
  margin-top: 4px;
}
.kpi-sub {
  font-size: 12px;
  color: var(--text-3);
  margin-top: 2px;
}

.chart-card {
  width: 100%;
}
.chart-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.chart-title {
  font-weight: 600;
  color: var(--text-1);
}
.chart-box {
  height: 300px;
}
.chart-row {
  display: grid;
  grid-template-columns: 3fr 2fr;
  gap: 16px;
}

@media (max-width: 1200px) {
  .kpi-grid { grid-template-columns: repeat(2, 1fr); }
  .chart-row { grid-template-columns: 1fr; }
}
</style>
