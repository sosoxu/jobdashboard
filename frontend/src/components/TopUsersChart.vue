<template>
  <v-chart class="chart" :option="option" autoresize />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import type { TopUsersResult } from '@/api/types'

use([CanvasRenderer, BarChart, GridComponent, TooltipComponent, LegendComponent])

const props = defineProps<{ data: TopUsersResult | null }>()

const option = computed(() => {
  const users = props.data?.users ?? []
  const others = props.data?.others
  const names = users.map((u) => u.userName)
  const counts = users.map((u) => u.count)
  if (others) {
    names.push(others.userName)
    counts.push(others.count)
  }
  // horizontal bar: reverse so Top1 is on top
  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params: any[]) => {
        const p = params[0]
        const name = p.name
        const count = p.value
        const total = props.data?.total ?? 0
        const pct = total > 0 ? ((count / total) * 100).toFixed(1) : '0'
        return `${name}<br/>作业数: ${count} (${pct}%)`
      },
    },
    grid: { left: 80, right: 30, top: 10, bottom: 20 },
    xAxis: { type: 'value', minInterval: 1 },
    yAxis: {
      type: 'category',
      data: names.slice().reverse(),
      axisLabel: { fontSize: 11 },
    },
    series: [
      {
        type: 'bar',
        data: counts.slice().reverse(),
        itemStyle: { color: '#409eff', borderRadius: [0, 4, 4, 0] },
        barMaxWidth: 22,
      },
    ],
  }
})
</script>

<style scoped>
.chart {
  height: 320px;
  width: 100%;
}
</style>
