# ⚡ ElectricQuery 宿舍水电查询系统

Go + Vue 3 版 v2.1.0，支持电量/水量查询、历史趋势、告警通知。

> ⚠️ **v2.0.0 为破坏性升级**，不支持从 v1.x 数据平滑迁移。
> ⚠️ **v2.1.0 数据库重构**，电量/水量拆分为独立表，启动自动迁移。

---

## 🧩 功能特性

- ⚡ 实时查询宿舍剩余电量与水量（服务端从用户 profile 取宿舍号，防越权）
- 📊 历史趋势图（近 14 天折线图，含电量 + 水量日消耗量）
- 🔔 多渠道告警通知（企业微信 / 邮件，敏感日志脱敏）
- 📱 响应式 Web 界面（Material Design 3）
- 🔐 JWT 认证，用户名登录，支持改密和 TOTP 两步验证
- 🛡️ 登录/注册速率限制（默认登录 10 次/5 分钟，注册 5 次/10 分钟）
- 👤 学号绑定（解耦于注册流程，独立 API 绑定）
- 🏠 宿舍选项同步（ydgl.xzcit.cn，后台可手动触发）
- ⚙️ 管理后台（用户管理 / 同步状态 / 强制关闭 TOTP / 手动查询宿舍电量，需 `X-Admin-Token` 鉴权）
- 📋 历史记录正/倒序切换，充值记录友好显示

---

## 🛠️ 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.25 + Gin + GORM |
| 前端 | Vue 3 + Vite + Vuetify 3 |
| 数据库 | SQLite（`glebarez/sqlite` 纯 Go 驱动，可切换 MySQL） |
| 配置 | HOCON（`application.conf`，支持注释） |

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
# 编辑 application.conf，填入 jwt_secret、internal_token、admin_token 等
mkdir -p data logs
```

如果使用 Docker，默认会挂载 `application.conf.example`。要切换成正式配置，把 `.env` 里的 `HOST_CONFIG_FILE` 改成 `./application.conf`。

### 3. 启动后端

```bash
go run ./cmd/server/
# 监听端口：8080
# 启动时自动同步一次宿舍选项
```

### 4. 启动前端

```bash
cd frontend
npm install
npm run dev
# 访问：http://localhost:5173
```

---

## ⚙️ 配置说明

### 配置文件结构

配置文件为 HOCON 格式，存于项目根目录 `application.conf`（勿提交至仓库）。HOCON 兼容 JSON，同时支持 `#` / `//` 注释、可省略逗号和根对象大括号。

敏感字段（jwt_secret / internal_token / admin_token / smtp.password）可直接写在文件中，
**或**使用环境变量覆盖（环境变量优先级更高，适合容器化部署）。

### app（服务端）

| HOCON 键 | 类型 | 默认值 | 说明 |
|---------|------|--------|------|
| `host` | string | `"0.0.0.0"` | 服务监听地址 |
| `port` | int | `8080` | HTTP 服务端口 |
| `jwt_secret` | string | — | JWT 签名密钥，**必须 >= 32 字节**，生产必填 |
| `jwt_expire_hours` | int | `72` | JWT Token 有效期（小时） |
| `internal_token` | string | — | 内部同步 Token，**必填**，用于后端定时任务鉴权 |
| `admin_token` | string | `""` | 管理后台 Token，v2.0.1+，生产建议独立配置 |
| `mode` | string | `"debug"` | 运行模式：`debug` / `info` / `release` |
| `allowed_origin` | string | `""` | 前端域名，`mode != debug` 时必填 |
| `max_login_per_window` | int | `10` | 登录限流次数，配合 `rate_limit_window_sec` 使用 |
| `max_register_per_window` | int | `5` | 注册限流次数，配合 `rate_limit_window_sec` 使用 |
| `rate_limit_window_sec` | int | `300` | 登录/注册限流窗口（秒） |

### log（日志）

| HOCON 键 | 类型 | 默认值 | 说明 |
|---------|------|--------|------|
| `level` | string | `"info"` | 日志级别：`debug` / `info` / `warn` / `error` |
| `path` | string | `"logs/app.log"` | 日志文件路径 |
| `max_size_mb` | int | `10` | 单文件最大体积（MB），超过自动轮转 |
| `max_backups` | int | `7` | 保留历史日志文件份数 |
| `max_age_days` | int | `30` | 保留历史日志天数 |
| `compress` | bool | `true` | 轮转后是否 gzip 压缩历史文件 |
| `console` | bool | `true` | 是否同时输出到 stdout |

