<template>
  <v-chart class="chart" :option="option" autoresize />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, DataZoomComponent } from 'echarts/components'
import type { TrendPoint } from '@/api/types'
import { fmtTimeShort } from '@/utils/format'

use([CanvasRenderer, LineChart, GridComponent, TooltipComponent, LegendComponent, DataZoomComponent])

const props = defineProps<{ points: TrendPoint[] }>()

const option = computed(() => ({
  tooltip: {
    trigger: 'axis',
    formatter: (params: any[]) => {
      const ts = params[0]?.axisValue
      const lines = [ts]
      for (const p of params) {
        lines.push(`${p.marker} ${p.seriesName}: ${p.value}`)
      }
      return lines.join('<br/>')
    },
  },
  legend: { data: ['已完成', '运行中'], top: 0 },
  grid: { left: 40, right: 20, top: 40, bottom: 50 },
  dataZoom: [{ type: 'inside' }, { type: 'slider', height: 16, bottom: 8 }],
  xAxis: {
    type: 'category',
    boundaryGap: false,
    data: props.points.map((p) => fmtTimeShort(p.ts)),
    axisLabel: { fontSize: 10 },
  },
  yAxis: { type: 'value', minInterval: 1 },
  series: [
    {
      name: '已完成',
      type: 'line',
      smooth: true,
      showSymbol: false,
      itemStyle: { color: '#909399' },
      areaStyle: { opacity: 0.1 },
      data: props.points.map((p) => p.finish),
    },
    {
      name: '运行中',
      type: 'line',
      smooth: true,
      showSymbol: false,
      itemStyle: { color: '#67c23a' },
      areaStyle: { opacity: 0.15 },
      data: props.points.map((p) => p.active),
    },
  ],
}))
</script>

<style scoped>
.chart {
  height: 320px;
  width: 100%;
}
</style>
