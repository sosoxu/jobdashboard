<template>
  <el-container class="app-container">
    <el-header class="app-header">
      <div class="app-title">
        <el-icon><Monitor /></el-icon>
        <span>作业监控 Dashboard</span>
      </div>
      <el-menu
        :default-active="activeMenu"
        mode="horizontal"
        :ellipsis="false"
        router
        class="app-menu"
      >
        <el-menu-item index="/">作业大盘</el-menu-item>
        <el-menu-item index="/jobs">作业详情</el-menu-item>
      </el-menu>
    </el-header>
    <el-main class="app-main">
      <router-view />
    </el-main>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Monitor } from '@element-plus/icons-vue'

const route = useRoute()
const activeMenu = computed(() => {
  if (route.path.startsWith('/jobs')) return '/jobs'
  return '/'
})
</script>

<style scoped>
.app-container {
  height: 100vh;
}
.app-header {
  display: flex;
  align-items: center;
  background: #001529;
  color: #fff;
  padding: 0 24px;
  height: 56px;
}
.app-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 18px;
  font-weight: 600;
  margin-right: 32px;
  white-space: nowrap;
}
.app-menu {
  background: transparent;
  border-bottom: none;
  flex: 1;
}
.app-menu :deep(.el-menu-item) {
  color: #c9d1d9;
}
.app-menu :deep(.el-menu-item.is-active) {
  color: #fff;
  background: transparent;
  border-bottom-color: #409eff;
}
.app-main {
  padding: 0;
  overflow: auto;
}
</style>
