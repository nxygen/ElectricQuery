# ElectricQuery v2.1.0 发布说明

**2026-05-21**

---

## 重要变更

- **数据库拆表**：历史水电记录从 `power_logs` 拆分为 `electricity_logs` 和 `water_logs`，启动时自动执行幂等迁移。
- **HOCON 配置**：`application.conf` 支持 `#` / `//` 注释，`application.conf.example` 已改为带注释模板。
- **容器部署更新**：新增 `.env.example`、`.dockerignore`，`docker-compose.yml` 默认挂载 `/app/application.conf` 并使用环境变量覆盖敏感配置。
- **容器时区修复**：镜像默认设置 `TZ=Asia/Shanghai`，避免容器内时间按 UTC 记录和调度。
- **安全增强**：登录 CSRF Token、TOTP 两步验证、管理员强制关闭 TOTP、可配置登录/注册限流。

---

## 新功能

- 电量和水量历史分别入库，并在查询时合并返回。
- 仪表盘展示近 14 天电量/水量日变化趋势。
- 通知渠道支持企业微信 Webhook 和邮件，保存时可发送测试通知。
- 管理后台支持用户分页搜索、重置密码、删除用户、关闭用户 TOTP、查看/触发宿舍同步。
- `/health` 健康检查返回服务、数据库和目标水电系统连通状态。
- 新增配置解析单元测试，覆盖 HOCON 注释和环境变量覆盖逻辑。

---

## API 调整

| 路径 | 说明 |
|------|------|
| `PATCH /api/user/profile` | 更新个人信息 |
| `POST /api/user/change-password` | 修改密码 |
| `GET /api/user/totp/setup` | 生成 TOTP 配置 URI |
| `POST /api/user/totp/enable` | 启用 TOTP |
| `POST /api/user/totp/disable` | 关闭 TOTP |
| `GET /api/user/channel` | 获取通知渠道 |
| `PUT /api/user/channel` | 更新通知渠道 |
| `POST /api/admin/users/:id/disable-totp` | 管理员关闭用户 TOTP |
| `GET /health` | 健康检查 |

---

## 升级提醒

升级前请备份 `application.conf` 和数据库文件。v2.1.0 会自动迁移历史表，但生产环境仍建议先在备份环境验证启动和查询流程。
