<template>
  <div class="page">
    <div class="flex-between mb-16">
      <div>
        <h2 class="page-title">作业监控大盘</h2>
        <span v-if="degraded" class="muted" style="color:#e6a23c">数据来自缓存（上游服务暂不可用）</span>
      </div>
      <RefreshControl
        v-model:interval="interval"
        :last-updated="lastUpdated"
        @refresh="refresh"
      />
    </div>

    <div class="stat-grid mb-16">
      <StatCard v-for="g in stats?.groups ?? []" :key="g.key" :stat="g" />
    </div>
    <div v-if="!stats" class="card muted">加载中…</div>

    <el-row :gutter="16">
      <el-col :xs="24" :md="15">
        <div class="card">
          <div class="flex-between mb-16">
            <h3 class="section-title">作业趋势</h3>
            <el-radio-group v-model="trendRange" size="small" @change="loadTrend">
              <el-radio-button label="24h">24小时</el-radio-button>
              <el-radio-button label="7d">7天</el-radio-button>
              <el-radio-button label="30d">30天</el-radio-button>
            </el-radio-group>
          </div>
          <TrendChart :points="trend?.points ?? []" />
        </div>
      </el-col>
      <el-col :xs="24" :md="9">
        <div class="card">
          <h3 class="section-title mb-16">作业量用户 Top10</h3>
          <TopUsersChart :data="topUsers" />
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import StatCard from '@/components/StatCard.vue'
import TrendChart from '@/components/TrendChart.vue'
import TopUsersChart from '@/components/TopUsersChart.vue'
import RefreshControl from '@/components/RefreshControl.vue'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { getStats, getTrend, getTopUsers } from '@/api/dashboard'
import type { StatsResult, TrendResult, TopUsersResult } from '@/api/types'

const stats = ref<StatsResult | null>(null)
const trend = ref<TrendResult | null>(null)
const topUsers = ref<TopUsersResult | null>(null)
const trendRange = ref<'24h' | '7d' | '30d'>('24h')

const degraded = ref(false)

async function loadStats() {
  try {
    const r = await getStats(false)
    stats.value = r
    degraded.value = r.degraded
  } catch (e) {
    console.error(e)
  }
}
async function loadTrend() {
  try {
    trend.value = await getTrend(trendRange.value)
  } catch (e) {
    console.error(e)
  }
}
async function loadTop() {
  try {
    topUsers.value = await getTopUsers(10)
  } catch (e) {
    console.error(e)
  }
}

const fetchAll = async () => {
  await Promise.all([loadStats(), loadTrend(), loadTop()])
}

const { interval, lastUpdated, refresh } = useAutoRefresh(fetchAll, { defaultInterval: 60 })
</script>

<style scoped>
.page-title {
  margin: 0;
  font-size: 18px;
}
.section-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 16px;
}
</style>
