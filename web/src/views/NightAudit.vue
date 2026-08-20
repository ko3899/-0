<template>
  <div class="page night-audit">
    <div class="page-header">
      <span class="page-title">夜审管理</span>
      <div>
        <el-button @click="loadHistory">刷新历史</el-button>
        <el-button type="primary" :loading="running" @click="onRun" :disabled="!canRun">
          执行夜审
        </el-button>
      </div>
    </div>

    <!-- 当前营业日状态 -->
    <el-card shadow="never" class="status-card">
      <div class="status-row">
        <div class="status-item">
          <div class="status-label">当前营业日</div>
          <div class="status-value">{{ current.biz_date || '—' }}</div>
        </div>
        <div class="status-item">
          <div class="status-label">审核状态</div>
          <el-tag :type="current.audited ? 'success' : 'warning'">
            {{ current.audited ? '已完成' : '待夜审' }}
          </el-tag>
        </div>
        <div class="status-item">
          <div class="status-label">上次夜审</div>
          <div class="status-value small">{{ current.last_audit_at || '从未夜审' }}</div>
          <div class="status-sub" v-if="current.last_audit_by">{{ current.last_audit_by }}</div>
        </div>
        <div class="status-item" v-if="current.audited">
          <el-alert title="该营业日已完成夜审，下一营业日为" type="info" :closable="false" show-icon>
            <template #default>
              下一营业日：{{ nextBizDate }}
            </template>
          </el-alert>
        </div>
      </div>
    </el-card>

    <!-- 夜审预览 -->
    <el-card shadow="never" v-loading="loading">
      <template #header>
        <span class="card-title">夜审预览 · {{ preview.biz_date || '—' }}</span>
      </template>

      <div class="preview-grid">
        <div class="preview-item">
          <div class="preview-num">{{ preview.today_revenue ? '¥' + Number(preview.today_revenue).toFixed(2) : '¥0' }}</div>
          <div class="preview-label">当日营收</div>
        </div>
        <div class="preview-item">
          <div class="preview-num">{{ preview.today_checkin ?? 0 }}</div>
          <div class="preview-label">当日入住</div>
        </div>
        <div class="preview-item">
          <div class="preview-num">{{ preview.today_checkout ?? 0 }}</div>
          <div class="preview-label">当日退房</div>
        </div>
        <div class="preview-item">
          <div class="preview-num">{{ preview.in_house_count ?? 0 }}</div>
          <div class="preview-label">当前在住</div>
        </div>
        <div class="preview-item warn">
          <div class="preview-num">{{ preview.to_post_count ?? 0 }}</div>
          <div class="preview-label">待补过账(超期)</div>
        </div>
        <div class="preview-item danger">
          <div class="preview-num">¥{{ Number(preview.to_post_amount || 0).toFixed(2) }}</div>
          <div class="preview-label">补过账金额</div>
        </div>
      </div>

      <!-- 异常清单 -->
      <div v-if="(preview.overdue && preview.overdue.length) || (preview.unpaid && preview.unpaid.length)" class="exception-box">
        <el-alert
          v-if="preview.overdue && preview.overdue.length"
          type="warning"
          :closable="false"
          show-icon
          class="exc-alert"
        >
          <template #title>超期未退房 {{ preview.overdue.length }} 间</template>
          <div class="exc-list">
            <el-tag v-for="r in preview.overdue" :key="r.check_in_id" type="warning" class="exc-tag">
              {{ r.room_no }} {{ r.guest_name }}（应退 {{ r.expected_checkout }}）
            </el-tag>
          </div>
        </el-alert>
        <el-alert
          v-if="preview.unpaid && preview.unpaid.length"
          type="error"
          :closable="false"
          show-icon
          class="exc-alert"
        >
          <template #title>账单未结 {{ preview.unpaid.length }} 笔</template>
          <div class="exc-list">
            <el-tag v-for="r in preview.unpaid" :key="'u'+r.check_in_id" type="danger" class="exc-tag">
              {{ r.room_no }} {{ r.guest_name }} 待收 ¥{{ Number(r.balance).toFixed(2) }}
            </el-tag>
          </div>
        </el-alert>
      </div>

      <!-- 待补过账清单 -->
      <div v-if="preview.to_post && preview.to_post.length" class="to-post-box">
        <div class="section-title">将自动补过账的房间</div>
        <el-table :data="preview.to_post" border size="small">
          <el-table-column prop="room_no" label="房号" width="80" />
          <el-table-column prop="guest_name" label="客人" width="100" />
          <el-table-column prop="store_name" label="门店" width="150" />
          <el-table-column label="应退时间" width="150">
            <template #default="{ row }">{{ row.expected_checkout }}</template>
          </el-table-column>
          <el-table-column label="补过账金额" width="120">
            <template #default="{ row }">
              <span style="color: #f56c6c; font-weight: 600">¥{{ Number(row.today_price).toFixed(2) }}</span>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-card>

    <!-- 夜审历史 -->
    <el-card shadow="never">
      <template #header><span class="card-title">夜审历史</span></template>
      <el-table :data="history" border stripe size="small">
        <el-table-column prop="biz_date" label="营业日" width="110" />
        <el-table-column prop="operator" label="操作人" width="100" />
        <el-table-column prop="completed_at" label="完成时间" width="160" />
        <el-table-column label="营收" width="110">
          <template #default="{ row }">¥{{ Number(row.revenue).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="checkins" label="入住" width="70" />
        <el-table-column prop="checkouts" label="退房" width="70" />
        <el-table-column prop="in_house" label="在住" width="70" />
        <el-table-column label="补过账" width="120">
          <template #default="{ row }">
            <span v-if="row.posted_count > 0">{{ row.posted_count }}笔 ¥{{ Number(row.posted_amount).toFixed(2) }}</span>
            <span v-else style="color: #c0c4cc">—</span>
          </template>
        </el-table-column>
        <el-table-column prop="exceptions" label="异常" min-width="160" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'

