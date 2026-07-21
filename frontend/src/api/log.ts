import http from './http'
import type { LogResult, AnalyzeResult } from './types'

// project/survey 由 BFF 通过 jobName 在作业缓存中解析（jobName 全局唯一）。
export interface LogParams {
  type: 'list' | 'log'
  keyword?: string
  page?: number
  pageSize?: number
}

export function getLogs(jobName: string, params: LogParams): Promise<LogResult> {
  return http.get(`/jobs/${encodeURIComponent(jobName)}/logs`, { params })
}

export function analyzeLogs(
  jobName: string,
  body: {
    type: 'list' | 'log'
    keyword?: string
    page?: number
    pageSize?: number
  },
): Promise<AnalyzeResult> {
  return http.post(`/jobs/${encodeURIComponent(jobName)}/logs/analyze`, body)
}
