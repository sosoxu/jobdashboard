import http from './http'
import type { JobListResult, ControlResult, JobFilters } from './types'

export interface JobListParams {
  page?: number
  pageSize?: number
  /** multi-value, comma-joined by http */
  jobStatus?: number[]
  userName?: string[]
  project?: string[]
  survey?: string[]
  database?: string[]
  /** case-insensitive contains on jobDesc */
  jobDesc?: string
  commitTimeStart?: number
  commitTimeEnd?: number
}

function joinMulti(v?: string[]): string | undefined {
  if (!v || v.length === 0) return undefined
  return v.join(',')
}

export function getJobs(params: JobListParams): Promise<JobListResult> {
  return http.get('/jobs', {
    params: {
      page: params.page,
      pageSize: params.pageSize,
      jobStatus: joinMulti(params.jobStatus?.map((n) => String(n))),
      userName: joinMulti(params.userName),
      project: joinMulti(params.project),
      survey: joinMulti(params.survey),
      database: joinMulti(params.database),
      jobDesc: params.jobDesc || undefined,
      commitTimeStart: params.commitTimeStart,
      commitTimeEnd: params.commitTimeEnd,
    },
  })
}

export interface JobFilterParams {
  /** 选中数据库（用于级联收窄 project/survey 候选值） */
  database?: string[]
  /** 选中项目（用于级联收窄 survey 候选值） */
  project?: string[]
}

export function getJobFilters(params?: JobFilterParams): Promise<JobFilters> {
  return http.get('/jobs/filters', {
    params: {
      database: joinMulti(params?.database),
      project: joinMulti(params?.project),
    },
  })
}

export function controlJobs(
  action: 'delete' | 'rerun' | 'suspend' | 'resume',
  names: string[],
): Promise<ControlResult> {
  return http.post('/jobs/control', { action, names })
}
