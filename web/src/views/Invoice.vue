<template>
  <div class="page invoice-page">
    <div class="page-header">
      <span class="page-title">发票管理</span>
    </div>

    <el-tabs v-model="activeTab">
      <!-- 发票记录 -->
      <el-tab-pane label="发票记录" name="invoices">
        <!-- 汇总 -->
        <div class="summary-grid">
          <div class="summary-card">
            <div class="summary-num">{{ summary.total_count ?? 0 }}</div>
            <div class="summary-label">已开发票数</div>
          </div>
          <div class="summary-card">
            <div class="summary-num">¥{{ Number(summary.total_amount || 0).toFixed(2) }}</div>
            <div class="summary-label">开票总额</div>
          </div>
          <div class="summary-card">
            <div class="summary-num">¥{{ Number(summary.total_tax || 0).toFixed(2) }}</div>
            <div class="summary-label">税额合计</div>
          </div>
        </div>

        <!-- 工具栏 -->
        <el-card shadow="never" class="toolbar-card">
          <div class="toolbar">
            <el-select v-model="storeId" placeholder="全部门店" clearable style="width: 180px" @change="loadInvoices">
              <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
            </el-select>
            <el-select v-model="filterStatus" placeholder="全部状态" clearable style="width: 120px" @change="loadInvoices">
              <el-option label="已开" :value="1" />
              <el-option label="作废" :value="2" />
            </el-select>
            <el-input v-model="keyword" placeholder="发票号/抬头" clearable style="width: 180px" @clear="loadInvoices" @keyup.enter="loadInvoices" />
            <el-button type="primary" @click="loadInvoices">查询</el-button>
            <el-button type="success" @click="openCreate">开具发票</el-button>
          </div>
        </el-card>

        <el-card shadow="never" v-loading="loading">
          <el-table :data="invoices" border stripe size="small">
            <el-table-column prop="invoice_no" label="发票号" width="170" />
            <el-table-column prop="store_name" label="门店" width="140" />
            <el-table-column prop="title_name" label="抬头" min-width="160" show-overflow-tooltip />
            <el-table-column label="类型" width="90">
              <template #default="{ row }">{{ invTypeMap[row.invoice_type] }}</template>
            </el-table-column>
            <el-table-column label="金额" width="100">
              <template #default="{ row }">¥{{ Number(row.amount).toFixed(2) }}</template>
            </el-table-column>
            <el-table-column label="税额" width="90">
              <template #default="{ row }">¥{{ Number(row.tax_amount).toFixed(2) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="70">
              <template #default="{ row }">
                <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
                  {{ invStatusMap[row.status] }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="issued_by" label="开票人" width="90" />
            <el-table-column label="开票时间" width="150">
              <template #default="{ row }">{{ fmtTime(row.issued_at || row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="80" fixed="right">
              <template #default="{ row }">
                <el-button v-if="row.status === 1" link type="danger" @click="doVoid(row)">作废</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- 发票抬头 -->
      <el-tab-pane label="发票抬头" name="titles">
        <el-card shadow="never">
          <div class="toolbar" style="margin-bottom: 12px">
            <el-button type="primary" @click="openCreateTitle">新增抬头</el-button>
            <el-button @click="loadTitles">刷新</el-button>
          </div>
          <el-table :data="titles" border stripe size="small">
            <el-table-column label="类型" width="70">
              <template #default="{ row }">
                <el-tag :type="row.title_type === 1 ? '' : 'info'" size="small">{{ row.title_type === 1 ? '企业' : '个人' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="title_name" label="抬头名称" min-width="180" />
            <el-table-column prop="tax_no" label="税号" width="160" />
            <el-table-column prop="phone" label="电话" width="130" />
            <el-table-column prop="email" label="邮箱" width="160" />
            <el-table-column label="默认" width="60">
              <template #default="{ row }">
                <el-tag v-if="row.is_default === 1" type="success" size="small">默认</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="120" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openEditTitle(row)">编辑</el-button>
                <el-button link type="danger" @click="doDeleteTitle(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 开具发票弹窗 -->
    <el-dialog v-model="createVisible" title="开具发票" width="520px" destroy-on-close>
      <el-form :model="createForm" label-width="90px">
        <el-form-item label="门店" required>
          <el-select v-model="createForm.store_id" placeholder="选择门店" style="width: 100%">
            <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="入住单号">
          <el-input v-model="createForm.check_in_id" placeholder="选填，输入入住单号查询账单" @change="lookupFolio">
            <template #append>
              <el-button @click="lookupFolio">查询</el-button>
            </template>
          </el-input>
          <div v-if="folioInfo" class="folio-tip">
            账单总额 ¥{{ Number(folioInfo.total).toFixed(2) }} · 已付 ¥{{ Number(folioInfo.paid).toFixed(2) }}
          </div>
        </el-form-item>
        <el-form-item label="发票抬头" required>
          <el-select v-model="createForm.title_id" placeholder="选择抬头" filterable style="width: 100%" @change="onTitleChange">
            <el-option v-for="t in titles" :key="t.id" :label="`${t.title_name}${t.title_type === 1 ? '（企业）' : '（个人）'}`" :value="t.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="发票类型">
          <el-select v-model="createForm.invoice_type" style="width: 100%">
            <el-option label="增值税普通发票" :value="0" />
            <el-option label="增值税专用发票" :value="1" />
            <el-option label="电子普通发票" :value="2" />
          </el-select>
        </el-form-item>
        <el-form-item label="开票金额" required>
          <el-input-number v-model="createForm.amount" :min="0" :precision="2" :step="100" style="width: 100%" />
        </el-form-item>
        <el-form-item label="税额">
          <el-input-number v-model="createForm.tax_amount" :min="0" :precision="2" :step="10" style="width: 100%" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="createForm.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="doCreate">确认开票</el-button>
      </template>
    </el-dialog>

    <!-- 抬头弹窗 -->
    <el-dialog v-model="titleVisible" :title="editingTitle ? '编辑抬头' : '新增抬头'" width="520px" destroy-on-close>
      <el-form :model="titleForm" label-width="90px">
        <el-form-item label="类型">
          <el-radio-group v-model="titleForm.title_type">
            <el-radio :value="0">个人</el-radio>
            <el-radio :value="1">企业</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="抬头名称" required>
          <el-input v-model="titleForm.title_name" placeholder="个人姓名或企业名称" />
        </el-form-item>
        <el-form-item label="税号" v-if="titleForm.title_type === 1" required>
          <el-input v-model="titleForm.tax_no" placeholder="企业统一社会信用代码" />
        </el-form-item>
        <el-form-item label="地址">
          <el-input v-model="titleForm.address" />
        </el-form-item>
        <el-form-item label="电话">
          <el-input v-model="titleForm.phone" />
        </el-form-item>
        <el-form-item label="开户银行" v-if="titleForm.title_type === 1">
          <el-input v-model="titleForm.bank_name" />
        </el-form-item>
        <el-form-item label="银行账号" v-if="titleForm.title_type === 1">
          <el-input v-model="titleForm.bank_account" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="titleForm.email" placeholder="电子发票推送邮箱" />
        </el-form-item>
        <el-form-item label="默认抬头">
          <el-switch v-model="titleForm.is_default" :active-value="1" :inactive-value="0" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="titleVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="doSaveTitle">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'

const activeTab = ref('invoices')
const stores = ref([])
const invoices = ref([])
const titles = ref([])
const summary = ref({})
const loading = ref(false)
const submitting = ref(false)

const storeId = ref(Number(localStorage.getItem('current_store') || 0))
const filterStatus = ref(null)
const keyword = ref('')

const invTypeMap = { 0: '普票', 1: '专票', 2: '电子' }
const invStatusMap = { 0: '待开', 1: '已开', 2: '作废', 3: '红冲' }

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

async function loadInvoices() {
  loading.value = true
  try {
    const params = {}
    if (storeId.value) params.store_id = storeId.value
    if (filterStatus.value !== null && filterStatus.value !== '') params.status = filterStatus.value
    if (keyword.value) params.keyword = keyword.value
    const r = await api.listInvoices(params)
    invoices.value = r.invoices || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function loadSummary() {
  try {
    const params = {}
    if (storeId.value) params.store_id = storeId.value
    summary.value = await api.invoiceSummary(params)
  } catch (e) { /* 静默 */ }
}

async function loadTitles() {
  try {
    const r = await api.listInvoiceTitles()
    titles.value = r.titles || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

watch(activeTab, (t) => {
  if (t === 'titles' && titles.value.length === 0) loadTitles()
})

// 开具发票
const createVisible = ref(false)
const createForm = ref({ store_id: null, check_in_id: '', title_id: null, invoice_type: 0, amount: 0, tax_amount: 0, remark: '' })
const folioInfo = ref(null)

function openCreate() {
  createForm.value = {
    store_id: storeId.value || null,
    check_in_id: '',
    title_id: null,
    invoice_type: 0,
    amount: 0,
    tax_amount: 0,
    remark: ''
  }
  folioInfo.value = null
  if (titles.value.length === 0) loadTitles()
  createVisible.value = true
}

async function lookupFolio() {
  const cid = createForm.value.check_in_id
  if (!cid) { folioInfo.value = null; return }
  try {
    const f = await api.getFolio(Number(cid))
    folioInfo.value = f
    if (f.paid > 0) createForm.value.amount = f.paid
  } catch (e) {
    folioInfo.value = null
    ElMessage.warning('未找到该入住单的账单')
  }
}

function onTitleChange() {
  // 可在此预填客户关联，暂无需处理
}

async function doCreate() {
  if (!createForm.value.store_id || !createForm.value.title_id) {
    ElMessage.warning('请选择门店和发票抬头')
    return
  }
  if (createForm.value.amount <= 0) {
    ElMessage.warning('开票金额必须大于 0')
    return
  }
  submitting.value = true
  try {
    const payload = {
      store_id: createForm.value.store_id,
      title_id: createForm.value.title_id,
      invoice_type: createForm.value.invoice_type,
      amount: createForm.value.amount,
      tax_amount: createForm.value.tax_amount,
      remark: createForm.value.remark
    }
    if (folioInfo.value) payload.folio_id = folioInfo.value.folio_id
    const res = await api.createInvoice(payload)
    ElMessage.success(`开票成功：${res.invoice_no}`)
    createVisible.value = false
    loadInvoices()
    loadSummary()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function doVoid(row) {
  try {
    await ElMessageBox.confirm(`确认作废发票 ${row.invoice_no}？`, '作废确认', { type: 'warning' })
    await api.voidInvoice(row.id)
    ElMessage.success('已作废')
    loadInvoices()
    loadSummary()
  } catch (e) {
    if (e !== 'cancel' && e.message) ElMessage.error(e.message)
  }
}

// 抬头管理
const titleVisible = ref(false)
const editingTitle = ref(null)
const titleForm = ref({ title_type: 0, title_name: '', tax_no: '', address: '', phone: '', bank_name: '', bank_account: '', email: '', is_default: 0 })

function openCreateTitle() {
  editingTitle.value = null
  titleForm.value = { title_type: 0, title_name: '', tax_no: '', address: '', phone: '', bank_name: '', bank_account: '', email: '', is_default: 0 }
  titleVisible.value = true
}

function openEditTitle(row) {
  editingTitle.value = row
  titleForm.value = { ...row }
  titleVisible.value = true
}

async function doSaveTitle() {
  if (!titleForm.value.title_name) {
    ElMessage.warning('抬头名称不能为空')
    return
  }
  if (titleForm.value.title_type === 1 && !titleForm.value.tax_no) {
    ElMessage.warning('企业抬头需填写税号')
    return
  }
  submitting.value = true
  try {
    if (editingTitle.value) {
      await api.updateInvoiceTitle(editingTitle.value.id, titleForm.value)
    } else {
      await api.createInvoiceTitle(titleForm.value)
    }
    ElMessage.success('保存成功')
    titleVisible.value = false
    loadTitles()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function doDeleteTitle(row) {
  try {
    await ElMessageBox.confirm(`确认删除抬头「${row.title_name}」？`, '删除确认', { type: 'warning' })
    await api.deleteInvoiceTitle(row.id)
    ElMessage.success('已删除')
    loadTitles()
  } catch (e) {
    if (e !== 'cancel' && e.message) ElMessage.error(e.message)
  }
}

onMounted(() => {
  loadStores()
  loadInvoices()
  loadSummary()
})
</script>

<style scoped>
.invoice-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; }
.page-title { font-size: 18px; font-weight: 600; color: var(--text-1); }

.summary-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-bottom: 16px; }
.summary-card {
  background: #fff; border-radius: var(--radius-md); padding: 20px; text-align: center;
  box-shadow: var(--shadow-card);
}
.summary-num { font-size: 22px; font-weight: 700; color: var(--text-1); }
.summary-label { font-size: 12px; color: var(--text-2); margin-top: 4px; }

.toolbar { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }
.folio-tip { font-size: 12px; color: var(--text-2); margin-top: 4px; }
</style>
