import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 开发时可在此处写死前后端通信的管理员 token secret（仅用于本地开发）
// 注意：这里是用于生成 HMAC token 的 secret，不是 token 本身
const ADMIN_TOKEN_SECRET = 'REPLACE_WITH_YOUR_SECRET'

export default defineConfig({
  plugins: [vue()],
  define: {
    // 注入到客户端代码中的环境变量（构建时会被替换）
    'import.meta.env.VITE_ADMIN_TOKEN_SECRET': JSON.stringify(ADMIN_TOKEN_SECRET)
  },
  server: {
    port: 5173,
    proxy: {
      // 将 /api 转发到本地后端
      '/api': {
        target: 'http://localhost:5000',
        changeOrigin: true,
        secure: false,
        rewrite: (path) => path
      }
    }
  }
})
