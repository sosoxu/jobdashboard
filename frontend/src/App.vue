<template>
  <el-container class="app-container">
    <el-header v-if="!isPublicRoute" class="app-header">
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
      <div v-if="userStore.isLoggedIn" class="user-area">
        <el-icon><UserFilled /></el-icon>
        <span class="user-name">{{ userStore.username || '未命名' }}</span>
        <el-button text class="logout-btn" @click="onLogout">登出</el-button>
      </div>
    </el-header>
    <el-main class="app-main">
      <router-view />
    </el-main>
  </el-container>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Monitor, UserFilled } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const activeMenu = computed(() => {
  if (route.path.startsWith('/jobs')) return '/jobs'
  return '/'
})

const isPublicRoute = computed(() => route.meta.public === true)

async function onLogout() {
  userStore.logout()
  router.replace({ name: 'login' })
}

// On a fresh page load the token may exist in localStorage while the
// username is unknown; validate the session and populate the username.
onMounted(() => {
  if (userStore.isLoggedIn && !userStore.username) {
    userStore.fetchMe().catch(() => {
      // Token invalid: interceptor/store already cleared the session.
      router.replace({ name: 'login' })
    })
  }
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
.user-area {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #c9d1d9;
  font-size: 14px;
  white-space: nowrap;
}
.user-name {
  margin-right: 4px;
}
.logout-btn {
  color: #c9d1d9;
}
.logout-btn:hover {
  color: #fff;
}
.app-main {
  padding: 0;
  overflow: auto;
}
</style>
