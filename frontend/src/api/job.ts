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

export function getJobFilters(): Promise<JobFilters> {
  return http.get('/jobs/filters')
}

export function controlJobs(
  action: 'delete' | 'rerun' | 'suspend' | 'resume',
  names: string[],
): Promise<ControlResult> {
  return http.post('/jobs/control', { action, names })
}
