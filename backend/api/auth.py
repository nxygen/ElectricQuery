from functools import wraps
from flask import Blueprint, request, jsonify
from utils.configManager import ConfigManager
from utils import logger
from utils.token_manager import verify_timebound_token

ConfigManager.init_config()
config = ConfigManager.get_config()
log = logger.setup_logger()

auth_bp = Blueprint('auth', __name__, url_prefix='/api/auth')


def _get_admin_token():
    # 从配置读取管理员 token（若未配置则返回 None）
    return config.get('admin_api_token')


def require_admin_token(f):
    """装饰器：检查请求头 X-Admin-Token 或查询参数 token 是否匹配配置的 admin_api_token

    支持两种验证方式：
    1. 静态 token：直接匹配配置中的 admin_api_token
    2. 时间绑定 token：使用 HMAC-SHA256 签名的短期 token（格式：timestamp.hmac）

    如果未配置 admin_api_token，装饰器将拒绝所有请求（更安全），以强制部署时必须配置。
    """
    @wraps(f)
    def wrapped(*args, **kwargs):
        expected = _get_admin_token()
        if not expected:
            log.warning('admin_api_token 未配置：拒绝管理员接口访问')
            return jsonify({'ok': False, 'error': 'admin token not configured'}), 403

        token = request.headers.get('X-Admin-Token') or request.args.get('token') or (request.get_json(silent=True) or {}).get('token')
        
        # 先检查静态 token
        if token == expected:
            return f(*args, **kwargs)

        # 如果静态 token 不匹配，尝试时间绑定 token（短期 token）验证
        ttl = config.get('timebound_token_ttl', 300)  # 默认 5 分钟有效期
        try:
            if token and verify_timebound_token(token, expected, max_age_seconds=ttl):
                return f(*args, **kwargs)
        except Exception as e:
            # 验证时出现异常视为未授权
            log.debug(f'时间绑定 token 验证失败: {e}')

        return jsonify({'ok': False, 'error': 'unauthorized'}), 401

    return wrapped


@auth_bp.route('/verify', methods=['POST'])
def verify():
    """验证 token 是否有效：POST JSON {token: '...'} 或在 Header 使用 X-Admin-Token
    
    支持静态 token 或时间绑定 token（HMAC）验证。
    """
    expected = _get_admin_token()
    if not expected:
        log.warning('admin_api_token 未配置：/api/auth/verify 返回 403')
        return jsonify({'ok': False, 'error': 'admin token not configured'}), 403

    data = request.get_json(silent=True) or {}
    token = request.headers.get('X-Admin-Token') or data.get('token') or request.args.get('token')
    
    # 静态 token 验证
    if token == expected:
        return jsonify({'ok': True, 'type': 'static'})
    
    # 时间绑定 token 验证
    ttl = config.get('timebound_token_ttl', 300)
    if token and verify_timebound_token(token, expected, max_age_seconds=ttl):
        return jsonify({'ok': True, 'type': 'timebound'})
    
    return jsonify({'ok': False}), 401
