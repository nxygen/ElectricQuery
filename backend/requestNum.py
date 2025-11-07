import requests
import logging
from bs4 import BeautifulSoup
from utils.logger import setup_logger
from utils.configManager import ConfigManager

logger = setup_logger()

config = ConfigManager.get_config()

power_cfg = config['power_checker']

session = requests.Session()
session.headers.update({'User-Agent': power_cfg['user_agent']})

url = power_cfg['login_url']

def get_viewstate(html):
    soup = BeautifulSoup(html, 'html.parser')
    try:
        viewstate = soup.find('input', {'name': '__VIEWSTATE'})['value']
        viewstategenerator = soup.find('input', {'name': '__VIEWSTATEGENERATOR'})['value']
        return viewstate, viewstategenerator
    except (TypeError, KeyError):
        logger.error("无法提取 __VIEWSTATE 或 __VIEWSTATEGENERATOR")
        raise

def post_event(html, event_target, extra_fields):
    viewstate, viewstategenerator = get_viewstate(html)
    data = {
        '__VIEWSTATE': viewstate,
        '__VIEWSTATEGENERATOR': viewstategenerator,
        '__EVENTTARGET': event_target,
        '__EVENTARGUMENT': '',
        'radio': 'buyR',
        'ImageButton1.x': '10',
        'ImageButton1.y': '10',
    }
    data.update(extra_fields)
    return session.post(url, data=data)

def check_power(building=None, floor=None, room=None):
    building = building
    floor = floor
    room = room

    logger.info(f"开始获取电量（楼栋={building}, 楼层={floor}, 房间={room}）")

    try:
        resp1 = session.get(url)
        resp2 = post_event(resp1.text, 'drlouming', {'drlouming': building})
        resp3 = post_event(resp2.text, 'ablou', {'drlouming': building, 'ablou': floor})
        resp4 = post_event(resp3.text, 'drceng', {'drlouming': building, 'ablou': floor, 'drceng': room})

        soup = BeautifulSoup(resp4.text, 'html.parser')
        h6_tags = soup.find_all('h6')
        
        # 获取 h6 标签数量
        num_tags = len(h6_tags)
        if num_tags == 0:
            logger.warning(f"未找到任何 h6 标签（楼号：{building}）")
            return None
            
        # 13-14楼有两个标签用第二个，1-12楼只有一个标签就用第一个
        tag_index = 1 if num_tags >= 2 else 0
        
        spans = h6_tags[tag_index].find_all('span', {'class': 'number orange'})
        if len(spans) >= 3:
            remaining = spans[2].text.strip()
            logger.info(f"获取成功，剩余电量：{remaining} 度（楼号：{building}，标签索引：{tag_index}，总标签数：{num_tags}）")
            return remaining
        else:
            logger.warning(f"未找到足够的 <span> 标签（楼号：{building}，标签索引：{tag_index}，总标签数：{num_tags}）")
            return None
    except Exception as e:
        logger.error(f"查询出错: {e}")

    return None
