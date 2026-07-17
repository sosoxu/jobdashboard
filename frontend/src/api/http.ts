import axios from 'axios'

export const TOKEN_KEY = 'job_dashboard_token'

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

// Request interceptor: attach Bearer token from localStorage when present.
http.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_KEY)
  if (token && !config.headers.Authorization) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Response interceptor: unwrap unified envelope { code, msg, data };
// on HTTP 401 clear token and redirect to /login (unless already there).
http.interceptors.response.use(
  (resp) => {
    const body = resp.data
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code === 0) {
        return body.data
      }
      const err = new Error(body.msg || '请求失败')
      return Promise.reject(err)
    }
    return body
  },
  (error) => {
    const status = error.response?.status
    if (status === 401) {
      localStorage.removeItem(TOKEN_KEY)
      // Avoid redirect loop: skip when already on /login or when the
      // failing request is the login/register endpoint itself.
      const url: string = error.config?.url || ''
      const isAuthEndpoint = url.includes('/auth/login') || url.includes('/auth/register')
      const onLoginPage = window.location.pathname === '/login'
      if (!isAuthEndpoint && !onLoginPage) {
        const redirect = encodeURIComponent(window.location.pathname + window.location.search)
        window.location.href = `/login?redirect=${redirect}`
      }
    }
    let msg = error.message || '网络错误'
    if (error.response?.data?.msg) msg = error.response.data.msg
    return Promise.reject(new Error(msg))
  },
)

export default http
