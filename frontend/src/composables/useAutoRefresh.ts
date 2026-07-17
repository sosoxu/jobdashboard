import { ref, watch, onMounted, onUnmounted } from 'vue'

export interface AutoRefreshOptions {
  /** initial interval in seconds; 0 = off */
  defaultInterval?: number
}

/**
 * Provides a periodic auto-refresh mechanism. The fetcher is invoked on mount
 * and then on every tick of the chosen interval. Setting interval to 0 stops
 * the timer (manual refresh still available).
 */
export function useAutoRefresh(
  fetcher: () => Promise<void> | void,
  options: AutoRefreshOptions = {},
) {
  const interval = ref<number>(options.defaultInterval ?? 60)
  const lastUpdated = ref<Date | null>(null)
  let timer: number | null = null

  const stop = () => {
    if (timer !== null) {
      window.clearInterval(timer)
      timer = null
    }
  }
  const start = () => {
    stop()
    if (interval.value <= 0) return
    timer = window.setInterval(() => {
      void run()
    }, interval.value * 1000)
  }
  const run = async () => {
    try {
      await fetcher()
    } finally {
      lastUpdated.value = new Date()
    }
  }
  const refresh = async () => {
    await run()
  }

  watch(interval, start)

  onMounted(() => {
    void run()
    start()
  })
  onUnmounted(stop)

  return { interval, lastUpdated, refresh }
}

/** Available refresh intervals in seconds. */
export const REFRESH_INTERVALS = [
  { label: '关闭', value: 0 },
  { label: '10秒', value: 10 },
  { label: '1分钟', value: 60 },
  { label: '5分钟', value: 300 },
  { label: '10分钟', value: 600 },
  { label: '30分钟', value: 1800 },
]
