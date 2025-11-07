import sqlite3
import os
from datetime import datetime
from utils.logger import setup_logger
from utils.configManager import ConfigManager

logger = setup_logger()
config = ConfigManager.get_config()

DB_PATH = config.get('db_name', 'data/power_history.db')


def init_db():
    # ensure directory exists
    db_dir = os.path.dirname(DB_PATH)
    if db_dir and not os.path.exists(db_dir):
        os.makedirs(db_dir, exist_ok=True)

    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()

    # 如果表不存在，则创建带 dorm 字段的新表
    cursor.execute("""
        SELECT name FROM sqlite_master WHERE type='table' AND name='power_log'
    """)
    exists = cursor.fetchone()
    if not exists:
        cursor.execute('''
            CREATE TABLE power_log (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                date TEXT NOT NULL,
                dorm TEXT NOT NULL,
                remaining_power TEXT NOT NULL,
                UNIQUE(date, dorm)
            )
        ''')
        logger.info("创建新的 power_log 表（包含 dorm 字段）")
    else:
        # 如果表已存在，检查是否含有 dorm 列；若无则添加该列（兼容老版本）
        cursor.execute("PRAGMA table_info(power_log)")
        cols = [r[1] for r in cursor.fetchall()]
        if 'dorm' not in cols:
            try:
                cursor.execute("ALTER TABLE power_log ADD COLUMN dorm TEXT DEFAULT ''")
                conn.commit()
                logger.info("已为现有 power_log 表添加 dorm 列（兼容旧 schema）")
            except Exception as e:
                logger.error(f"为 power_log 添加 dorm 列失败: {e}")

    # 绑定表：user_id -> dorm, email（可选）
    cursor.execute("""
        SELECT name FROM sqlite_master WHERE type='table' AND name='bindings'
    """)
    exists_bind = cursor.fetchone()
    if not exists_bind:
        cursor.execute('''
            CREATE TABLE bindings (
                user_id TEXT PRIMARY KEY,
                dorm TEXT NOT NULL,
                email TEXT,
                created_at TEXT NOT NULL
            )
        ''')
        logger.info("创建 bindings 表")
    else:
        # 检查 email 列是否存在
        cursor.execute("PRAGMA table_info(bindings)")
        bcols = [r[1] for r in cursor.fetchall()]
        if 'email' not in bcols:
            try:
                cursor.execute("ALTER TABLE bindings ADD COLUMN email TEXT DEFAULT NULL")
                conn.commit()
                logger.info("已为 bindings 表添加 email 列（兼容旧 schema）")
            except Exception as e:
                logger.error(f"为 bindings 添加 email 列失败: {e}")

    conn.commit()
    conn.close()


def save_power_data(remaining_power, date_str=None, dorm=None):
    if date_str is None:
        date_str = datetime.now().strftime('%Y-%m-%d')
    if dorm is None:
        # 使用默认配置中的宿舍作为回退
        pc = config.get('power_checker', {})
        dorm = f"{pc.get('building','')}-{pc.get('floor','')}-{pc.get('room','')}"

    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    try:
        cursor.execute('''
            INSERT OR REPLACE INTO power_log (date, dorm, remaining_power) VALUES (?, ?, ?)
        ''', (date_str, dorm, str(remaining_power)))
        conn.commit()
        logger.info(f"成功保存电量数据：{date_str} | {dorm} -> {remaining_power}")
    except Exception as e:
        logger.error(f"保存电量数据失败: {e}")
    finally:
        conn.close()


def get_recent_power_logs(limit=3, dorm=None):
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    try:
        if dorm:
            cursor.execute('''
                SELECT date, remaining_power FROM power_log
                WHERE dorm = ?
                ORDER BY date DESC
                LIMIT ?
            ''', (dorm, limit))
        else:
            cursor.execute('''
                SELECT date, remaining_power FROM power_log
                ORDER BY date DESC
                LIMIT ?
            ''', (limit,))
        return cursor.fetchall()
    except Exception as e:
        logger.error(f"查询电量记录失败: {e}")
        return []
    finally:
        conn.close()


def get_latest_power(dorm=None):
    logs = get_recent_power_logs(limit=1, dorm=dorm)
    if logs:
        try:
            return float(logs[0][1])
        except Exception:
            return None
    return None


# 绑定相关函数
def add_binding(user_id, dorm, email=None):
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    created_at = datetime.now().isoformat()
    try:
        cursor.execute('''
            INSERT OR REPLACE INTO bindings (user_id, dorm, email, created_at) VALUES (?, ?, ?, ?)
        ''', (user_id, dorm, email, created_at))
        conn.commit()
        logger.info(f"已绑定 user {user_id} -> {dorm}")
        return True
    except Exception as e:
        logger.error(f"绑定失败: {e}")
        return False
    finally:
        conn.close()


def remove_binding(user_id):
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    try:
        cursor.execute('DELETE FROM bindings WHERE user_id = ?', (user_id,))
        conn.commit()
        logger.info(f"已移除绑定 user {user_id}")
        return True
    except Exception as e:
        logger.error(f"移除绑定失败: {e}")
        return False
    finally:
        conn.close()


def get_binding(user_id):
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    try:
        cursor.execute('SELECT user_id, dorm, email, created_at FROM bindings WHERE user_id = ?', (user_id,))
        row = cursor.fetchone()
        if row:
            return {'user_id': row[0], 'dorm': row[1], 'email': row[2], 'created_at': row[3]}
        return None
    except Exception as e:
        logger.error(f"查询绑定失败: {e}")
        return None
    finally:
        conn.close()


def get_all_bindings():
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    try:
        cursor.execute('SELECT user_id, dorm, email, created_at FROM bindings')
        rows = cursor.fetchall()
        return [{'user_id': r[0], 'dorm': r[1], 'email': r[2], 'created_at': r[3]} for r in rows]
    except Exception as e:
        logger.error(f"获取全部绑定失败: {e}")
        return []
    finally:
        conn.close()
