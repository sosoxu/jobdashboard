import http from './http'
import type { StatsResult, TrendResult, TopUsersResult } from './types'

export function getStats(fresh = false): Promise<StatsResult> {
  return http.get('/dashboard/stats', { params: { fresh: fresh ? 1 : 0 } })
}

export function getTrend(range: '24h' | '7d' | '30d'): Promise<TrendResult> {
  return http.get('/dashboard/trend', { params: { range } })
}

export function getTopUsers(limit = 10): Promise<TopUsersResult> {
  return http.get('/dashboard/top-users', { params: { limit } })
}
