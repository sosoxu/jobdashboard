export enum JobState {
  jsFrozen = 0,
  jsQueue = 1,
  jsScheduled = 2,
  jsReady = 3,
  jsReleased = 4,
  jsActive = 5,
  jsSuspending = 6,
  jsSuspended = 7,
  jsResuming = 8,
  jsCanceling = 9,
  jsFinished = 10,
  jsFailed = 11,
  jsCanceled = 12,
}

export type StateTagType = 'success' | 'warning' | 'info' | 'danger' | 'primary'

export interface StateMeta {
  label: string
  type: StateTagType
}

export const STATE_META: Record<number, StateMeta> = {
  5: { label: '运行中', type: 'success' },
  1: { label: '排队中', type: 'primary' },
  0: { label: '冻结', type: 'info' },
  2: { label: '已调度', type: 'info' },
  3: { label: '就绪', type: 'info' },
  4: { label: '已释放', type: 'info' },
  6: { label: '挂起中', type: 'warning' },
  7: { label: '已挂起', type: 'warning' },
  8: { label: '恢复中', type: 'warning' },
  9: { label: '取消中', type: 'warning' },
  10: { label: '已完成', type: 'info' },
  11: { label: '失败', type: 'danger' },
  12: { label: '已取消', type: 'warning' },
}

export function stateMeta(code: number): StateMeta {
  return STATE_META[code] || { label: '未知', type: 'info' }
}

export function stateLabel(code: number): string {
  return stateMeta(code).label
}

// Filterable status options for the job table.
export const STATUS_OPTIONS: { value: number; label: string }[] = [
  { value: JobState.jsActive, label: '运行中' },
  { value: JobState.jsQueue, label: '排队中' },
  { value: JobState.jsFrozen, label: '冻结' },
  { value: JobState.jsScheduled, label: '已调度' },
  { value: JobState.jsReady, label: '就绪' },
  { value: JobState.jsSuspending, label: '挂起中' },
  { value: JobState.jsSuspended, label: '已挂起' },
  { value: JobState.jsResuming, label: '恢复中' },
  { value: JobState.jsCanceling, label: '取消中' },
  { value: JobState.jsFinished, label: '已完成' },
  { value: JobState.jsFailed, label: '失败' },
  { value: JobState.jsCanceled, label: '已取消' },
]