const current = ref({})
const preview = ref({})
const history = ref([])
const loading = ref(false)
const running = ref(false)

const canRun = computed(() => !current.value.audited)
const nextBizDate = computed(() => {
  if (!current.value.biz_date) return ''
  const d = new Date(current.value.biz_date)
  d.setDate(d.getDate() + 1)
  return d.toISOString().slice(0, 10)
})

async function loadCurrent() {
  try {
    current.value = await api.nightAuditCurrent()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function loadPreview() {
  loading.value = true
  try {
    preview.value = await api.nightAuditPreview()
  } catch (e) {
    if (e.message && e.message.includes('已完成夜审')) {
      preview.value = { biz_date: current.value.biz_date, in_house: [], to_post: [], overdue: [], unpaid: [] }
    } else {
      ElMessage.error(e.message)
    }
  } finally {
    loading.value = false
  }
}

async function loadHistory() {
  try {
    const r = await api.nightAuditHistory({ page: 1, page_size: 30 })
    history.value = r.items || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function onRun() {
  try {
    await ElMessageBox.confirm(
      `确认对营业日 ${current.value.biz_date} 执行夜审？\n执行后将自动补过账并锁定该营业日，不可重复。`,
      '夜审确认',
      { type: 'warning', confirmButtonText: '执行夜审', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  running.value = true
  try {
    const res = await api.nightAuditRun()
    ElMessage.success(`夜审完成：补过账 ${res.posted_count} 笔，营收 ¥${Number(res.revenue).toFixed(2)}`)
    await loadCurrent()
    await loadPreview()
    await loadHistory()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    running.value = false
  }
}

onMounted(() => {
  loadCurrent()
  loadPreview()
  loadHistory()
})
</script>

<style scoped>
.night-audit { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; }
.page-title { font-size: 18px; font-weight: 600; color: var(--text-1); }

.status-card .status-row { display: flex; gap: 40px; align-items: center; flex-wrap: wrap; }
.status-item { display: flex; flex-direction: column; gap: 6px; }
.status-label { font-size: 12px; color: var(--text-3); }
.status-value { font-size: 22px; font-weight: 700; color: var(--text-1); }
.status-value.small { font-size: 15px; font-weight: 600; }
.status-sub { font-size: 12px; color: var(--text-2); }

.card-title { font-weight: 600; color: var(--text-1); }

.preview-grid {
  display: grid; grid-template-columns: repeat(6, 1fr); gap: 12px; margin-bottom: 16px;
}
.preview-item {
  background: var(--bg-page); border-radius: var(--radius-md); padding: 16px; text-align: center;
}
.preview-item.warn { background: #fef0e6; }
.preview-item.danger { background: #fee; }
.preview-num { font-size: 20px; font-weight: 700; color: var(--text-1); }
.preview-item.warn .preview-num { color: #e6a23c; }
.preview-item.danger .preview-num { color: #f56c6c; }
.preview-label { font-size: 12px; color: var(--text-2); margin-top: 4px; }

.exception-box { margin-bottom: 16px; display: flex; flex-direction: column; gap: 10px; }
.exc-alert { margin: 0; }
.exc-list { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
.exc-tag { font-size: 12px; }

.to-post-box { margin-top: 8px; }
.section-title { font-weight: 600; color: var(--text-1); margin-bottom: 10px; }

@media (max-width: 1200px) { .preview-grid { grid-template-columns: repeat(3, 1fr); } }
</style>
