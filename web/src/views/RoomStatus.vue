<template>
  <div class="room-status">
    <el-card shadow="never" class="toolbar-card">
      <div class="toolbar">
        <div class="legend">
          <span class="legend-item"><i class="dot dot-0"></i>空净</span>
          <span class="legend-item"><i class="dot dot-1"></i>空脏</span>
          <span class="legend-item"><i class="dot dot-2"></i>住客</span>
          <span class="legend-item"><i class="dot dot-3"></i>维修</span>
          <span class="legend-item"><i class="dot dot-4"></i>预留</span>
        </div>
        <div class="actions">
          <el-button size="small" @click="loadRooms">刷新</el-button>
        </div>
      </div>
    </el-card>

    <div v-for="group in floorGroups" :key="group.floor" class="floor-group">
      <div class="floor-title">{{ group.floor }} 层</div>
      <div class="room-grid">
        <div
          v-for="room in group.rooms"
          :key="room.id"
          class="room-card"
          :class="'status-' + room.status"
          @click="onRoomClick(room)"
        >
          <div class="room-no">{{ room.room_no }}</div>
          <div class="room-type">{{ room.room_type_name }}</div>
          <div class="room-status-text">{{ statusText(room.status) }}</div>
        </div>
      </div>
    </div>

    <el-empty v-if="!loading && rooms.length === 0" description="暂无房间数据" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'

const rooms = ref([])
const loading = ref(false)

const statusMap = {
  0: '空净',
  1: '空脏',
  2: '住客',
  3: '维修',
  4: '预留'
}

function statusText(s) {
  return statusMap[s] || '未知'
}

const floorGroups = computed(() => {
  const map = new Map()
  for (const r of rooms.value) {
    const f = r.floor || '未分层'
    if (!map.has(f)) map.set(f, [])
    map.get(f).push(r)
  }
  return Array.from(map, ([floor, list]) => ({ floor, rooms: list }))
})

async function loadRooms() {
  loading.value = true
  try {
    const res = await fetch('/api/v1/rooms')
    const data = await res.json()
    if (!res.ok) {
      ElMessage.error(data.error || '加载失败')
      return
    }
    rooms.value = data.rooms || []
  } catch (e) {
    ElMessage.error('网络错误，无法加载房态')
  } finally {
    loading.value = false
  }
}

function onRoomClick(room) {
  ElMessage.info(`${room.room_no}（${room.room_type_name}）：${statusText(room.status)}`)
}

onMounted(loadRooms)
</script>

<style scoped>
.room-status {
  padding: 4px;
}
.toolbar-card {
  margin-bottom: 16px;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.legend {
  display: flex;
  gap: 18px;
  flex-wrap: wrap;
}
.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #606266;
}
.dot {
  width: 12px;
  height: 12px;
  border-radius: 3px;
  display: inline-block;
}
.dot-0 { background: #67c23a; }
.dot-1 { background: #e6a23c; }
.dot-2 { background: #409eff; }
.dot-3 { background: #909399; }
.dot-4 { background: #9b59b6; }

.floor-group {
  margin-bottom: 20px;
}
.floor-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 10px;
  padding-left: 4px;
  border-left: 3px solid #409eff;
}
.room-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 12px;
}
.room-card {
  border-radius: 6px;
  padding: 14px 10px;
  text-align: center;
  color: #fff;
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
}
.room-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.18);
}
.room-no {
  font-size: 22px;
  font-weight: 700;
  line-height: 1.2;
}
.room-type {
  font-size: 12px;
  margin-top: 4px;
  opacity: 0.9;
}
.room-status-text {
  font-size: 12px;
  margin-top: 4px;
  font-weight: 600;
}

.status-0 { background: #67c23a; }
.status-1 { background: #e6a23c; }
.status-2 { background: #409eff; }
.status-3 { background: #909399; }
.status-4 { background: #9b59b6; }
</style>
