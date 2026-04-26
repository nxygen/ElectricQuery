package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"electricquery/internal/checker"
	"electricquery/internal/config"
	"electricquery/internal/logger"
	"electricquery/internal/model"
	"electricquery/internal/notifier"
	"electricquery/internal/strutil"
)

// DormLookupResult 查询结果，包含映射表查出的完整物理信息
type DormLookupResult struct {
	Opt         model.DormOption // 映射表记录（含 DrcengValue/Building/Floor/Label）
	Building    string           // 楼栋代码（直接取自 FormValue 第一段）
	Floor       string           // 楼层代码（直接取自 FormValue 第二段）
	DrcengValue string           // 物理ID（直接取自 DormOption.DrcengValue，不做推算）
}

// LookupByFormValue 根据 FormValue 查映射表，返回物理ID和元信息
//
// 核心原则：物理ID是绝对真理。AI 不得对物理ID进行数学推算或逻辑猜测。
//
// 解析步骤：
//  1. 精确匹配 FormValue（三段式 "building|floor|drceng" 或普通6位数字）
//  2. 从匹配到的 DormOption 取出 DrcengValue（这就是网页查询的物理ID）
//  3. 直接返回 DrcengValue，不做任何数学运算
//
// AI 禁忌：
//  - 绝对禁止看到后两位 "69" 就认为是水表
//  - 绝对禁止尝试将 "110169" 转换回 "110132"
//  - 物理ID只是一个字符串钥匙，AI 不需要理解其含义
//
// 返回值：nil 表示未找到映射
func LookupByFormValue(formValue string) *DormLookupResult {
	formValue = strings.TrimSpace(formValue)
	if formValue == "" {
		return nil
	}

	var opt model.DormOption
	query := model.DB.Where("level = ?", model.OptionLevelRoom)

	// 优先：精确匹配 FormValue
	if err := query.Where("form_value = ?", formValue).First(&opt).Error; err != nil {
		// 次优：匹配 Label（兜底，用于兼容旧格式或前端传 Label 的场景）
		if err := query.Where("label = ?", formValue).First(&opt).Error; err != nil {
			logger.Debug("宿舍号未命中 DormOption 映射", "input", formValue)
			return nil
		}
		logger.Debug("宿舍号通过 label 命中映射", "label", formValue, "drceng_value", opt.DrcengValue)
	}

	// 提取 building 和 floor（从 FormValue 第一、二段）
	var building, floor string
	if parts := strings.Split(formValue, "|"); len(parts) == 3 {
		building, floor = parts[0], parts[1]
	} else if len(formValue) >= 2 {
		// 普通6位数字：building = 前2位
		building = formValue[:2]
		floor = formValue[:2] + formValue[2:4]
	}

	return &DormLookupResult{
		Opt:         opt,
		Building:    building,
		Floor:       floor,
		DrcengValue: opt.DrcengValue, // 物理ID：直接取自映射表，原封不动
	}
}

// IsWaterMeterType 根据 Label 判断当前 DormOption 是否为水表类型
// 判断依据：Label 含"水"字（由 GenerateDisplayName 生成）
// 注意：此函数仅用于判断存储字段，不影响查询逻辑
func IsWaterMeterType(label string) bool {
	return strings.Contains(label, "水")
}









// extractDigits 提取字符串中的所有连续数字（第一个连续数字段）
// 例："110169" → "110169"，"132水表" → "132"
func extractDigits(s string) string {
	var result strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result.WriteRune(r)
		} else if result.Len() > 0 {
			break
		}
	}
	return result.String()
}

