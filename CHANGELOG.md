# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [2.0.2] — 2026-04-27

### 🔒 安全增强

| # | 问题 | 修复方案 | 影响文件 |
|---|------|----------|----------|
| 1 | 登录/注册无速率限制 | 滑动窗口限流：登录 10次/5分钟，注册 5次/10分钟 | `middleware/ratelimit.go` |
| 2 | 管理后台双重鉴权不足 | JWT + AdminToken 同时验证 | `middleware/auth.go` |
| 3 | TOTP 密钥明文存储 | AES-256-GCM 加密，向后兼容明文旧数据 | `internal/cryptoutil/` |
| 4 | 错误信息暴露内部细节 | API 响应统一错误归一化，不返回 `err.Error()` | 全局 handler |

### 🔄 API 重构

| 旧路径 | 新路径 | 说明 |
|--------|--------|------|
| `POST /api/power/query` | `POST /api/power/current` | 电量查询 |
| `GET /api/power/history` | `GET /api/records` | 历史记录 |
| `POST /api/power/water` | `POST /api/water/balance` | 水量查询 |

### 🏗️ 架构优化

- **包名冲突解决**：`internal/sync` → `internal/dormsyncer`，避免与 Go stdlib 冲突
- **日志模块重构**：
  - 相对路径改为相对于工作目录（非可执行文件目录）
  - 支持 Minecraft 风格日志轮转（`app.log.YYYY-MM-DD.gz`）
- **前端目录重组**：页面按 `auth/`、`main/`、`admin/` 分类
- **下拉选项缓存**：前端 localStorage 缓存宿舍选项，减少重复请求

### ⚡ 性能优化

- **并发轮询**：errgroup + 信号量，最多 5 并发，ctx 5min 超时熔断
- **N+1 查询修复**：用户列表改为批量 IN 查询
- **O(n²) 排序修复**：历史数据排序改 `sort.Slice`

### 🐛 Bug Fixes

| 问题 | 修复 |
|------|------|
| 刷新按钮出现 12 次请求 | 重置自动刷新定时器，避免重叠 |
| 自动刷新间隔错误（30s） | 修正为 5 分钟 |
| `MePage.vue` 引用已删除函数 | 移除死代码引用 |
| 主题切换重复调用 | 合并为单一 `applyTheme()` 调用 |

---

## [2.0.1] — 2026-04-27

### 🔒 安全修复

| # | 问题 | 修复方案 | 影响文件 |
|---|------|----------|----------|
| 1 | JWT 密钥未校验长度，可使用弱密钥 | 启动时强制校验 `len(jwt_secret) >= 32`，不足则 `log.Fatal` | `config.go` |
| 2 | 生产模式未强制要求 `allowed_origin` | 启动时 `mode != debug` 且 `allowed_origin` 为空则 `log.Fatal` | `config.go` |
| 3 | 密码复杂度无验证，可设 "12345678" | 注册/改密时正则校验：至少 3 种字符类型 | `user_service.go` |
| 4 | AdminToken 与 InternalToken 共用 | 新增独立 `admin_token` 配置项，向后兼容 fallback | `config.go` / `auth.go` / `*.conf.example` |

---

## [2.0.0] — 2026-04-26

> ⚠️ **破坏性升级** — 不支持从 v1.x 数据平滑迁移。数据库结构、API 接口、登录方式均有变更。

### 🔴 Breaking Changes

| 模块 | 变更 | 说明 |
|------|------|------|
| 登录方式 | `学号` → `用户名` 登录 | 登录表单改为"用户名"，注册不再要求学号 |
| 学号绑定 | 学号解耦为独立 API | 学号通过 `POST /api/user/student-id` 单独绑定，不再在注册时填写 |
| 水量查询 | 客户端传参 → 服务端取宿舍号 | `POST /api/power/water` 不再接受 `dorm_room` 参数，服务端从用户 profile 自动获取 |
| JWT Claims | 存 `username/sid` → 存 `uuid` | JWT Token 内容变更，旧 Token 失效 |
| 数据库主键 | 自增 ID → UUID v7 | 所有表主键改为时序 UUID（高性能 B+ 树插入顺序） |

### 🟠 安全性升级

- **JWT 签名算法强制** — 拒绝 `alg: none` 等无签名 Token，防止签名绕过攻击
- **bcrypt cost 提升** — 10 → 12，增量哈希耗时防暴力破解
- **敏感日志脱敏** — Webhook URL、邮箱地址不再出现在日志中，仅记录布尔标记
- **InternalToken 强制非空** — 启动时校验，空值则 `log.Fatal`，防止误配置
- **CORS 生产限制** — `allowed_origin` 可配置，生产环境不再通配 `*`
- **GORM 预编译 SQL** — `PrepareStmt: true`，防止 SQL 注入
- **注册竞态修复** — 删除 SELECT 预判，依赖数据库唯一索引保证并发安全

### 🟡 功能重构

- **通知渠道解耦** — 测试通知由 `send_test_notification` 改为可选触发，渠道保存后可即时验证
- **宿舍选项同步** — 后台自动同步 ydgl.xzcit.cn，支持手动触发（`POST /api/admin/sync/trigger`）
- **管理后台** — 用户管理（列表/删除）+ 同步状态监控，`X-Admin-Token` 鉴权
- **水量数据完整入库** — `remaining_water` 历史数据写入，`WaterAmount` 入库
- **数据库层优化** — SQLite WAL 模式（写不阻塞读）+ busy_timeout + 连接池配置
- **日志模块重构** — `log/slog` + JSONHandler + lumberjack 文件轮转

### 🔵 Bug Fixes

- 水量显示负号（预付费透支）→ 前端统一取绝对值
- 历史趋势表缺少水量列 → 新增水量列，仅在水数据存在时显示
- `queryWater` 客户端越权传参 → 改为服务端从 profile 取
- `parseDorm` 边界 panic → `len(suffix) < 2` 时安全返回
- `UserChannel` 软删除查询不一致 → 统一 `Unscoped()`

### 🟢 清理

- 删除所有测试脚本（`test_*.go`、`test_*.py`）
- 删除所有编译产物（`*.exe`）
- 删除 Python 缓存（`__pycache__/`、`.venv/`）
- 归档旧 Python 代码至 `archive` 分支

### 📦 依赖更新

| 依赖 | 变更 |
|------|------|
| `golang.org/x/crypto` | v0.31.0 → **v0.50.0** |
| `golang.org/x/net` | v0.33.0 → **v0.52.0** |
| `github.com/google/uuid` | v1.6.0 → **v1.6.0+**（支持 NewV7） |
| `golang.org/x/sync` | 新增（errgroup 并发轮询） |
| `github.com/natefinch/lumberjack` | 新增（日志文件轮转） |

---

## [1.0.0] — 早期版本

> 已归档至 `archive` 分支，不再维护。