### database（数据库）

| HOCON 键 | 类型 | 说明 |
|---------|------|------|
| `driver` | string | 驱动类型：`sqlite`（默认）或 `mysql` |
| `max_open_conns` | int | MySQL 最大打开连接数，默认 `50` |
| `max_idle_conns` | int | MySQL 最大空闲连接数，默认 `10` |
| `sqlite.path` | string | SQLite 数据库文件路径 |
| `mysql.host` | string | MySQL 主机地址 |
| `mysql.port` | int | MySQL 端口（默认 3306） |
| `mysql.user` | string | MySQL 用户名 |
| `mysql.password` | string | MySQL 密码 |
| `mysql.dbname` | string | 数据库名 |
| `mysql.charset` | string | 字符集（默认 `utf8mb4`） |
| `mysql.loc` | string | 时区（URL 编码，如 `Asia%2FShanghai`） |

### smtp（邮件通知）

| HOCON 键 | 类型 | 说明 |
|---------|------|------|
| `enabled` | bool | 是否启用邮件通知 |
| `sender_email` | string | 发件人地址 |
| `sender_name` | string | 发件人显示名称 |
| `server` | string | SMTP 服务器（如 `smtp.163.com`） |
| `port` | int | 端口（465=SSL，587=STARTTLS） |
| `use_ssl` | bool | 是否使用 SSL |
| `password` | string | SMTP 授权码（163/QQ 邮箱需开启 IMAP 后获取） |

### power_checker（水电爬虫）

| HOCON 键 | 类型 | 默认值 | 说明 |
|---------|------|--------|------|
| `login_url` | string | （固定值） | 登录页面 URL，一般无需修改 |
| `user_agent` | string | Chrome UA | HTTP 请求 User-Agent |
| `timeout_seconds` | int | `15` | 请求超时时间（秒） |

### scheduler（定时调度）

| HOCON 键 | 类型 | 默认值 | 说明 |
|---------|------|--------|------|
| `poll_interval` | int | `600` | 全量查询间隔（秒），建议 >= 900 |
| `alert_threshold` | float | `20.0` | 电量告警阈值（度） |
| `weekly_report_weekday` | int | `1` | 每周报告推送日（1=周一，7=周日） |
| `weekly_report_hour` | int | `8` | 每周报告推送小时（24小时制） |

---

## 🌎 环境变量对照表

当前支持下表中的配置项通过环境变量覆盖（优先级高于配置文件）。
前缀统一为 `EQ_`，布尔值接受 `true`/`1`（不区分大小写）。

