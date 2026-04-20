from flask import Blueprint, request, jsonify
from requestNum import check_power
from utils import logger
from utils.configManager import ConfigManager
from utils.dataManager import (
    add_binding,
    remove_binding,
    get_binding,
    get_all_bindings,
    save_power_data,
)
from utils.senderManager import send_notification
from api.auth import require_admin_token

logger = logger.setup_logger()
ConfigManager.init_config()
config = ConfigManager.get_config()
threshold = config.get('alert_threshold', 20)

public_bp = Blueprint('api', __name__, url_prefix='/api')


# 企业微信机器人无法直接监听用户消息，故不再在 webhook 中解析文本命令。
#
# 目前对外 API 仅提供结构化接口：
#   POST /api/bind    -> JSON { user_id, dorm, email? }
#   POST /api/unbind  -> JSON { user_id }
#   GET  /api/status  -> ?user_id=...
#
# 如果将来需要接收第三方 webhook（例如企业微信回调），可以单独添加一个路由来适配
# 该回调格式并调用相应的绑定逻辑。此处保持接口简洁与结构化，便于前端与运维使用。


@public_bp.route('/bind', methods=['POST'])
def api_bind():
    """显式的绑定接口，供前端调用：POST JSON {user_id, dorm, email?}"""
    data = request.get_json(force=True, silent=True) or {}
    user_id = data.get('user_id')
    dorm = data.get('dorm')
    email = data.get('email')
    if not user_id or not dorm:
        return jsonify({'ok': False, 'error': 'missing user_id or dorm'}), 400
    ok = add_binding(user_id, dorm, email=email)
    # 绑定成功后，立即尝试查询当前电量并保存到历史
    if ok:
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

            remaining = check_power(building=building, floor=floor, room=room)
            if remaining:
                # 保存电量历史
                save_power_data(remaining, dorm=dorm)
                # 如果低于阈值则发送告警
                try:
                    rp = float(remaining)
                    if rp < config.get('alert_threshold', 20):
                        subject = "电量告警 | 剩余电量过低"
                        body = f"宿舍 {dorm} 的当前剩余电量为 {remaining} 度，低于阈值 {config.get('alert_threshold')} 度。"
                        recv_emails = [email] if email else None
                        mention = [user_id]
                        send_notification(subject, body, receiver_emails=recv_emails, mention_list=mention)
                except Exception:
                    logger.exception("绑定后立即查询：告警判断失败")
        except Exception as e:
            logger.error(f"绑定后立即查询电量失败: {e}")

    return jsonify({'ok': ok})


@public_bp.route('/unbind', methods=['POST'])
def api_unbind():
    """显式的解除绑定接口：POST JSON {user_id}"""
    data = request.get_json(force=True, silent=True) or {}
    user_id = data.get('user_id') or request.form.get('user_id') or request.args.get('user_id')
    if not user_id:
        return jsonify({'ok': False, 'error': 'missing user_id'}), 400
    ok = remove_binding(user_id)
    return jsonify({'ok': ok})


@public_bp.route('/status', methods=['GET'])
def api_status():
    """查询某个 user_id 的绑定信息：GET /api/status?user_id=..."""
    user_id = request.args.get('user_id')
    if not user_id:
        return jsonify({'ok': False, 'error': 'missing user_id'}), 400
    b = get_binding(user_id)
    if b:
        return jsonify({'ok': True, 'binding': b})
    else:
        return jsonify({'ok': False, 'error': 'not bound'}), 404

@public_bp.route('/bindings', methods=['GET'])
@require_admin_token
def api_bindings():
    """返回系统中所有绑定记录，供前端查询列表用"""
    try:
        bindings = get_all_bindings()
        return jsonify({'ok': True, 'bindings': bindings})
    except Exception as e:
        return jsonify({'ok': False, 'error': str(e)}), 500


def register_routes(app):
    """Register public API blueprint on provided Flask app."""
    app.register_blueprint(public_bp)
