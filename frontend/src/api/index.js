import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 20000,
  headers: { 'Content-Type': 'application/json' }
})

// 请求拦截器：自动附带 JWT
api.interceptors.request.use(config => {
  const token = localStorage.getItem('eq_token')
  if (token) {
    config.headers['Authorization'] = `Bearer ${token}`
  }
  return config
})

// 响应拦截器：401 自动跳转登录
api.interceptors.response.use(
  res => res,
  err => {
    if (err.response?.status === 401) {
      localStorage.removeItem('eq_token')
      localStorage.removeItem('eq_user')
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)

// ---- 认证接口 ----
export const authAPI = {
  register: (data) => api.post('/auth/register', data),
  login:    (data) => api.post('/auth/login', data),
}

// ---- 用户接口 ----
export const userAPI = {
  getProfile:     ()           => api.get('/user/profile'),
  updateProfile:  (data)       => api.patch('/user/profile', data),
  bindStudentId:  (data)       => api.post('/user/student-id', data),
  validateDorm:   (data)       => api.post('/user/validate-dorm', data),
  getChannel:     ()           => api.get('/user/channel'),
  updateChannel:  (data)       => api.put('/user/channel', data),
}

// ---- 电量接口 ----
export const powerAPI = {
  query:     ()            => api.post('/power/query'),
  queryWater: (dormRoom)   => api.post('/power/water', { dorm_room: dormRoom }),
  history:   (limit = 30)  => api.get(`/power/history?limit=${limit}`),
}

export default api
