import axios from 'axios'
import { generateToken } from '../utils/token'

// 使用相对路径，vite 的 proxy 会把 /api 转发到后端
const client = axios.create({
  baseURL: '/api',
  timeout: 5000
})

// 插入内部 token（可通过 VITE_INTERNAL_TOKEN 设置在构建/运行时）
const internalToken = import.meta.env.VITE_INTERNAL_TOKEN || ''
if (internalToken) {
  client.defaults.headers.common['X-Internal-Token'] = internalToken
}

// 管理员 token secret（从 vite.config 注入，用于生成短期 token）
const adminTokenSecret = import.meta.env.VITE_ADMIN_TOKEN_SECRET || ''

// request interceptor 在每次请求时动态生成短期 token 并注入 X-Admin-Token
client.interceptors.request.use(async (cfg) => {
  if (adminTokenSecret) {
    try {
      const token = await generateToken(adminTokenSecret)
      cfg.headers['X-Admin-Token'] = token
    } catch (e) {
      console.error('Failed to generate admin token:', e)
    }
  }
  return cfg
})

export default {
  // 业务 API
  bind (payload) {
    return client.post('/bind', payload).then(r => r.data).catch(() => null)
  },
  unbind (payload) {
    return client.post('/unbind', payload).then(r => r.data).catch(() => null)
  },
  bindings () {
    return client.get('/bindings').then(r => r.data).catch(() => null)
  },
  power (dorm) {
    return client.get(`/power/${dorm}`).then(r => r.data).catch(() => null)
  },
  notify (payload) {
    return client.post('/notify', payload).then(r => r.data).catch(() => null)
  }
}