// QueryAndSavePower 查询指定宿舍电量并保存到数据库
//
// 核心原则：物理ID是绝对真理。此函数严格遵循以下流程：
//
//  1. 【查映射表】LookupByFormValue(formValue) → 获取 DormOption（含 DrcengValue）
//  2. 【提取参数】从映射表取出 building/floor/DrcengValue
//  3. 【直接查询】使用 DrcengValue（物理ID）作为 room 参数发给网页
//     ⚠ AI 禁忌：绝对禁止对 DrcengValue 做数学推算，如检查后两位是否>68
//  4. 【数据存储】根据楼栋类型和 Label 自动写入对应字段：
//     - C11/C12 水电分房：根据 Label 判断，更新电或水字段
//     - C13/C14 水电合一：同时更新电和水字段
//
// formValue 支持：
//   - FormValue：三段式 "11|1101|110169"（水电分房）或普通6位 "140328"
//   - Label：前端友好显示名如 "C11 132水"（通过 LookupByFormValue 兜底匹配 Label）
func QueryAndSavePower(formValue string, appCfg *config.AppConfig) (*checker.PowerResult, error) {
	// 第1步：查映射表，获取物理ID
	lk := LookupByFormValue(formValue)
	if lk == nil {
		return nil, fmt.Errorf("宿舍号未找到映射: %s（请先执行同步）", formValue)
	}

	// 第2步：提取参数（直接取自映射表，不做推算）
	building := lk.Building
	floor := lk.Floor
	physicalID := lk.DrcengValue // 物理ID：原封不动
	label := lk.Opt.Label

	logger.Debug("宿舍查询",
		"form_value", formValue,
		"label", label,
		"physical_id", physicalID,
		"building", building,
		"floor", floor)

	// 第3步：直接用物理ID查询网页（不传 formValue，不传任何解析后的字符串）
	chk := checker.NewChecker(appCfg)
	result, err := chk.CheckPower(building, floor, physicalID)
	if err != nil {
		return nil, fmt.Errorf("查询失败 form_value=%s physical_id=%s: %w", formValue, physicalID, err)
	}
	result.DormRoom = formValue // 回填原始输入，保持追溯性

	// 第4步：确定楼栋类型，决定存储字段
	isC13C14 := checker.IsC13OrC14(building)
	isWaterMeter := IsWaterMeterType(label)

	// 准备存储数据（由 LookupByFormValue 确定哪个字段有值）
	storeKwh := result.RemainingKwh
	storeWater := result.WaterAmount

	// AI 禁忌：绝对禁止对物理ID做数学判断来决定存储逻辑
	// 存储逻辑由楼栋类型和 Label 共同决定：
	// - C13/C14（水电合一）：网页返回同时含电和水，全部存储
	// - C11/C12（水电分离）+ Label含"水"：仅存储水（网页返回的 RemainingWater 有值）
	// - C11/C12（水电分离）+ Label不含"水"：仅存储电（网页返回的 RemainingKwh 有值）

	today := time.Now().Format("2006-01-02")
	log := &model.PowerLog{
		DormRoom:   formValue,
		RecordDate: today,
	}
	if isC13C14 {
		// C13/C14：水电同页，一个查询同时填两个字段
		log.RemainingKwh = storeKwh
		log.RemainingWater = storeWater
	} else if isWaterMeter {
		// C11/C12 水表：网页返回的 RemainingWater 即为该物理ID对应的水量
		// RemainingKwh 对水表物理ID 无意义，置空
		log.RemainingKwh = ""
		log.RemainingWater = storeWater
	} else {
		// C11/C12 电表：网页返回的 RemainingKwh 即为该物理ID对应的电量
		// RemainingWater 对电表物理ID 无意义，置空
		log.RemainingKwh = storeKwh
		log.RemainingWater = ""
	}

	// 保存到数据库（同一天同一 formValue 只保存一次，重复时覆盖）
	if err := model.DB.Where(model.PowerLog{DormRoom: formValue, RecordDate: today}).
		Assign(model.PowerLog{RemainingKwh: log.RemainingKwh, RemainingWater: log.RemainingWater}).
		FirstOrCreate(log).Error; err != nil {
		return result, fmt.Errorf("保存电量记录失败: %w", err)
	}

	// 第5步：阈值告警（仅针对电表物理ID）
	threshold := appCfg.Scheduler.AlertThreshold
	if !isWaterMeter && result.RemainingF < threshold && result.RemainingF > 0 {
		go alertUsersForDorm(formValue, result.RemainingKwh, threshold)
	}

	return result, nil
}

// alertUsersForDorm 向绑定了指定宿舍的用户发送低电量告警
func alertUsersForDorm(dormRoom, remaining string, threshold float64) {
	var users []model.User
	if err := model.DB.Where("dorm_room = ?", dormRoom).Find(&users).Error; err != nil {
		return
	}

	subject := "⚡ 电量告警 | 剩余电量过低"
	body := fmt.Sprintf(
		"您的宿舍 %s 当前剩余电量为 %s 度，已低于告警阈值 %.1f 度，请及时充值！",
		dormRoom, remaining, threshold,
	)

	for _, u := range users {
		var ch model.UserChannel
		if err := model.DB.Where("user_id = ?", u.ID).First(&ch).Error; err != nil {
			continue
		}
		notifier.SendToUser(ch.Email, ch.WechatWebhook, subject, body)
	}
}

