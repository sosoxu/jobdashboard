import axios from 'axios'

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

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
    let msg = error.message || '网络错误'
    if (error.response?.data?.msg) msg = error.response.data.msg
    return Promise.reject(new Error(msg))
  },
)

export default http
