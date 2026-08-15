<template>
  <div class="member">
    <div class="toolbar">
      <el-input v-model="keyword" placeholder="搜索姓名/手机号/会员号" style="width: 260px" clearable @keyup.enter="load" />
      <el-button type="primary" @click="load">搜索</el-button>
    </div>

    <el-table :data="list" border stripe>
      <el-table-column prop="member_no" label="会员号" width="120" />
      <el-table-column prop="name" label="姓名" width="100" />
      <el-table-column prop="phone" label="手机号" width="135" />
      <el-table-column label="等级" width="100">
        <template #default="{ row }">
          <el-tag :type="['info', 'success', 'warning'][row.level] || 'info'">
            {{ ['普通', '银卡', '金卡'][row.level] || '普通' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="points" label="积分" width="90" />
      <el-table-column label="储值余额" width="120">
        <template #default="{ row }">¥{{ Number(row.balance).toFixed(2) }}</template>
      </el-table-column>
      <el-table-column prop="join_date" label="入会日期" width="115" />
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="primary" link @click="openRecharge(row)">储值</el-button>
          <el-button size="small" type="warning" link @click="openPoints(row)">积分</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="rechargeVisible" title="会员储值充值" width="420px" destroy-on-close>
      <el-form label-width="90px">
        <el-form-item label="会员">
          <span>{{ current.name }}（{{ current.member_no }}）</span>
        </el-form-item>
        <el-form-item label="当前余额">
          <span>¥{{ Number(current.balance).toFixed(2) }}</span>
        </el-form-item>
        <el-form-item label="充值金额" required>
          <el-input-number v-model="rechargeAmount" :min="1" :precision="2" :step="100" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rechargeVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitRecharge">确认充值</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="pointsVisible" title="会员积分调整" width="420px" destroy-on-close>
      <el-form label-width="90px">
        <el-form-item label="会员">
          <span>{{ current.name }}（{{ current.member_no }}）</span>
        </el-form-item>
        <el-form-item label="当前积分">
          <span>{{ current.points }}</span>
        </el-form-item>
        <el-form-item label="调整分值" required>
          <el-input-number v-model="pointsDelta" :step="100" style="width: 100%" />
          <div class="hint">正数增加积分，负数扣减积分</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pointsVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitPoints">确认调整</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const list = ref([])
const keyword = ref('')
const rechargeVisible = ref(false)
const pointsVisible = ref(false)
const submitting = ref(false)
const current = ref({})
const rechargeAmount = ref(0)
const pointsDelta = ref(0)

async function load() {
  try {
    const d = await api.listMembers(keyword.value)
    list.value = d.members || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openRecharge(row) {
  current.value = row
  rechargeAmount.value = 0
  rechargeVisible.value = true
}

function openPoints(row) {
  current.value = row
  pointsDelta.value = 0
  pointsVisible.value = true
}

async function submitRecharge() {
  if (!rechargeAmount.value || rechargeAmount.value <= 0) {
    ElMessage.warning('请输入充值金额')
    return
  }
  submitting.value = true
  try {
    await api.rechargeMember(current.value.id, rechargeAmount.value)
    ElMessage.success('储值成功')
    rechargeVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function submitPoints() {
  if (!pointsDelta.value) {
    ElMessage.warning('请输入调整分值')
    return
  }
  submitting.value = true
  try {
    await api.adjustMemberPoints(current.value.id, pointsDelta.value)
    ElMessage.success('积分已调整')
    pointsVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}
.hint {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
</style>
