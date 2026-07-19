import http from './http'
import type { LogResult, AnalyzeResult } from './types'

export interface LogParams {
  type: 'list' | 'log'
  keyword?: string
  project: string
  survey: string
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
    project: string
    survey: string
    keyword?: string
    page?: number
    pageSize?: number
  },
): Promise<AnalyzeResult> {
  return http.post(`/jobs/${encodeURIComponent(jobName)}/logs/analyze`, body)
}