| 环境变量 | 对应配置路径 | 类型 | 说明 |
|----------|-------------|------|------|
| `EQ_HOST` | `app.host` | string | 服务监听地址 |
| `EQ_PORT` | `app.port` | int | 服务端口 |
| `EQ_MODE` | `app.mode` | string | 运行模式 |
| `EQ_JWT_EXPIRE_HOURS` | `app.jwt_expire_hours` | int | JWT 有效期 |
| `EQ_JWT_SECRET` | `app.jwt_secret` | string | JWT 签名密钥 |
| `EQ_INTERNAL_TOKEN` | `app.internal_token` | string | 内部同步 Token |
| `EQ_ADMIN_TOKEN` | `app.admin_token` | string | 管理后台 Token |
| `EQ_ALLOWED_ORIGIN` | `app.allowed_origin` | string | 前端域名 |
| `EQ_LOG_LEVEL` | `log.level` | string | 日志级别 |
| `EQ_LOG_PATH` | `log.path` | string | 日志文件路径 |
| `EQ_LOG_MAX_SIZE_MB` | `log.max_size_mb` | int | 日志文件最大体积 |
| `EQ_LOG_MAX_BACKUPS` | `log.max_backups` | int | 保留历史日志份数 |
| `EQ_LOG_MAX_AGE_DAYS` | `log.max_age_days` | int | 保留历史日志天数 |
| `EQ_LOG_COMPRESS` | `log.compress` | bool | 是否压缩轮转日志 |
| `EQ_LOG_CONSOLE` | `log.console` | bool | 是否输出到 stdout |
| `EQ_DB_DRIVER` | `database.driver` | string | 数据库驱动 |
| `EQ_DB_MAX_OPEN` | `database.max_open_conns` | int | MySQL 最大打开连接数 |
| `EQ_DB_MAX_IDLE` | `database.max_idle_conns` | int | MySQL 最大空闲连接数 |
| `EQ_SQLITE_PATH` | `database.sqlite.path` | string | SQLite 数据库路径 |
| `EQ_MYSQL_HOST` | `database.mysql.host` | string | MySQL 主机 |
| `EQ_MYSQL_PORT` | `database.mysql.port` | int | MySQL 端口 |
| `EQ_MYSQL_USER` | `database.mysql.user` | string | MySQL 用户 |
| `EQ_MYSQL_PASSWORD` | `database.mysql.password` | string | MySQL 密码 |
| `EQ_MYSQL_DBNAME` | `database.mysql.dbname` | string | 数据库名 |
| `EQ_MYSQL_CHARSET` | `database.mysql.charset` | string | MySQL 字符集 |
| `EQ_MYSQL_LOC` | `database.mysql.loc` | string | MySQL 时区 |
| `TZ` | 容器系统时区 | string | Docker 默认时区，建议 `Asia/Shanghai` |
| `EQ_SMTP_ENABLED` | `smtp.enabled` | bool | 是否启用邮件 |
| `EQ_SMTP_SERVER` | `smtp.server` | string | SMTP 服务器 |
| `EQ_SMTP_PORT` | `smtp.port` | int | SMTP 端口 |
| `EQ_SMTP_USER` | `smtp.sender_email` | string | 发件人邮箱 |
| `EQ_SMTP_PASSWORD` | `smtp.password` | string | SMTP 授权码 |
| `EQ_SMTP_USE_SSL` | `smtp.use_ssl` | bool | SMTP 是否使用 SSL |
| `EQ_LOGIN_URL` | `power_checker.login_url` | string | 爬虫登录 URL |
| `EQ_TIMEOUT_SECONDS` | `power_checker.timeout_seconds` | int | 请求超时（秒） |
| `EQ_POLL_INTERVAL` | `scheduler.poll_interval` | int | 轮询间隔（秒） |
| `EQ_WEEKLY_REPORT_WEEKDAY` | `scheduler.weekly_report_weekday` | int | 每周报告星期 |
| `EQ_WEEKLY_REPORT_HOUR` | `scheduler.weekly_report_hour` | int | 每周报告小时 |

> 登录/注册限流可以通过 `app.max_login_per_window`、`app.max_register_per_window`、`app.rate_limit_window_sec` 在配置文件里调整。

---

## 📡 API 概览

### 认证相关

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `/api/auth/register` | 注册（用户名 + 密码） | — |
| POST | `/api/auth/login` | 登录（用户名 + 密码） | — |

### 用户相关

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `/api/user/profile` | 获取个人信息 | JWT |
| PATCH | `/api/user/profile` | 更新个人信息 | JWT |
| POST | `/api/user/student-id` | 绑定学号（独立接口） | JWT |
| POST | `/api/user/validate-dorm` | 验证宿舍号（调用爬虫） | JWT |
| POST | `/api/user/change-password` | 修改登录密码 | JWT |
| GET | `/api/user/totp/setup` | 生成 TOTP 密钥 | JWT |
| POST | `/api/user/totp/enable` | 启用两步验证 | JWT |
| POST | `/api/user/totp/disable` | 关闭两步验证 | JWT |
| GET | `/api/user/channel` | 获取通知渠道 | JWT |
| PUT | `/api/user/channel` | 更新通知渠道 | JWT |

### 电量查询

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `/api/power/current` | 查询当前电量 | JWT |
| GET | `/api/records` | 查询历史记录（limit ≤ 365） | JWT |

### 水量查询

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `/api/water/balance` | 查询当前水量 | JWT |

### 宿舍选项

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `/api/sync/dorm-options` | 获取宿舍下拉选项 | JWT |
| POST | `/api/sync/dorm-options` | 触发同步（Internal Token） | — |

