<template>
  <div class="ota-config">
    <el-tabs v-model="activeTab" type="border-card" @tab-change="onTabChange">
      <!-- 渠道管理 -->
      <el-tab-pane label="渠道管理" name="channels">
        <div class="toolbar">
          <el-button type="primary" @click="openChannelDialog()">新增渠道</el-button>
          <span class="tip">配置各 OTA 平台的 API 连接信息</span>
        </div>
        <el-table :data="channels" border stripe v-loading="loading">
          <el-table-column label="门店" prop="store_name" width="160" />
          <el-table-column label="渠道名称" prop="name" width="140" />
          <el-table-column label="渠道编码" width="120">
            <template #default="{ row }">
              <el-tag :type="channelTag(row.channel_code)" size="small">{{ row.channel_code }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="API 地址" prop="api_url" min-width="200" />
          <el-table-column label="酒店 ID" prop="hotel_id" width="120" />
          <el-table-column label="状态" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="最近同步" prop="synced_at" width="160" />
          <el-table-column label="操作" width="280" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="success" @click="syncChannel(row)">同步房态</el-button>
              <el-button size="small" @click="openChannelDialog(row)">编辑</el-button>
              <el-button size="small" type="danger" @click="deleteChannel(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- 房型映射 -->
      <el-tab-pane label="房型映射" name="mappings">
        <div class="toolbar">
          <el-select v-model="filterChannelId" placeholder="选择渠道" style="width: 200px" @change="loadMappings" clearable>
            <el-option v-for="c in channels" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
          <el-button type="primary" @click="openMappingDialog()" :disabled="!filterChannelId">新增映射</el-button>
          <el-button @click="previewInventory" :disabled="!filterChannelId">预览待同步房态</el-button>
        </div>
        <el-table :data="mappings" border stripe v-loading="loading">
          <el-table-column label="渠道" prop="channel_name" width="140" />
          <el-table-column label="PMS 房型" prop="room_type_name" width="140" />
          <el-table-column label="OTA 房型 ID" prop="ota_room_type_id" width="150" />
          <el-table-column label="OTA 房型名称" prop="ota_room_name" width="150" />
          <el-table-column label="价格系数" width="100" align="center">
            <template #default="{ row }">{{ (row.price_factor * 100).toFixed(0) }}%</template>
          </el-table-column>
          <el-table-column label="自动同步" width="90" align="center">
            <template #default="{ row }">
              <el-tag :type="row.auto_sync ? 'success' : 'info'" size="small">{{ row.auto_sync ? '是' : '否' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="160" fixed="right">
            <template #default="{ row }">
              <el-button size="small" @click="openMappingDialog(row)">编辑</el-button>
              <el-button size="small" type="danger" @click="deleteMapping(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- 同步日志 -->
      <el-tab-pane label="同步日志" name="logs">
        <div class="toolbar">
          <el-select v-model="logChannelId" placeholder="选择渠道" style="width: 200px" @change="loadLogs" clearable>
            <el-option v-for="c in channels" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
          <el-button @click="loadLogs">刷新</el-button>
        </div>
        <el-table :data="logs" border stripe v-loading="loading">
          <el-table-column label="渠道" prop="channel_name" width="140" />
          <el-table-column label="同步类型" width="120">
            <template #default="{ row }">
              <el-tag size="small">{{ syncTypeLabel(row.sync_type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small">{{ row.status === 'success' ? '成功' : '失败' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="错误信息" prop="error_msg" min-width="200" />
          <el-table-column label="时间" prop="created_at" width="170" />
        </el-table>
        <el-pagination
          v-if="logTotal > 50"
          layout="prev, pager, next"
          :total="logTotal"
          :page-size="50"
          :current-page="logPage"
          @current-change="onLogPageChange"
          style="margin-top: 16px; justify-content: center"
        />
      </el-tab-pane>

      <!-- 渠道配额（超卖防护） -->
      <el-tab-pane label="渠道配额" name="quotas">
        <div class="toolbar">
          <el-select v-model="quotaStoreId" placeholder="选择门店" clearable style="width: 180px" @change="loadQuotas">
            <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <el-button type="primary" @click="openQuotaDialog()">设置配额</el-button>
          <el-button @click="loadQuotas">刷新</el-button>
          <span class="tip">为每个渠道的房型分配可售配额，防止多渠道超卖</span>
        </div>
        <el-table :data="quotas" border stripe v-loading="loading">
          <el-table-column label="门店" prop="store_name" width="140" />
          <el-table-column label="渠道" prop="channel_name" width="140" />
          <el-table-column label="房型" prop="room_type_name" width="120" />
          <el-table-column label="配额" prop="quota" width="80" align="center" />
          <el-table-column label="已用" prop="used" width="80" align="center" />
          <el-table-column label="剩余" width="80" align="center">
            <template #default="{ row }">
              <span :style="{ color: row.quota - row.used > 0 ? '#67c23a' : '#f56c6c', fontWeight: 600 }">
                {{ row.quota - row.used }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button size="small" @click="openQuotaDialog(row)">编辑</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- 推送明细日志 -->
      <el-tab-pane label="推送日志" name="pushLogs">
        <div class="toolbar">
          <el-select v-model="pushLogStoreId" placeholder="选择门店" clearable style="width: 180px" @change="loadPushLogs">
            <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <el-select v-model="pushLogType" placeholder="全部类型" clearable style="width: 120px" @change="loadPushLogs">
            <el-option label="库存" value="inventory" />
            <el-option label="价格" value="rate" />
          </el-select>
          <el-button @click="loadPushLogs">刷新</el-button>
          <el-button type="success" @click="manualPush" :loading="pushing">手动全量推送</el-button>
        </div>
        <el-table :data="pushLogs" border stripe v-loading="loading" size="small">
          <el-table-column label="门店" prop="store_name" width="130" />
          <el-table-column label="渠道" prop="channel_name" width="120" />
          <el-table-column label="房型" prop="room_type_name" width="110" />
          <el-table-column label="类型" width="70">
            <template #default="{ row }">
              <el-tag size="small" :type="row.push_type === 'inventory' ? '' : 'warning'">
                {{ row.push_type === 'inventory' ? '库存' : '价格' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="动作" width="80">
            <template #default="{ row }">
              <el-tag size="small" :type="actionTagType(row.action)">{{ actionLabel(row.action) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="70" align="center">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === 'success' ? 'success' : 'danger'">
                {{ row.status === 'success' ? '成功' : '失败' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="推送内容" prop="payload" min-width="200" show-overflow-tooltip />
          <el-table-column label="时间" prop="created_at" width="160" />
        </el-table>
      </el-tab-pane>

      <!-- OTA 订单 -->
      <el-tab-pane label="OTA订单" name="orders">
        <div class="toolbar">
          <el-select v-model="orderStoreId" placeholder="选择门店" clearable style="width: 180px" @change="loadOrders">
            <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <el-select v-model="orderStatus" placeholder="全部状态" clearable style="width: 120px" @change="loadOrders">
            <el-option label="待处理" :value="0" />
            <el-option label="已转预订" :value="1" />
            <el-option label="已取消" :value="2" />
          </el-select>
          <el-button type="success" @click="pullOrders" :loading="pulling">模拟拉取订单</el-button>
          <el-button @click="loadOrders">刷新</el-button>
        </div>
        <el-table :data="orders" border stripe v-loading="loading" size="small">
          <el-table-column label="OTA订单号" prop="ota_order_no" width="160" />
          <el-table-column label="渠道" prop="channel_name" width="110" />
          <el-table-column label="客人" prop="customer_name" width="90" />
          <el-table-column label="电话" prop="customer_phone" width="120" />
          <el-table-column label="入住" width="100">
            <template #default="{ row }">{{ fmtDate(row.check_in_date) }}</template>
          </el-table-column>
          <el-table-column label="离店" width="100">
            <template #default="{ row }">{{ fmtDate(row.check_out_date) }}</template>
          </el-table-column>
          <el-table-column label="房型" prop="room_type_name" width="100" />
          <el-table-column label="金额" width="80">
            <template #default="{ row }">¥{{ Number(row.price).toFixed(0) }}</template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="orderStatusType(row.status)">{{ orderStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="来源" width="80">
            <template #default="{ row }">{{ sourceLabel(row.source) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="140" fixed="right">
            <template #default="{ row }">
              <el-button v-if="row.status === 0" link type="success" @click="confirmOrder(row)">接单</el-button>
              <el-button v-if="row.status === 0 || row.status === 1" link type="danger" @click="rejectOrder(row)">拒单</el-button>
              <span v-if="row.status === 1" style="font-size: 12px; color: #67c23a">预订#{{ row.reservation_id }}</span>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <!-- 渠道弹窗 -->
    <el-dialog v-model="channelVisible" :title="channelForm.id ? '编辑渠道' : '新增渠道'" width="500px" destroy-on-close>
      <el-form label-width="90px" :model="channelForm">
        <el-form-item label="门店" required>
          <el-select v-model="channelForm.store_id" style="width: 100%" :disabled="!!channelForm.id">
            <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="渠道名称" required>
          <el-input v-model="channelForm.name" placeholder="如：美团·华庭总店" />
        </el-form-item>
        <el-form-item label="渠道编码" required>
          <el-select v-model="channelForm.channel_code" style="width: 100%" :disabled="!!channelForm.id">
            <el-option label="美团 (meituan)" value="meituan" />
            <el-option label="同程旅行 (tongcheng)" value="tongcheng" />
            <el-option label="携程 (ctrip)" value="ctrip" />
            <el-option label="飞猪 (fliggy)" value="fliggy" />
          </el-select>
        </el-form-item>
        <el-form-item label="API 地址">
          <el-input v-model="channelForm.api_url" placeholder="OTA 平台提供的 API 接口地址" />
        </el-form-item>
        <el-form-item label="App Key">
          <el-input v-model="channelForm.app_key" placeholder="OTA 平台分配的 Key" />
        </el-form-item>
        <el-form-item label="App Secret">
          <el-input v-model="channelForm.app_secret" type="password" placeholder="OTA 平台分配的 Secret" show-password />
        </el-form-item>
        <el-form-item label="酒店 ID">
          <el-input v-model="channelForm.hotel_id" placeholder="OTA 平台上的酒店 ID" />
        </el-form-item>
        <el-form-item label="回调地址">
          <el-input v-model="channelForm.callback_url" placeholder="接收 OTA 订单推送的回调 URL" />
        </el-form-item>
        <el-form-item v-if="channelForm.id" label="状态">
          <el-switch v-model="channelForm.status" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="channelVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitChannel">保存</el-button>
      </template>
    </el-dialog>

    <!-- 房型映射弹窗 -->
    <el-dialog v-model="mappingVisible" :title="mappingForm.id ? '编辑映射' : '新增映射'" width="480px" destroy-on-close>
      <el-form label-width="100px" :model="mappingForm">
        <el-form-item label="OTA 渠道" required>
          <el-select v-model="mappingForm.channel_id" style="width: 100%" :disabled="!!mappingForm.id">
            <el-option v-for="c in channels" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="PMS 房型" required>
          <el-select v-model="mappingForm.room_type_id" style="width: 100%" :disabled="!!mappingForm.id">
            <el-option v-for="r in roomTypes" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="OTA 房型 ID" required>
          <el-input v-model="mappingForm.ota_room_type_id" placeholder="OTA 平台上的房型 ID，如 1001" />
        </el-form-item>
        <el-form-item label="OTA 房型名称">
          <el-input v-model="mappingForm.ota_room_name" placeholder="OTA 上显示的名称，如「豪华大床房」" />
        </el-form-item>
        <el-form-item label="价格系数">
          <el-input-number v-model="mappingForm.price_factor" :min="0.1" :max="5" :step="0.05" :precision="3" style="width: 100%" />
          <span class="form-tip">如 0.9 表示 OTA 价 = PMS 价 × 0.9</span>
        </el-form-item>
        <el-form-item label="自动同步">
          <el-switch v-model="mappingForm.auto_sync" active-text="是" inactive-text="否" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="mappingVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitMapping">保存</el-button>
      </template>
    </el-dialog>

    <!-- 房态预览弹窗 -->
    <el-dialog v-model="inventoryVisible" title="待同步房态预览" width="700px" destroy-on-close>
      <el-table :data="inventory" border stripe>
        <el-table-column label="PMS 房型" prop="room_type_name" width="140" />
        <el-table-column label="OTA 房型 ID" prop="ota_room_type_id" width="150" />
        <el-table-column label="OTA 房型名称" prop="ota_room_name" width="150" />
        <el-table-column label="价格系数" width="90" align="center">
          <template #default="{ row }">{{ (row.price_factor * 100).toFixed(0) }}%</template>
        </el-table-column>
        <el-table-column label="可用房数" width="90" align="center">
          <template #default="{ row }">
            <span :style="{ color: row.available > 0 ? '#67c23a' : '#f56c6c', fontWeight: 'bold' }">{{ row.available }}</span>
            <span style="color: #999"> / {{ row.total }}</span>
          </template>
        </el-table-column>
        <el-table-column label="推送状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.sync_status === 'open' ? 'success' : 'danger'" size="small">
              {{ row.sync_status === 'open' ? '可售' : '售罄' }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="inventoryVisible = false">关闭</el-button>
        <el-button type="primary" @click="inventoryVisible = false; syncChannel({ id: filterChannelId })">立即同步</el-button>
      </template>
    </el-dialog>

    <!-- 配额弹窗 -->
    <el-dialog v-model="quotaVisible" title="设置渠道配额" width="460px" destroy-on-close>
      <el-alert title="配额用于超卖防护：该渠道该房型最多可售此数量。OTA下单时原子扣减，配额不足自动拒单。" type="info" :closable="false" show-icon style="margin-bottom: 16px" />
      <el-form label-width="90px" :model="quotaForm">
        <el-form-item label="门店" required>
          <el-select v-model="quotaForm.store_id" style="width: 100%" :disabled="!!quotaForm.id" @change="onQuotaStoreChange">
            <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="渠道" required>
          <el-select v-model="quotaForm.channel_id" style="width: 100%" :disabled="!!quotaForm.id" @change="onQuotaChannelChange">
            <el-option v-for="c in quotaChannels" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="房型" required>
          <el-select v-model="quotaForm.room_type_id" style="width: 100%" :disabled="!!quotaForm.id">
            <el-option v-for="r in quotaRoomTypes" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="配额数量" required>
          <el-input-number v-model="quotaForm.quota" :min="0" :max="50" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="quotaVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitQuota">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'

const activeTab = ref('channels')
const loading = ref(false)
const submitting = ref(false)

// 渠道管理
const stores = ref([])
const channels = ref([])
const channelVisible = ref(false)
const channelForm = reactive({ id: 0, store_id: null, name: '', channel_code: '', api_url: '', app_key: '', app_secret: '', hotel_id: '', callback_url: '', status: 1 })

// 房型映射
const filterChannelId = ref(null)
const roomTypes = ref([])
const mappings = ref([])
const mappingVisible = ref(false)
const mappingForm = reactive({ id: 0, channel_id: null, room_type_id: null, ota_room_type_id: '', ota_room_name: '', price_factor: 1.0, auto_sync: true })

// 同步日志
const logChannelId = ref(null)
const logs = ref([])
const logTotal = ref(0)
const logPage = ref(1)

// 房态预览
const inventoryVisible = ref(false)
const inventory = ref([])

function channelTag(code) {
  const map = { meituan: 'warning', tongcheng: 'success', ctrip: '', fliggy: 'danger' }
  return map[code] || 'info'
}
function syncTypeLabel(type) {
  const map = { inventory: '房态', rate: '房价', order: '订单' }
  return map[type] || type
}

async function loadStores() {
  try {
    const r = await api.listStores()
    stores.value = r.stores || []
  } catch {}
}

async function loadChannels() {
  loading.value = true
  try {
    const r = await api.listOtaChannels()
    channels.value = r.channels || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function loadRoomTypes() {
  try {
    const r = await api.listRoomTypes()
    roomTypes.value = r.room_types || []
  } catch {}
}

function openChannelDialog(row) {
  if (row) {
    Object.assign(channelForm, {
      id: row.id, store_id: row.store_id, name: row.name, channel_code: row.channel_code,
      api_url: row.api_url || '', app_key: '', app_secret: '', hotel_id: row.hotel_id || '',
      callback_url: '', status: row.status
    })
  } else {
    Object.assign(channelForm, { id: 0, store_id: null, name: '', channel_code: '', api_url: '', app_key: '', app_secret: '', hotel_id: '', callback_url: '', status: 1 })
  }
  channelVisible.value = true
}

async function submitChannel() {
  if (!channelForm.store_id || !channelForm.name || !channelForm.channel_code) {
    ElMessage.warning('请填写门店、渠道名称和渠道编码')
    return
  }
  submitting.value = true
  try {
    if (channelForm.id) {
      await api.updateOtaChannel(channelForm.id, channelForm)
      ElMessage.success('渠道更新成功')
    } else {
      await api.createOtaChannel(channelForm)
      ElMessage.success('渠道创建成功')
    }
    channelVisible.value = false
    loadChannels()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function deleteChannel(row) {
  try {
    await ElMessageBox.confirm(`确定删除渠道「${row.name}」吗？关联的映射和日志将一并删除。`, '确认删除', { type: 'warning' })
    await api.deleteOtaChannel(row.id)
    ElMessage.success('已删除')
    loadChannels()
  } catch {}
}

async function syncChannel(row) {
  loading.value = true
  try {
    const r = await api.syncOtaChannel(row.id)
    ElMessage.success(`同步成功：${r.synced_rooms} 个房型已推送到 ${r.channel}（${r.mode === 'simulated' ? '模拟模式' : '正式模式'}）`)
    loadChannels()
    loadLogs()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

// 映射管理
async function loadMappings() {
  loading.value = true
  try {
    const r = await api.listOtaMappings(filterChannelId.value)
    mappings.value = r.mappings || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function openMappingDialog(row) {
  if (row) {
    Object.assign(mappingForm, {
      id: row.id, channel_id: row.channel_id, room_type_id: row.room_type_id,
      ota_room_type_id: row.ota_room_type_id, ota_room_name: row.ota_room_name || '',
      price_factor: row.price_factor, auto_sync: row.auto_sync
    })
  } else {
    Object.assign(mappingForm, { id: 0, channel_id: filterChannelId.value, room_type_id: null, ota_room_type_id: '', ota_room_name: '', price_factor: 1.0, auto_sync: true })
  }
  mappingVisible.value = true
}

async function submitMapping() {
  if (!mappingForm.channel_id || !mappingForm.room_type_id || !mappingForm.ota_room_type_id) {
    ElMessage.warning('请填写渠道、PMS房型和OTA房型ID')
    return
  }
  submitting.value = true
  try {
    if (mappingForm.id) {
      await api.updateOtaMapping(mappingForm.id, mappingForm)
      ElMessage.success('映射更新成功')
    } else {
      await api.createOtaMapping(mappingForm)
      ElMessage.success('映射创建成功')
    }
    mappingVisible.value = false
    loadMappings()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function deleteMapping(row) {
  try {
    await ElMessageBox.confirm('确定删除该映射吗？', '确认删除', { type: 'warning' })
    await api.deleteOtaMapping(row.id)
    ElMessage.success('已删除')
    loadMappings()
  } catch {}
}

// 房态预览
async function previewInventory() {
  loading.value = true
  try {
    const r = await api.otaInventoryPreview(filterChannelId.value)
    inventory.value = r.rooms || []
    inventoryVisible.value = true
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

// 同步日志
async function loadLogs() {
  loading.value = true
  try {
    const params = { page: logPage.value, page_size: 50 }
    if (logChannelId.value) params.channel_id = logChannelId.value
    const r = await api.listOtaSyncLogs(params)
    logs.value = r.logs || []
    logTotal.value = r.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function onLogPageChange(p) {
  logPage.value = p
  loadLogs()
}

// ==================== 渠道配额（超卖防护） ====================
const quotaStoreId = ref(null)
const quotas = ref([])
const quotaVisible = ref(false)
const quotaForm = reactive({ id: 0, store_id: null, channel_id: null, room_type_id: null, quota: 0 })
const quotaChannels = ref([])
const quotaRoomTypes = ref([])

async function loadQuotas() {
  loading.value = true
  try {
    const params = {}
    if (quotaStoreId.value) params.store_id = quotaStoreId.value
    const r = await api.listOtaQuotas(params)
    quotas.value = r.quotas || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function openQuotaDialog(row) {
  if (row) {
    quotaForm.id = row.id
    quotaForm.store_id = row.store_id
    quotaForm.channel_id = row.channel_id
    quotaForm.room_type_id = row.room_type_id
    quotaForm.quota = row.quota
  } else {
    quotaForm.id = 0
    quotaForm.store_id = quotaStoreId.value || null
    quotaForm.channel_id = null
    quotaForm.room_type_id = null
    quotaForm.quota = 0
  }
  onQuotaStoreChange(quotaForm.store_id)
  quotaVisible.value = true
}

async function onQuotaStoreChange(sid) {
  quotaChannels.value = []
  quotaRoomTypes.value = []
  if (!sid) return
  try {
    const r = await api.listOtaChannels(sid)
    quotaChannels.value = r.channels || []
  } catch {}
  try {
    const r = await api.listRoomTypes(sid)
    quotaRoomTypes.value = r.room_types || []
  } catch {}
}

function onQuotaChannelChange() {
  // 仅供未来扩展
}

async function submitQuota() {
  if (!quotaForm.store_id || !quotaForm.channel_id || !quotaForm.room_type_id) {
    ElMessage.warning('门店/渠道/房型不能为空')
    return
  }
  submitting.value = true
  try {
    await api.upsertOtaQuota(quotaForm)
    ElMessage.success('配额已保存，已触发库存推送')
    quotaVisible.value = false
    loadQuotas()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

// ==================== 推送明细日志 ====================
const pushLogStoreId = ref(null)
const pushLogType = ref(null)
const pushLogs = ref([])
const pushing = ref(false)

async function loadPushLogs() {
  loading.value = true
  try {
    const params = {}
    if (pushLogStoreId.value) params.store_id = pushLogStoreId.value
    if (pushLogType.value) params.push_type = pushLogType.value
    const r = await api.listOtaPushLogs(params)
    pushLogs.value = r.logs || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function manualPush() {
  if (!pushLogStoreId.value) {
    ElMessage.warning('请先选择门店')
    return
  }
  pushing.value = true
  try {
    const r = await api.manualPushInventory(pushLogStoreId.value)
    ElMessage.success(`已推送 ${r.pushed_room_types} 个房型`)
    loadPushLogs()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    pushing.value = false
  }
}

function actionLabel(a) {
  return { open: '开房', close: '关房', update: '更新' }[a] || a
}
function actionTagType(a) {
  return { open: 'success', close: 'danger', update: 'warning' }[a] || ''
}

// ==================== OTA 订单 ====================
const orderStoreId = ref(null)
const orderStatus = ref(null)
const orders = ref([])
const pulling = ref(false)

async function loadOrders() {
  loading.value = true
  try {
    const params = {}
    if (orderStoreId.value) params.store_id = orderStoreId.value
    if (orderStatus.value !== null && orderStatus.value !== '') params.status = orderStatus.value
    const r = await api.listOtaOrders(params)
    orders.value = r.orders || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function pullOrders() {
  if (!orderStoreId.value) {
    ElMessage.warning('请先选择门店')
    return
  }
  pulling.value = true
  try {
    const r = await api.pullOtaOrders(orderStoreId.value)
    ElMessage.success(`模拟拉取 ${r.pulled} 个订单`)
    loadOrders()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    pulling.value = false
  }
}

async function confirmOrder(row) {
  try {
    await ElMessageBox.confirm(`确认接单？将自动创建 PMS 预订并扣减配额。`, '接单确认', { type: 'warning' })
    const r = await api.confirmOtaOrder(row.id)
    ElMessage.success(`已接单，预订 #${r.reservation_id}`)
    loadOrders()
  } catch (e) {
    if (e !== 'cancel' && e.message) ElMessage.error(e.message)
  }
}

async function rejectOrder(row) {
  try {
    await ElMessageBox.confirm(`确认拒单？${row.status === 1 ? '已确认的订单拒单会释放配额。' : ''}`, '拒单确认', { type: 'warning' })
    await api.rejectOtaOrder(row.id)
    ElMessage.success('已拒单')
    loadOrders()
  } catch (e) {
    if (e !== 'cancel' && e.message) ElMessage.error(e.message)
  }
}

function fmtDate(d) {
  if (!d) return ''
  return new Date(d).toISOString().slice(0, 10)
}
function orderStatusLabel(s) {
  return ['待处理', '已转预订', '已取消', '已入住'][s] || s
}
function orderStatusType(s) {
  return ['warning', 'success', 'info', 'success'][s] || ''
}
function sourceLabel(s) {
  return { callback: '回调', pull: '拉取', manual: '手动' }[s] || s
}

// tab 切换时懒加载
function onTabChange(tab) {
  const name = tab.paneName
  if (name === 'quotas') loadQuotas()
  else if (name === 'pushLogs') loadPushLogs()
  else if (name === 'orders') loadOrders()
}

onMounted(() => {
  loadStores()
  loadChannels()
  loadRoomTypes()
  loadLogs()
})
</script>

<style scoped>
.ota-config {
  max-width: 1200px;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.tip {
  color: #999;
  font-size: 13px;
  margin-left: 8px;
}
.form-tip {
  color: #999;
  font-size: 12px;
  margin-left: 8px;
  line-height: 32px;
}
</style>