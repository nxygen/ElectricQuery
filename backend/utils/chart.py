import os
import matplotlib.pyplot as plt
from matplotlib import font_manager

# 设置字体
font_path = "fonts/SourceHanSansSC-Regular.otf"
if os.path.exists(font_path):
    font_manager.fontManager.addfont(font_path)
    font_name = font_manager.FontProperties(fname=font_path).get_name()
    plt.rcParams['font.family'] = font_name
    plt.rcParams['axes.unicode_minus'] = False
else:
    print("⚠️ 找不到中文字体文件，图表可能显示乱码")

def generate_power_plot(data, output_path="output/power_delta_report.png"):
    if not data:
        return None

    dates, deltas = [], []
    for record in data:
        try:
            delta_val = float(record.get("consumption_since_prev_day", 0))
            deltas.append(delta_val)
            dates.append(record["date"])
        except:
            continue

    if not dates or not deltas:
        return None

    plt.figure(figsize=(10, 6))
    plt.plot(dates, deltas, marker='o', linestyle='-', color='#2A6F9E', linewidth=2, label='每日电量变化（度）')

    # 点标注，正绿负红
    for x, y in zip(dates, deltas):
        color = '#5CB85C' if y >= 0 else '#D9534F'
        va = 'bottom' if y >= 0 else 'top'
        plt.text(x, y, f"{y:+.2f}", ha='center', va=va, fontsize=10, color=color)

    plt.title("近一周每日电量变化图", fontsize=16)
    plt.xlabel("日期", fontsize=13)
    plt.ylabel("电量变化（度）", fontsize=13)

    plt.axhline(0, color='#888888', linestyle='--', linewidth=1)  # 零线
    plt.grid(True, linestyle='--', linewidth=0.4, color='#CCCCCC')

    plt.xticks(rotation=45, fontsize=11)
    plt.yticks(fontsize=11)

    plt.tight_layout()
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    plt.savefig(output_path, dpi=150)
    plt.close()

    return output_path