import base64
import hashlib
import os
import smtplib

import requests
from utils.logger import setup_logger
from email.mime.text import MIMEText
from email.mime.image import MIMEImage
from email.mime.multipart import MIMEMultipart
from utils.configManager import ConfigManager

logger = setup_logger()

class BaseSender:
    def send(self, subject, body):
        raise NotImplementedError("子类必须实现 send 方法")

class EmailSender(BaseSender):
    def __init__(self, config):
        self.config = config
    
    def send(self, subject, body, image_path=None, receiver_emails=None):
        smtp_config = self.config['smtp']
        try:
            server = smtplib.SMTP_SSL(smtp_config['server'], smtp_config['port'])
            server.login(smtp_config['sender_email'], smtp_config['password'])
            targets = receiver_emails if receiver_emails else smtp_config.get('receiver_emails', [])
            if isinstance(targets, str):
                targets = [targets]
            for receiver_email in targets:
                msg = MIMEMultipart('related')
                msg['From'] = f"{smtp_config['sender_name']} <{smtp_config['sender_email']}>"
                msg['To'] = receiver_email
                msg['Subject'] = subject

                msg.attach(MIMEText(body, 'plain'))
                # 添加图片
                if image_path and os.path.exists(image_path):
                    with open(image_path, 'rb') as f:
                        img = MIMEImage(f.read())
                        img.add_header('Content-Disposition', 'attachment', filename=os.path.basename(image_path))
                        msg.attach(img)
                text = msg.as_string()
                server.sendmail(smtp_config['sender_email'], receiver_email, text)
                logger.info(f"邮件已成功发送给 {receiver_email}")
            logger.info("所有邮件发送成功！")
        except Exception as e:
            logger.error(f"邮件发送失败: {e}")
        finally:
            try:
                server.quit()
            except:
                pass

class WeChatSender:
    def __init__(self, config):
        self.webhook_url = config.get('wechat', {}).get('webhook_url')
        self.mention_list = config.get('wechat', {}).get('mention_list', [])
    def send(self, subject, body, image_path=None, mention_list=None):
        if not self.webhook_url:
            logger.warning("未配置企业微信机器人 Webhook URL，跳过发送")
            return

        content = f"【{subject}】\n{body}"
        text_data = {"msgtype": "text", "text": {"content": content}}

        use_list = mention_list if mention_list is not None else self.mention_list
        if use_list:
            mobiles = [m for m in use_list if isinstance(m, str) and (m.isdigit() or (m.startswith("+") and m[1:].isdigit()))]
            users = [u for u in use_list if not (isinstance(u, str) and (u.isdigit() or (u.startswith("+") and u[1:].isdigit())))]

            if mobiles:
                text_data["text"]["mentioned_mobile_list"] = mobiles
            if users:
                text_data["text"]["mentioned_list"] = users

        try:
            requests.post(self.webhook_url, json=text_data)
        except Exception as e:
            logger.error(f"企业微信文本发送异常: {e}")

        # 再发送图片（如果提供）
        if image_path and os.path.exists(image_path):
            try:
                with open(image_path, 'rb') as f:
                    image_data = f.read()
                    base64_str = base64.b64encode(image_data).decode('utf-8')
                    md5_str = hashlib.md5(image_data).hexdigest()

                image_payload = {
                    "msgtype": "image",
                    "image": {
                        "base64": base64_str,
                        "md5": md5_str
                    }
                }
                response = requests.post(self.webhook_url, json=image_payload)
                if response.status_code == 200:
                    logger.info("企业微信图片发送成功")
                else:
                    logger.error(f"企业微信图片发送失败，状态码: {response.status_code}")
            except Exception as e:
                logger.error(f"企业微信发送图片异常: {e}")

class SenderManager:
    def __init__(self, config):
        self.config = config
        self.enabled_senders = config.get('enabled_senders', [])
        self.senders = []
        self._init_senders()

    def _init_senders(self):
        # 根据配置决定启用哪些发送方式
        for sender_name in self.enabled_senders:
            if sender_name == 'email':
                self.senders.append(EmailSender(self.config))
            elif sender_name == 'wechat':
                self.senders.append(WeChatSender(self.config))
            else:
                logger.warning(f"未知的发送器类型: {sender_name}")

    def send_all(self, subject, body, image_path=None, receiver_emails=None, mention_list=None):
        if not self.senders:
            logger.warning("没有启用任何发送方式，通知未发送")
            return
        for sender in self.senders:
            # 根据 sender 的类型，传递相应的可选参数
            try:
                if isinstance(sender, EmailSender):
                    sender.send(subject, body, image_path=image_path, receiver_emails=receiver_emails)
                elif isinstance(sender, WeChatSender):
                    sender.send(subject, body, image_path=image_path, mention_list=mention_list)
                else:
                    # fallback
                    sender.send(subject, body, image_path)
            except Exception as e:
                logger.error(f"发送器 {sender} 发送失败: {e}")

def send_notification(subject, body, image_path=None, receiver_emails=None, mention_list=None):
    config = ConfigManager.get_config()
    manager = SenderManager(config)
    manager.send_all(subject, body, image_path=image_path, receiver_emails=receiver_emails, mention_list=mention_list)


def send_via_email(subject, body, image_path=None, receiver_emails=None):
    """直接只通过邮件发送（不触及企业微信）。"""
    config = ConfigManager.get_config()
    email_sender = EmailSender(config)
    email_sender.send(subject, body, image_path=image_path, receiver_emails=receiver_emails)


def send_via_wechat(subject, body, image_path=None, mention_list=None):
    """直接只通过企业微信机器人发送（不触及邮件）。"""
    config = ConfigManager.get_config()
    wechat_sender = WeChatSender(config)
    wechat_sender.send(subject, body, image_path=image_path, mention_list=mention_list)