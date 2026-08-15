<template>
  <div class="home">
    <el-card shadow="never">
      <template #header>
        <span>脚手架已就绪</span>
      </template>
      <p>前端 Vue 3 + Element Plus + Vite 运行正常。</p>
      <p>后端 Go 服务请访问 <code>/health</code> 验证。</p>
      <el-divider />
      <p>第一期模块：预订 / 前台接待 / 房态 / 房价 / 客户 / 收银 / 会员 / 报表 / 权限。</p>
    </el-card>
  </div>
</template>

<script setup>
</script>

<style scoped>
.home {
  padding: 20px;
}
.home p {
  margin: 8px 0;
  color: #333;
}
</style>
