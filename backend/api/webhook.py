from flask import request, jsonify
from requestNum import check_power
from utils import logger
from utils.configManager import ConfigManager
from utils.dataManager import (
    save_power_data,
    add_binding,
    remove_binding,
    get_binding,
)
from utils.senderManager import send_via_wechat, send_notification

logger = logger.setup_logger()
ConfigManager.init_config()
config = ConfigManager.get_config()
threshold = config.get('alert_threshold', 20)

def register_routes(app):
    """Register webhook route on given Flask app (avoid creating app here)."""

    def _webhook():
        data = request.get_json(force=True, silent=True) or {}
        logger.info(f"收到 Webhook 请求: {data}")

        # 兼容企业微信格式
        msgtype = data.get('msgtype')
        if msgtype == 'text':
            message = data.get('text', {}).get('content', '')
            user_id = data.get('from_user') or data.get('user_id')
        else:
            message = data.get('message') or ''
            user_id = data.get('user_id')

        if not user_id or not message:
            return jsonify({'ok': False, 'error': 'invalid payload'}), 400

        # 命令解析
        text = message.strip()
        reply = ""

        if text.startswith('/bind'):
            parts = text.split()
            if len(parts) < 2:
                reply = "❌ 绑定命令格式错误，用法：/bind <宿舍号> [邮箱]"
            else:
                dorm = parts[1]
                email = parts[2] if len(parts) >= 3 else None
                success = add_binding(user_id, dorm, email=email)
                reply = f"✅ 已绑定宿舍 {dorm}" if success else "❌ 绑定失败"
                # 如果绑定成功，发送一条带 @ 的确认通知给用户
                if success:
                    # 在企业微信推送中同时在文本内包含 @user，以便在页面上直接可见
                    mention_body = f"@{user_id} 已为您绑定宿舍 {dorm}，接下来将为您推送电量提醒。"
                    # 通过企业微信机器人 @ 用户进行确认（mention_list 保留以供企业微信使用）
                    send_via_wechat("绑定确认", mention_body, mention_list=[user_id])

                    # 绑定成功后立即尝试获取该宿舍的电量并保存、告警（内联告警逻辑，避免与 main 循环依赖）
                    try:
                        parts = dorm.split('-') if dorm else []
                        pc = config.get('power_checker', {})
                        default_building = pc.get('building')
                        default_floor = pc.get('floor')
                        default_room = pc.get('room')

                        if len(parts) >= 3:
                            building, floor, room = parts[0], parts[1], parts[2]
                        elif len(parts) == 2:
                            building, floor = parts[0], parts[1]
                            room = default_room
                        elif len(parts) == 1 and parts[0]:
                            single = parts[0]
                            if len(single) >= 6:
                                building = single[0:2]
                                floor = single[0:4]
                                room = single[0:6]
                            else:
                                building, floor, room = default_building, default_floor, single
                        else:
                            building, floor, room = default_building, default_floor, default_room

                        logger.info(f"绑定立即查询 -> 楼栋={building}, 楼层={floor}, 房间={room} (原始绑定: {dorm})")
                        remaining = check_power(building=building, floor=floor, room=room)
                        if remaining:
                            save_power_data(remaining, dorm=dorm)
                            # 内联告警：若低于阈值，通过配置的发送器通知用户与邮箱
                            try:
                                rp = float(remaining)
                                if rp < config.get('alert_threshold', 20):
                                    subject = "电量告警 | 剩余电量过低"
                                    body = f"宿舍 {dorm} 的当前剩余电量为 {remaining} 度，低于阈值 {config.get('alert_threshold')} 度。"
                                    recv_emails = [email] if email else None
                                    mention = [user_id]
                                    send_notification(subject, body, receiver_emails=recv_emails, mention_list=mention)
                            except Exception:
                                logger.exception("绑定立即查询：告警判断失败")
                    except Exception as e:
                        logger.error(f"绑定后立即查询电量失败: {e}")

        elif text.startswith('/unbind'):
            success = remove_binding(user_id)
            reply = "✅ 已解除绑定" if success else "❌ 解绑失败"

        elif text.startswith('/status'):
            b = get_binding(user_id)
            if b:
                reply = f"📊 当前绑定宿舍：{b['dorm']} | 邮箱：{b['email'] or '无'}"
            else:
                reply = "⚠️ 当前未绑定任何宿舍"

        else:
            reply = "🤖 可用命令：/bind /unbind /status"

        # 不再通过企业微信重复推送命令结果（避免重复发送）；
        # 将可显示的回复和显式 mention 一并返回给调用方，供页面显示。
        return jsonify({'ok': True, 'reply': reply, 'mention': f"@{user_id}"})

    # bind the route to the provided app
    app.add_url_rule('/webhook', 'webhook', _webhook, methods=['POST'])
