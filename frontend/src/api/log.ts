import http from './http'
import type { LogResult, AnalyzeResult } from './types'

export interface LogParams {
  type: 'list' | 'log'
  level?: 'all' | 'info' | 'warn' | 'error'
  keyword?: string
  project: string
  survey: string
}

export function getLogs(jobName: string, params: LogParams): Promise<LogResult> {
  return http.get(`/jobs/${encodeURIComponent(jobName)}/logs`, { params })
}

export function analyzeLogs(
  jobName: string,
  body: { type: 'list' | 'log'; project: string; survey: string; keyword?: string },
): Promise<AnalyzeResult> {
  return http.post(`/jobs/${encodeURIComponent(jobName)}/logs/analyze`, body)
}
