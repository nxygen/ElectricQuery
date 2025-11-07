"""后台轮询任务 (daemon)

此模块负责周期性检查电量并通过后端 API 进行数据持久化与通知发送。
Worker 只做调度与调用后端 HTTP 接口。

运行方式:
    python -m worker.runner
"""

import asyncio
import logging
import aiohttp
from datetime import datetime
import os

from utils.configManager import ConfigManager

# 配置加载与日志设置
ConfigManager.init_config()
config = ConfigManager.get_config()
logger = logging.getLogger(__name__)

# 轮询间隔与阈值（从配置读取）
POLL_INTERVAL = config.get('poll_interval', 600)  # 默认 10 分钟
THRESHOLD = config.get('alert_threshold', 20)     # 默认 20 度
REPORT_DAY = config.get('weekly_report_day', 0)   # 默认周一
REPORT_HOUR = config.get('report_hour', 8)        # 默认早 8 点

# 后端 API 地址与内部 token
BACKEND_URL = os.getenv('BACKEND_URL', 'http://localhost:5000')
BACKEND_TOKEN = os.getenv('BACKEND_INTERNAL_TOKEN') or config.get('internal_api_token')


def _auth_headers():
    headers = {}
    if BACKEND_TOKEN:
        headers['X-Internal-Token'] = BACKEND_TOKEN
    return headers


async def fetch_bindings():
    """从后端 API 获取所有绑定信息"""
    async with aiohttp.ClientSession() as session:
        url = f"{BACKEND_URL}/api/bindings"
        try:
            async with session.get(url, headers=_auth_headers()) as response:
                data = await response.json()
                if data.get('ok'):
                    return data.get('bindings', [])
                else:
                    logger.error(f"获取绑定信息失败：{data.get('error')}")
                    return []
        except Exception as e:
            logger.error(f"调用绑定 API 出错：{e}")
            return []


async def fetch_power(dorm):
    """从后端 API 获取指定宿舍的电量"""
    async with aiohttp.ClientSession() as session:
        url = f"{BACKEND_URL}/api/power/{dorm}"
        try:
            async with session.get(url, headers=_auth_headers()) as response:
                data = await response.json()
                if data.get('ok'):
                    return data.get('power')
                else:
                    logger.error(f"获取电量失败：{data.get('error')}")
                    return None
        except Exception as e:
            logger.error(f"调用电量 API 出错：{e}")
            return None


async def send_notification(subject, body, receiver_emails=None, mention_list=None):
    """通过后端 API 发送通知（邮件/企业微信统一由后端处理）"""
    async with aiohttp.ClientSession() as session:
        url = f"{BACKEND_URL}/api/notify"
        payload = {
            'subject': subject,
            'body': body,
            'receiver_emails': receiver_emails or [],
            'mention_list': mention_list or []
        }
        try:
            async with session.post(url, json=payload, headers=_auth_headers()) as response:
                data = await response.json()
                if data.get('ok'):
                    return True
                else:
                    logger.error(f"发送通知失败：{data.get('error')}")
                    return False
        except Exception as e:
            logger.error(f"调用通知 API 出错：{e}")
            return False


async def async_worker():
    """异步轮询主循环：获取绑定、查询电量、并在需要时通过后端发送告警"""
    logger.info("后台轮询任务已启动...")

    while True:
        try:
            bindings = await fetch_bindings()
            if not bindings:
                logger.info("当前无绑定记录")
            else:
                logger.info(f"开始查询 {len(bindings)} 个绑定宿舍的电量")

            for binding in bindings:
                dorm = binding.get('dorm')
                user_id = binding.get('user_id')
                email = binding.get('email')

                try:
                    remaining = await fetch_power(dorm)
                    if remaining:
                        logger.info(f"宿舍 {dorm} 电量：{remaining} 度")
                        try:
                            rp = float(remaining)
                            if rp < THRESHOLD:
                                subject = "电量告警 | 剩余电量过低"
                                body = f"宿舍 {dorm} 的当前剩余电量为 {remaining} 度，低于阈值 {THRESHOLD} 度，请及时充值。"
                                ok = await send_notification(subject, body, receiver_emails=[email] if email else None, mention_list=[user_id] if user_id else None)
                                if ok:
                                    logger.info(f"已发送电量过低告警：{dorm} -> {remaining}")
                        except ValueError:
                            logger.warning(f"电量数据 {remaining} 无法转换为浮点数")
                except Exception as e:
                    logger.error(f"处理宿舍 {dorm} 时出错：{e}")
                    continue

                await asyncio.sleep(2)

            now = datetime.now()
            if now.weekday() == REPORT_DAY and now.hour == REPORT_HOUR and now.minute < 10:
                await send_weekly_report()

        except Exception as e:
            logger.error(f"轮询主循环发生错误：{e}")

        await asyncio.sleep(POLL_INTERVAL)


async def send_weekly_report():
    """发送周度用电报告（通过后端 API 派发）"""
    try:
        subject = "宿舍用电周报"
        body = "本周用电情况统计..."
        bindings = await fetch_bindings()
        for binding in bindings:
            email = binding.get('email')
            user_id = binding.get('user_id')
            await send_notification(subject, body, receiver_emails=[email] if email else None, mention_list=[user_id] if user_id else None)
        logger.info("已发送本周用电报告")
    except Exception as e:
        logger.error(f"发送周报时出错：{e}")


if __name__ == '__main__':
    try:
        asyncio.run(async_worker())
    except KeyboardInterrupt:
        logger.info("收到退出信号，正在停止...")