# ElectricQuery - Frontend

这是前端骨架（Vue 3 + Vite + Element Plus），用于绑定宿舍、企业微信用户与邮箱，并与后端 `/api` 接口通信。

快速运行（Windows PowerShell）:

```powershell
cd frontend
npm install
npm run dev
```

- 开发服务器会在 http://localhost:5173
- Vite dev server 已配置 proxy，将 `/api` 转发到 `http://localhost:5000`（可根据需要修改 `vite.config.js`）
- 可通过环境变量 `VITE_INTERNAL_TOKEN` 在开发/构建时设置内部 token，示例：

```powershell
$env:VITE_INTERNAL_TOKEN = "your_token_here"
npm run dev
```

下一步建议：
- 根据后端 API 细化字段和错误处理
- 添加表单反馈提示与加载态
- 增加测试、ESLint 和格式化配置
