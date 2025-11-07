import os
import yaml
from utils.logger import setup_logger

logger = setup_logger()

class ConfigManager:
    _config = None
    _config_path = 'config.yml'

    _default_config = {
        'db_name': 'data/power_history.db',
        'alert_threshold': 20,
        'weekly_report_day': 0,
        'poll_interval': 600,  # seconds between periodic checks
        'enabled_senders': ['email'],
        'smtp': {
            'server': "smtp.163.com",
            'port': 465,
            'sender_email': "awesome@163.com",
            'password': "",  # 注意安全处理密码
            'sender_name': 'Bot',
            'receiver_emails': ["awesome@163.com"]
        },
        'wechat': {
            'mention_list': ["@all"],
            'webhook_url': "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxxxxxxxxxxxx"
        },
        'power_checker': {
            'login_url': 'http://ydgl.xzcit.cn/web/Default.aspx',
            'user_agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/114.0.0.0 Safari/537.36',
            'building': '01',
            'floor': '0101',
            'room': '010101',
            'post_data': {}
        }
    }

    @classmethod
    def init_config(cls):
        if not os.path.exists(cls._config_path):
            try:
                with open(cls._config_path, 'w', encoding='utf-8') as f:
                    yaml.dump(cls._default_config, f, allow_unicode=True)
                logger.info(f"默认配置文件已创建: {cls._config_path}")
            except Exception as e:
                logger.error(f"创建默认配置文件失败: {e}")
                raise

    @classmethod
    def load_config(cls):
        if cls._config is None:
            if not os.path.exists(cls._config_path):
                logger.warning(f"配置文件不存在: {cls._config_path}，尝试初始化默认配置")
                cls.init_config()
            try:
                with open(cls._config_path, 'r', encoding='utf-8') as f:
                    cls._config = yaml.safe_load(f)
                    logger.info("配置文件加载成功")
            except Exception as e:
                logger.error(f"加载配置文件失败: {e}")
                raise
        return cls._config

    @classmethod
    def get_config(cls):
        if cls._config is None:
            return cls.load_config()
        return cls._config
