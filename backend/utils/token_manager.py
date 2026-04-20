"""Token 生成与校验工具。

提供基于 HMAC-SHA256 的时间绑定 token 生成与验证函数。格式为:
    <timestamp>.<hmac_hex>

服务端保存一个秘密（如 config 中的 admin_api_token），客户端可以使用该秘密生成短期 token，
后端通过验证 HMAC 与时间戳来确认 token 的有效性（并且限制最大有效期）。

注意：相比直接使用 SHA1，这里使用 HMAC-SHA256 更安全。
"""
from __future__ import annotations

import time
import hmac
import hashlib
from typing import Tuple


def generate_timebound_token(secret: str, at: int | None = None) -> str:
    """生成基于时间的 token。返回字符串格式："<ts>.<hmac_hex>"。

    secret: 共享秘密（例如配置中的 admin_api_token）
    at: 可选的时间戳（秒），默认使用当前时间
    """
    if at is None:
        at = int(time.time())
    ts = str(int(at))
    mac = hmac.new(secret.encode('utf-8'), ts.encode('utf-8'), hashlib.sha256).hexdigest()
    return f"{ts}.{mac}"


def verify_timebound_token(token: str, secret: str, max_age_seconds: int = 300) -> bool:
    """验证时间绑定 token 是否有效。

    - token 格式应为 "<ts>.<hmac_hex>"；
    - 验证 HMAC 是否匹配并且时间戳在允许范围内。

    返回 True/False。
    """
    try:
        ts_str, mac = token.split('.', 1)
        ts = int(ts_str)
    except Exception:
        return False

    now = int(time.time())
    if ts < 0 or ts > now + 60:
        # 时间戳不合理，未来时间超过 60 秒视为无效
        return False

    if now - ts > max_age_seconds:
        # 超过有效期
        return False

    expected = hmac.new(secret.encode('utf-8'), ts_str.encode('utf-8'), hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, mac)
