<template>
  <div class="customer">
    <div class="toolbar">
      <el-input v-model="keyword" placeholder="搜索姓名/手机号" style="width: 240px" clearable @keyup.enter="load" />
      <el-button type="primary" @click="load">搜索</el-button>
      <el-button @click="openCreate">新建客户</el-button>
    </div>

    <el-table :data="list" border stripe>
      <el-table-column prop="name" label="姓名" width="110" />
      <el-table-column label="性别" width="70">
        <template #default="{ row }">{{ row.gender === 2 ? '女' : '男' }}</template>
      </el-table-column>
      <el-table-column prop="id_no" label="证件号" width="190" />
      <el-table-column prop="phone" label="手机号" width="135" />
      <el-table-column prop="member_no" label="会员号" width="115" />
      <el-table-column label="会员等级" width="100">
        <template #default="{ row }">
          <el-tag v-if="row.member_no" :type="['info', 'success', 'warning'][row.level] || 'info'">
            {{ ['普通', '银卡', '金卡'][row.level] || '普通' }}
          </el-tag>
          <span v-else style="color: #c0c4cc">-</span>
        </template>
      </el-table-column>
      <el-table-column prop="points" label="积分" width="90" />
      <el-table-column prop="tags" label="标签" />
    </el-table>

    <el-dialog v-model="createVisible" title="新建客户" width="480px" destroy-on-close>
      <el-form :model="form" label-width="80px">
        <el-form-item label="姓名" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="性别">
          <el-radio-group v-model="form.gender">
            <el-radio :value="1">男</el-radio>
            <el-radio :value="2">女</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="form.phone" />
        </el-form-item>
        <el-form-item label="证件号">
          <el-input v-model="form.id_no" />
        </el-form-item>
        <el-form-item label="标签">
          <el-input v-model="form.tags" placeholder="如 贵宾 / 黑名单" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitCreate">保存</el-button>
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
const createVisible = ref(false)
const submitting = ref(false)
const form = ref({ name: '', gender: 1, phone: '', id_no: '', tags: '' })

async function load() {
  try {
    const d = await api.listCustomers(keyword.value)
    list.value = d.customers || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openCreate() {
  form.value = { name: '', gender: 1, phone: '', id_no: '', tags: '' }
  createVisible.value = true
}

async function submitCreate() {
  if (!form.value.name.trim()) {
    ElMessage.warning('请输入姓名')
    return
  }
  submitting.value = true
  try {
    await api.createCustomer({ ...form.value, name: form.value.name.trim() })
    ElMessage.success('客户创建成功')
    createVisible.value = false
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
</style>
