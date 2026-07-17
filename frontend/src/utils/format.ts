import dayjs from 'dayjs'

export function fmtTime(ts?: number | null, fallback = '-'): string {
  if (!ts || ts <= 0) return fallback
  return dayjs.unix(ts).format('YYYY-MM-DD HH:mm:ss')
}

export function fmtTimeShort(ts?: number | null): string {
  if (!ts || ts <= 0) return '-'
  return dayjs.unix(ts).format('MM-DD HH:mm:ss')
}

/** Format seconds into a human-readable duration like "1h 2m 3s". */
export function fmtDuration(sec?: number | null): string {
  if (!sec || sec <= 0) return '0s'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  const parts: string[] = []
  if (h > 0) parts.push(`${h}h`)
  if (m > 0) parts.push(`${m}m`)
  if (s > 0 || parts.length === 0) parts.push(`${s}s`)
  return parts.join(' ')
}
