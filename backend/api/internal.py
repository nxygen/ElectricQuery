"""内部 API endpoints

这些 endpoints 供 worker 进程调用，用于查询电量和获取绑定信息。
"""

from flask import Blueprint, jsonify, request
from requestNum import check_power
from utils.dataManager import get_all_bindings, save_power_data
from utils.configManager import ConfigManager
from utils.senderManager import send_notification as sender_send_notification
import logging

bp = Blueprint('internal', __name__, url_prefix='/api')

config = ConfigManager.get_config()
logger = logging.getLogger(__name__)

def _check_internal_token():
    """简单的内部 token 验证，用于保护内部 API

    - 优先使用 HTTP 头 X-Internal-Token
    - 其次可通过查询参数 token 传入（兼容性）
    - 若配置中未设置 token，则视为不启用校验（仅限开发环境）
    """
    expected = config.get('internal_api_token')
    if not expected:
        # 未设置 token，允许访问但记录警告
        logger.warning("internal_api_token 未设置：内部 API 校验未启用（仅建议用于开发）")
        return True
    from flask import request
    token = request.headers.get('X-Internal-Token') or request.args.get('token')
    return token == expected


def parse_dorm(dorm):
    """解析宿舍号为楼栋、楼层、房间号"""
    parts = dorm.split('-') if dorm else []
    pc = config.get('power_checker', {})
    default_building = pc.get('building')
    default_floor = pc.get('floor')
    default_room = pc.get('room')

    if len(parts) >= 3:
        return parts[0], parts[1], parts[2]
    elif len(parts) == 2:
        return parts[0], parts[1], default_room
    elif len(parts) == 1 and parts[0]:
        single = parts[0]
        if len(single) >= 6:
            return single[0:2], single[0:4], single[0:6]
        else:
            return default_building, default_floor, single
    else:
        return default_building, default_floor, default_room


@bp.route('/power/<dorm>')
def get_power(dorm):
    """获取指定宿舍的电量"""
    try:
        building, floor, room = parse_dorm(dorm)
        power = check_power(building=building, floor=floor, room=room)
        if power:
            save_power_data(power, dorm=dorm)
        return jsonify({
            'ok': True,
            'power': power,
            'dorm': dorm,
            'parsed': {
                'building': building,
                'floor': floor,
                'room': room
            }
        })
    except Exception as e:
        return jsonify({
            'ok': False,
            'error': str(e)
        }), 500


@bp.route('/bindings')
def list_bindings():
    """获取所有绑定信息"""
    try:
        bindings = get_all_bindings()
        return jsonify({
            'ok': True,
            'bindings': bindings
        })
    except Exception as e:
        return jsonify({
            'ok': False,
            'error': str(e)
        }), 500


@bp.route('/notify', methods=['POST'])
def send_notification():
    """发送通知（支持邮件和企业微信）"""
    try:
        data = request.get_json(force=True)
        subject = data.get('subject')
        body = data.get('body')
        receiver_emails = data.get('receiver_emails', [])
        mention_list = data.get('mention_list', [])

        # 验证必需参数
        if not subject or not body:
            return jsonify({
                'ok': False,
                'error': 'missing required fields: subject, body'
            }), 400

        # 验证内部 token
        if not _check_internal_token():
            return jsonify({'ok': False, 'error': 'unauthorized'}), 401

        # 使用 sender 发送通知（代理到 senderManager）
        sender_send_notification(
            subject=subject,
            body=body,
            receiver_emails=receiver_emails if receiver_emails else None,
            mention_list=mention_list if mention_list else None
        )

        return jsonify({
            'ok': True,
            'message': 'notification sent'
        })
    except Exception as e:
        return jsonify({
            'ok': False,
            'error': str(e)
        }), 500