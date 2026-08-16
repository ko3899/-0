<template>
  <div class="users">
    <div class="toolbar">
      <div class="toolbar-title">用户与权限管理</div>
      <el-button type="primary" @click="openCreate">新增用户</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table :data="list" border stripe>
      <el-table-column prop="username" label="用户名" width="130" />
      <el-table-column prop="name" label="姓名" width="120" />
      <el-table-column label="角色" width="130">
        <template #default="{ row }">
          <el-tag :type="row.role_name === '集团管理员' ? 'danger' : row.role_name === '店长' ? 'warning' : 'info'">
            {{ row.role_name || '-' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
            {{ row.status === 1 ? '启用' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="数据权限门店" min-width="240">
        <template #default="{ row }">
          <template v-if="row.store_ids && row.store_ids.length">
            <el-tag v-for="sid in row.store_ids" :key="sid" size="small" class="store-tag" type="info">
              {{ storeMap[sid] || ('门店#' + sid) }}
            </el-tag>
          </template>
          <el-tag v-else size="small" type="warning">全部门店</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="90" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 新增/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑用户' : '新增用户'" width="520px" destroy-on-close>
      <el-form :model="form" label-width="90px">
        <el-form-item label="用户名" :required="!isEdit">
          <el-input v-model="form.username" :disabled="isEdit" placeholder="登录账号" />
        </el-form-item>
        <el-form-item label="密码" :required="!isEdit">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            :placeholder="isEdit ? '留空则不修改密码' : '登录密码'"
          />
        </el-form-item>
        <el-form-item label="姓名">
          <el-input v-model="form.name" placeholder="真实姓名" />
        </el-form-item>
        <el-form-item label="角色" required>
          <el-select v-model="form.role_id" placeholder="选择角色" style="width: 100%">
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="isEdit" label="状态">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="禁用" />
        </el-form-item>
        <el-form-item label="数据权限门店">
          <el-select v-model="form.store_ids" multiple placeholder="选择可访问的门店" style="width: 100%">
            <el-option v-for="s in stores" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <div class="form-tip">集团管理员默认可访问全部门店，无需勾选。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const list = ref([])
const roles = ref([])
const stores = ref([])
const storeMap = ref({})

const dialogVisible = ref(false)
const isEdit = ref(false)
const submitting = ref(false)
const form = ref({ id: 0, username: '', password: '', name: '', role_id: 0, status: 1, store_ids: [] })

async function load() {
  try {
    const [u, r, s] = await Promise.all([api.listUsers(), api.listRoles(), api.listStores()])
    list.value = u.users || []
    roles.value = r.roles || []
    stores.value = s.stores || []
    const m = {}
    stores.value.forEach((st) => { m[st.id] = st.name })
    storeMap.value = m
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openCreate() {
  isEdit.value = false
  form.value = { id: 0, username: '', password: '', name: '', role_id: 0, status: 1, store_ids: [] }
  dialogVisible.value = true
}

function openEdit(row) {
  isEdit.value = true
  form.value = {
    id: row.id,
    username: row.username,
    password: '',
    name: row.name,
    role_id: row.role_id,
    status: row.status,
    store_ids: [...(row.store_ids || [])]
  }
  dialogVisible.value = true
}

async function submit() {
  if (!isEdit.value && !form.value.username.trim()) {
    ElMessage.warning('请输入用户名')
    return
  }
  if (!isEdit.value && !form.value.password) {
    ElMessage.warning('请输入密码')
    return
  }
  if (!form.value.role_id) {
    ElMessage.warning('请选择角色')
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: form.value.name,
      role_id: form.value.role_id,
      store_ids: form.value.store_ids
    }
    if (isEdit.value) {
      payload.status = form.value.status
      if (form.value.password) payload.password = form.value.password
      await api.updateUser(form.value.id, payload)
      ElMessage.success('用户已更新')
    } else {
      payload.username = form.value.username.trim()
      payload.password = form.value.password
      await api.createUser(payload)
      ElMessage.success('用户已创建')
    }
    dialogVisible.value = false
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
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}
.toolbar-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-1, #303133);
  flex: 1;
}
.store-tag {
  margin: 2px 4px 2px 0;
}
.form-tip {
  font-size: 12px;
  color: #909399;
  line-height: 1.5;
  margin-top: 4px;
}
</style>
