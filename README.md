# ⚡ ElectricQuery 宿舍水电查询系统

Go + Vue 3 重构版，支持电量/水量查询、历史趋势、告警通知。

---

## 🧩 功能特性

- ⚡ 实时查询宿舍剩余电量与水量（C13/C14 楼）
- 📊 历史趋势图（近 14 天）
- 🔔 多渠道告警通知（企业微信 / 邮件）
- 📱 响应式 Web 界面（Material Design 3）
- 🔐 JWT 认证，学号登录

---

## 🛠️ 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.22 + Gin + GORM |
| 前端 | Vue 3 + Vite + Vuetify 3 |
| 数据库 | SQLite（可切换 MySQL） |
| 配置 | HOCON 格式（`application.conf`） |

---

## 📦 快速开始

### 1. 克隆项目

```bash
git clone <repo>
cd ElectricQuery
```

### 2. 配置

```bash
cp application.conf.example application.conf
# 编辑 application.conf，填入数据库、通知渠道等信息
```

### 3. 启动后端

```bash
go run ./cmd/server/
# 监听端口：8080
```

### 4. 启动前端

```bash
cd frontend
npm install
npm run dev
# 访问：http://localhost:5173
```

---

## 📁 项目结构

```
ElectricQuery/
├── cmd/server/          # Go 主入口
├── internal/
│   ├── config/         # HOCON 配置解析
│   ├── model/          # GORM 模型
│   ├── handler/        # Gin 路由处理
│   ├── service/        # 业务逻辑
│   ├── checker/        # 电量爬虫核心
│   ├── scheduler/      # 定时调度
│   ├── middleware/     # JWT 中间件
│   └── notifier/       # SMTP + 企业微信推送
├── frontend/           # Vue 3 前端
├── application.conf.example  # 配置模板
└── README.md
```

---

## 📜 开源协议

MIT License
