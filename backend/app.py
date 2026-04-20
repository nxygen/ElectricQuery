from datetime import datetime
from flask import Flask

from utils.dataManager import init_db
from utils.senderManager import send_notification
from utils.logger import setup_logger
from utils.configManager import ConfigManager
from api.webhook import register_routes as register_webhook_routes
from api.internal import bp as internal_bp
from api.auth import auth_bp, require_admin_token
# 注意：后台轮询已移到 worker/runner.py，生产环境应单独运行该进程

logger = setup_logger()
ConfigManager.init_config()
config = ConfigManager.get_config()

threshold = config.get('alert_threshold', 20)
report_day = config.get('weekly_report_day', 0)  # 0=Monday, 6=Sunday
poll_interval = config.get('poll_interval', 600)

app = Flask(__name__)

# 注册路由与蓝图
register_webhook_routes(app)  # 注册公开 API（/api/bind, /api/unbind, /api/status, /api/bindings）
app.register_blueprint(internal_bp)  # 注册内部 API（worker 调用，/api/internal/*）
# 注册认证蓝图（/api/auth）
try:
    app.register_blueprint(auth_bp)
except Exception:
    # 如果 auth_bp 无法导入或未配置，忽略并继续（会在 auth 模块内部记录）
    pass


def check_and_alert(remaining_power, dorm, user_id=None, user_email=None):
    try:
        rp = float(remaining_power)
    except Exception:
        logger.warning("电量数据无法转换为 float，跳过告警判断")
        return

    if rp < threshold:
        subject = "电量告警 | 剩余电量过低"
        user_info = f" (用户: {user_id})" if user_id else ""
        body = f"宿舍 {dorm} 的当前剩余电量为 {remaining_power} 度，低于设定阈值 {threshold} 度，请及时充值。{user_info}"
        recv_emails = [user_email] if user_email else None
        mention = [user_id] if user_id else None
        send_notification(subject, body, receiver_emails=recv_emails, mention_list=mention)
        logger.info(f"已发送电量过低告警：{dorm} -> {remaining_power}")
    else:
        logger.info(f"宿舍 {dorm} 电量在安全范围：{remaining_power} 度")


config = {
    'report': {
        'report_day': 0,   # 周一
        'report_hour': 8,  # 上午8点
    }
}

async def send_weekly_report_if_today(dorm, user_email=None, user_id=None):
    """
    异步版周报发送，每周一 08:00 整触发
    """
    now = datetime.now()
    report_day = config['report']['report_day']
    report_hour = config['report']['report_hour']

    if now.weekday() != report_day or now.hour != report_hour or now.minute != 0:
        return

    # background worker and report logic moved to worker.runner
    pass


def main():
    # 启动后台并运行 HTTP 服务
    init_db()
    app.run(host='0.0.0.0', port=5000)




if __name__ == '__main__':
    main()
