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
  changePassword: (data)       => api.post('/user/change-password', data),
  totpSetup:      ()           => api.get('/user/totp/setup'),
  totpEnable:     (data)       => api.post('/user/totp/enable', data),
  totpDisable:    (data)       => api.post('/user/totp/disable', data),
  getChannel:     ()           => api.get('/user/channel'),
  updateChannel:  (data)       => api.put('/user/channel', data),
}

// ---- 电量接口 ----
export const powerAPI = {
  current:      ()           => api.post('/power/current'),          // 当前电量
  records:      (limit = 30) => api.get(`/records?limit=${limit}`), // 历史记录
}

// ---- 水量接口 ----
export const waterAPI = {
  balance: () => api.post('/water/balance'),                        // 水量余额
}

// ---- 下拉选项接口 ----
export const dormAPI = {
  getOptions: () => api.get('/sync/dorm-options'),
  sync: () => api.post('/sync/dorm-options'),
}

// ---- 系统配置接口（公开，无需认证）----
export const systemAPI = {
  getConfig: () => api.get('/system/config'),
}

// ---- 管理后台接口 ----
// 使用 X-Admin-Token 头，token 存于 localStorage.eq_admin_token
const adminToken = () => localStorage.getItem('eq_admin_token') || ''

export const adminAPI = {
  // 用户管理
  listUsers:  (params = {}) => api.get('/admin/users', {
    params,
    headers: { 'X-Admin-Token': adminToken() }
  }),
  deleteUser: (id) => api.delete(`/admin/users/${id}`, {
    headers: { 'X-Admin-Token': adminToken() }
  }),
  resetPassword: (id) => api.post(`/admin/users/${id}/reset-password`, {}, {
    headers: { 'X-Admin-Token': adminToken() }
  }),
  disableTOTP: (id) => api.post(`/admin/users/${id}/disable-totp`, {}, {
    headers: { 'X-Admin-Token': adminToken() }
  }),
  // 同步管理
  getSyncStatus: () => api.get('/admin/sync/status', {
    headers: { 'X-Admin-Token': adminToken() }
  }),
  triggerSync: () => api.post('/admin/sync/trigger', {}, {
    headers: { 'X-Admin-Token': adminToken() }
  }),
}

export default api
