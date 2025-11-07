import logging
import os
from datetime import datetime, timedelta

LOG_DIR = 'logs'
os.makedirs(LOG_DIR, exist_ok=True)


def clean_old_logs(days=3):
    now = datetime.now()
    for filename in os.listdir(LOG_DIR):
        if filename.endswith(".log"):
            try:
                date_str = filename.replace(".log", "")
                file_date = datetime.strptime(date_str, "%Y-%m-%d")
                if (now - file_date).days > days:
                    os.remove(os.path.join(LOG_DIR, filename))
            except Exception:
                continue


def setup_logger():
    clean_old_logs()
    today = datetime.now().strftime('%Y-%m-%d')
    log_file = os.path.join(LOG_DIR, f"{today}.log")

    logger = logging.getLogger("ElectricQuery")
    logger.setLevel(logging.INFO)

    if not logger.handlers:
        # 文件处理器
        fh = logging.FileHandler(log_file, encoding='utf-8')
        formatter = logging.Formatter('[%(asctime)s] [%(levelname)s] %(message)s')
        fh.setFormatter(formatter)
        logger.addHandler(fh)

        # 控制台处理器
        ch = logging.StreamHandler()
        ch.setFormatter(formatter)
        logger.addHandler(ch)

    return logger
