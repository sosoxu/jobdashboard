<template>
  <div class="refresh-control">
    <el-select v-model="intervalVal" size="small" style="width: 110px" placeholder="自动刷新">
      <el-option
        v-for="opt in REFRESH_INTERVALS"
        :key="opt.value"
        :label="opt.label"
        :value="opt.value"
      />
    </el-select>
    <el-button size="small" :icon="Refresh" @click="emit('refresh')">刷新</el-button>
    <span v-if="lastUpdated" class="muted">
      更新于 {{ lastUpdated.toLocaleTimeString() }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { REFRESH_INTERVALS } from '@/composables/useAutoRefresh'

const props = defineProps<{
  interval: number
  lastUpdated: Date | null
}>()

const emit = defineEmits<{
  (e: 'update:interval', v: number): void
  (e: 'refresh'): void
}>()

const intervalVal = computed({
  get: () => props.interval,
  set: (v: number) => emit('update:interval', v),
})
</script>

<style scoped>
.refresh-control {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