### 运维

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `/health` | 健康检查，返回数据库和目标站点状态 | — |

### 管理后台（需 `X-Admin-Token` 头）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/admin/users` | 用户列表 |
| DELETE | `/api/admin/users/:id` | 删除用户 |
| POST | `/api/admin/users/:id/reset-password` | 重置密码 |
| POST | `/api/admin/users/:id/disable-totp` | 强制关闭两步验证 |
| GET | `/api/admin/sync/status` | 同步状态 |
| POST | `/api/admin/sync/trigger` | 手动触发同步 |
| POST | `/api/admin/power/query` | 管理员手动查询宿舍电量 |

---

## 📁 项目结构

```
ElectricQuery/
├── cmd/server/          # Go 主入口
├── internal/
│   ├── cache/          # 内存缓存模块
│   ├── config/         # 配置解析（HOCON + 环境变量覆盖）
│   ├── cryptoutil/     # AES-256-GCM 加密工具
│   ├── dormsyncer/     # 宿舍选项同步（ydgl.xzcit.cn）
│   ├── handler/        # Gin 路由处理
│   ├── logger/         # 结构化日志（log/slog + lumberjack）
│   ├── middleware/     # JWT / CORS / 速率限制 中间件
│   ├── migrations/     # 数据库迁移模块（幂等）
│   ├── model/          # GORM 模型（electricity_logs / water_logs）
│   ├── notifier/       # SMTP + 企业微信推送
│   ├── scheduler/      # 定时调度（poller + notifier 双循环）
│   ├── service/        # 业务逻辑
│   └── checker/        # 电量/水量爬虫核心
├── frontend/            # Vue 3 前端（Vuetify 3）
├── application.conf     # 运行时配置（含密钥，勿提交）
├── application.conf.example  # 配置模板（可提交）
└── README.md
```

---

## 🐳 Docker 部署

### 快速启动

```bash
# 1. 拉取镜像
docker pull ghcr.io/nxygen/electricquery:latest
# 或阿里云 ACR
docker pull registry.cn-hangzhou.aliyuncs.com/nxygen/electricquery:latest

# 2. 准备配置（环境变量或挂载 application.conf）
docker run -d \
  -p 8080:8080 \
  -e TZ="Asia/Shanghai" \
  -e EQ_JWT_SECRET="your-secret-min-32bytes" \
  -e EQ_INTERNAL_TOKEN="your-internal-token" \
  -e EQ_ADMIN_TOKEN="your-admin-token" \
  -e EQ_DB_DRIVER="sqlite" \
  -e EQ_SQLITE_PATH="/app/data/electricquery.db" \
  -v ./data:/app/data \
  -v ./logs:/app/logs \
  --name electricquery \
  ghcr.io/nxygen/electricquery:latest
```

### 使用 docker-compose（推荐）

```bash
# 1. 准备环境变量
cp .env.example .env
# 编辑 .env，填入 JWT_SECRET 等敏感配置
# 如需使用文件配置：
# cp application.conf.example application.conf
# 然后将 .env 里的 HOST_CONFIG_FILE 改为 ./application.conf

# 2. 启动
docker compose up -d

# 3. 查看日志
docker compose logs -f

# 4. 停止
docker compose down
```

> **生产环境** 建议使用 Docker Secret 或外部配置中心管理敏感信息，避免明文 `.env` 文件。
> 容器内默认从 `/app/application.conf` 读取配置，健康检查使用 `/health`。

### 镜像地址

| 仓库 | 地址 |
|------|------|
| GitHub Container Registry | `ghcr.io/nxygen/electricquery:latest` |
| 阿里云 ACR（国内加速） | `registry.cn-hangzhou.aliyuncs.com/nxygen/electricquery:latest` |

---

## 🚀 CI/CD

GitHub Actions 自动构建并推送镜像：

- **触发条件**：推送到 `dev` 分支 / 发起 `dev` 的 PR
- **流程**：Go 编译 → 前端构建 → Docker 多平台镜像构建 → 推送到 GHCR + 阿里云 ACR
- **镜像标签**：`latest` + `vX.Y.Z`（Git tag 触发）

```bash
# 本地手动构建镜像
docker build -t electricquery:local .
```

---

## 📜 开源协议

MIT License