// GetPowerHistory 获取指定宿舍的电量历史记录
// electricDormRoom: 电表宿舍号（FormValue）
// waterDormRoom: 水表宿舍号（FormValue，可为空）
// 逻辑：分别查电表历史和水表历史，按日期合并到同一行返回
func GetPowerHistory(electricDormRoom, waterDormRoom string, limit int) ([]model.PowerLog, error) {
	// 1. 查电表历史
	var elecLogs []model.PowerLog
	q := model.DB.Where("dorm_room = ?", electricDormRoom).Order("record_date DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&elecLogs).Error; err != nil {
		return nil, fmt.Errorf("查询历史记录失败: %w", err)
	}

	// 2. 如果没有水表，直接返回电表记录
	if waterDormRoom == "" || waterDormRoom == electricDormRoom {
		return elecLogs, nil
	}

	// 3. 查水表历史
	var waterLogs []model.PowerLog
	q2 := model.DB.Where("dorm_room = ?", waterDormRoom).Order("record_date DESC")
	if limit > 0 {
		q2 = q2.Limit(limit)
	}
	if err := q2.Find(&waterLogs).Error; err != nil {
		return elecLogs, nil // 水表查不到也继续
	}

	// 4. 按日期合并
	merged := make(map[string]*model.PowerLog)
	for i := range elecLogs {
		merged[elecLogs[i].RecordDate] = &elecLogs[i]
	}
	for i := range waterLogs {
		date := waterLogs[i].RecordDate
		if existing, ok := merged[date]; ok {
			// 同一日期已有记录，补充缺失的字段
			if existing.RemainingKwh == "" && waterLogs[i].RemainingKwh != "" {
				existing.RemainingKwh = waterLogs[i].RemainingKwh
			}
			if existing.RemainingWater == "" && waterLogs[i].RemainingWater != "" {
				existing.RemainingWater = waterLogs[i].RemainingWater
			}
		} else {
			// 新日期，添加水表记录
			merged[date] = &waterLogs[i]
		}
	}

	// 5. 转回 slice 并按日期降序排序（O(n log n)，替代冒泡 O(n²)）
	result := make([]model.PowerLog, 0, len(merged))
	for _, v := range merged {
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].RecordDate > result[j].RecordDate
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}



// GetAllUsersWithDorms 获取所有绑定了宿舍的用户（供 scheduler 批量查询使用）
func GetAllUsersWithDorms() ([]model.User, error) {
	var users []model.User
	if err := model.DB.Where("dorm_room != ?", "").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// SendWeeklyReport 向所有绑定通知渠道的用户发送用电周报
func SendWeeklyReport() {
	users, err := GetAllUsersWithDorms()
	if err != nil {
		return
	}

	for _, u := range users {
		logs, err := GetPowerHistory(u.DormRoom, u.WaterDormRoom, 7)
		if err != nil || len(logs) == 0 {
			continue
		}

		var ch model.UserChannel
		if err := model.DB.Where("user_id = ?", u.ID).First(&ch).Error; err != nil {
			continue
		}

		subject := "📊 宿舍用电周报"
		body := buildWeeklyReportBody(u.DormRoom, logs)
		notifier.SendToUser(ch.Email, ch.WechatWebhook, subject, body)
	}
}

// ToWebValue 将 FormValue 转换为网页 <option value> 标准格式
//
// 数据映射关系：
//   - 普通房间（6位数字）：FormValue == DrcengValue，直接返回（如 "110101"）
//   - 水电分房（building|floor|drceng）：从 DormOption 表查找 DrcengValue
//     （网页表单需要的真实值，如 "110169"）
//   - 无法匹配时返回原始 FormValue
//
// 用于统一 API 响应格式：确保前端拿到的 dorm_room 是网页标准值。
func ToWebValue(formValue string) string {
	if formValue == "" {
		return formValue
	}

	// 普通房间（6位纯数字）：已是标准值，无需转换
	if strutil.IsDigits(formValue) && len(formValue) == 6 {
		return formValue
	}

	// 三段式格式（水电分房）：查 DormOption 获取网页表单值
	if parts := strings.Split(formValue, "|"); len(parts) == 3 {
		_, _, drceng := parts[0], parts[1], parts[2]
		var opt model.DormOption
		if err := model.DB.
			Where("building = ? AND floor = ? AND drceng_value = ? AND level = ?",
				parts[0], parts[1], drceng, model.OptionLevelRoom).
			First(&opt).Error; err == nil && opt.DrcengValue != "" {
			logger.Debug("FormValue→WebValue 水电分房转换",
				"form_value", formValue,
				"web_value", opt.DrcengValue)
			return opt.DrcengValue
		}
	}

	// 兜底：原样返回（已是最底层值或无法解析）
	return formValue
}

// buildWeeklyReportBody 构造周报文本
func buildWeeklyReportBody(dormRoom string, logs []model.PowerLog) string {
	body := fmt.Sprintf("宿舍 %s 最近 7 天用电记录：\n", dormRoom)
	body += "----------------------------\n"
	for i, l := range logs {
		consumption := "暂无数据"
		if i < len(logs)-1 {
			var curr, prev float64
			fmt.Sscanf(l.RemainingKwh, "%f", &curr)
			fmt.Sscanf(logs[i+1].RemainingKwh, "%f", &prev)
			delta := curr - prev
			if delta > 0 {
				consumption = fmt.Sprintf("+%.2f 度", delta)
			} else {
				consumption = fmt.Sprintf("%.2f 度", delta)
			}
		}
		body += fmt.Sprintf("%s | 剩余: %s 度 | 当日消耗: %s\n",
			l.RecordDate, l.RemainingKwh, consumption)
	}
	return body
}
