export interface AuthUser {
  username: string
}

export interface AuthResult {
  token: string
  user: AuthUser
}

export interface GroupStat {
  key: string
  label: string
  count: number
  prevCount: number
  delta: number
  deltaPct: number
}

export interface StatsResult {
  updatedAt: number
  groups: GroupStat[]
  degraded: boolean
}

export interface TrendPoint {
  ts: number
  finish: number
  active: number
  queue: number
  failed: number
  canceled: number
}

export interface TrendResult {
  range: string
  points: TrendPoint[]
}

export interface TopUser {
  userName: string
  count: number
  pct: number
}

export interface TopUsersResult {
  window: string
  ts: number
  total: number
  users: TopUser[]
  others: TopUser
}

export interface JobListItem {
  jobName: string
  jobDesc: string
  jobStatus: number
  jobStatusLabel: string
  userName: string
  jobProcess: number
  project: string
  survey: string
  database: string
  department: string
  application: string
  execTime: number
  waitTime: number
  commitTime: number
  startTime: number
  endTime: number
  exitCode: number
  summary: string
}

export interface JobListResult {
  total: number
  page: number
  pageSize: number
  list: JobListItem[]
  cached?: boolean
  cacheTs?: number
}

export interface JobFilters {
  cacheTs: number
  projects: string[]
  surveys: string[]
  users: string[]
  databases: string[]
}

export interface ControlResult {
  success: string[]
  failed: { name: string; reason: string }[]
}

export interface LogLine {
  lineNo: number
  text: string
}

export interface LogSection {
  name: string
  lineNo: number
}

export interface LogResult {
  jobName: string
  type: string
  path: string
  page: number
  pageSize: number
  total: number       // -1 表示未知
  lines: LogLine[]
  truncated: boolean
  files: string[]
  filtered: boolean
  cached: boolean
  sections: LogSection[] | null  // list 文件的段落列表；log 类型为 null
}

export interface ModuleError {
  module: string
  count: number
}

export interface Suggestion {
  code: string
  desc: string
  severity: string
}

export interface AnalyzeResult {
  mode: string
  summary: string
  moduleErrors: ModuleError[]
  suggestions: Suggestion[]
}
