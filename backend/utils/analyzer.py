def compute_consumption(data_rows):
    """
    输入格式: [(date, remaining_power), ...]，返回计算后的结构
    """
    if len(data_rows) < 1:
        return None

    rows = sorted(data_rows, key=lambda x: x[0])
    results = []
    for i in range(len(rows)):
        curr_date, curr_power = rows[i]
        consumption = None
        if i > 0:
            try:
                prev_power = float(rows[i - 1][1])
                curr_power_float = float(curr_power)
                delta = round(curr_power_float - prev_power, 2)

                if delta > 0:
                    consumption = f"+{delta:.2f}"
                elif delta < 0:
                    consumption = f"{delta:.2f}"
                else:
                    consumption = "0.00"
            except Exception:
                consumption = None
        results.append({
            'date': curr_date,
            'remaining_power': f"{float(curr_power):.2f}",
            'consumption_since_prev_day': consumption
        })
    return results
