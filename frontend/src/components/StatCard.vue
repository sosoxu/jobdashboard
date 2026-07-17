<template>
  <div class="stat-card card" :class="`tone-${tone}`">
    <div class="stat-label">{{ stat.label }}</div>
    <div class="stat-count">{{ stat.count }}</div>
    <div class="stat-delta">
      <template v-if="stat.delta === 0">
        <span class="muted">较1分钟前 持平</span>
      </template>
      <template v-else>
        <span :class="stat.delta > 0 ? 'up' : 'down'">
          <el-icon><CaretTop v-if="stat.delta > 0" /><CaretBottom v-else /></el-icon>
          {{ stat.delta > 0 ? '+' : '' }}{{ stat.delta }}
          <span class="muted">({{ stat.deltaPct > 0 ? '+' : '' }}{{ stat.deltaPct.toFixed(1) }}%)</span>
        </span>
      </template>
      <span class="muted prev">前值 {{ stat.prevCount }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { CaretTop, CaretBottom } from '@element-plus/icons-vue'
import type { GroupStat } from '@/api/types'

const props = defineProps<{ stat: GroupStat }>()

const tone = computed(() => {
  switch (props.stat.key) {
    case 'active':
      return 'success'
    case 'failed':
      return 'danger'
    case 'queue':
      return 'primary'
    case 'finish':
      return 'info'
    case 'canceled':
      return 'warning'
    default:
      return 'other'
  }
})
</script>

<style scoped>
.stat-card {
  position: relative;
  overflow: hidden;
}
.stat-card::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 4px;
  background: #409eff;
}
.tone-success::before { background: #67c23a; }
.tone-danger::before { background: #f56c6c; }
.tone-primary::before { background: #409eff; }
.tone-info::before { background: #909399; }
.tone-warning::before { background: #e6a23c; }
.tone-other::before { background: #b0b3b8; }

.stat-label {
  font-size: 14px;
  color: #606266;
}
.stat-count {
  font-size: 32px;
  font-weight: 600;
  margin: 8px 0;
  color: #303133;
}
.stat-delta {
  font-size: 13px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.stat-delta .up { color: #f56c6c; display: inline-flex; align-items: center; gap: 2px; }
.stat-delta .down { color: #67c23a; display: inline-flex; align-items: center; gap: 2px; }
.stat-delta .prev { margin-left: auto; }
</style>
