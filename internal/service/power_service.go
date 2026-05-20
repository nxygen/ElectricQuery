package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"electricquery/internal/checker"
	"electricquery/internal/config"
	"electricquery/internal/logger"
	"electricquery/internal/model"
	"electricquery/internal/notifier"
)

// DormLookupResult 查询结果，包含映射表查出的完整物理信息
type DormLookupResult struct {
	Opt         model.DormOption // 映射表记录（含 DrcengValue/Building/Floor/Label）
	Building    string           // 楼栋代码（直接取自 DormOption.Building）
	Floor       string           // 楼层代码（直接取自 DormOption.Floor）
	DrcengValue string           // 物理ID（直接取自 DormOption.DrcengValue，不做推算）
}

// LookupByFormValue 根据 FormValue 查映射表，返回物理ID和元信息
//
// 核心原则：物理ID是绝对真理。AI 不得对物理ID进行数学推算或逻辑猜测。
//
// 解析步骤：
//  1. 精确匹配 FormValue（普通6位数字，如 "140328"）
//  2. 从匹配到的 DormOption 取出 DrcengValue（这就是网页查询的物理ID）
//
// AI 禁忌：
//  - 绝对禁止看到后两位 "69" 就认为是水表
//  - 绝对禁止尝试将 "110169" 转换回 "110132"
//
// 返回值：nil 表示未找到映射
func LookupByFormValue(formValue string) *DormLookupResult {
	formValue = strings.TrimSpace(formValue)
	if formValue == "" {
		return nil
	}

	var opt model.DormOption
	query := model.DB.Where("level = ?", model.OptionLevelRoom)

	// 精确匹配 FormValue
	if err := query.Where("form_value = ?", formValue).First(&opt).Error; err != nil {
		// 兜底：匹配 Label（兼容旧格式或前端传 Label 的场景）
		if err := query.Where("label = ?", formValue).First(&opt).Error; err != nil {
			logger.Debug("宿舍号未命中 DormOption 映射", "input", formValue)
			return nil
		}
		logger.Debug("宿舍号通过 label 命中映射", "label", formValue, "drceng_value", opt.DrcengValue)
	}

	return &DormLookupResult{
		Opt:         opt,
		Building:    opt.Building,
		Floor:       opt.Floor,
		DrcengValue: opt.DrcengValue,
	}
}

// LookupWaterFormValue 根据电宿舍 drceng_value 查找对应的水宿舍 drceng_value
//
// 通过电表 label 提取房间号（如 "C11-132" → "132"），再找 label 含该房间号的水表
// （如 "C11-132水表" → drceng_value = "110170"）。
// C13/C14 水电合一楼栋返回空（无独立水宿舍）。
func LookupWaterFormValue(electricDrceng string) string {
	lk := LookupByFormValue(electricDrceng)
	if lk == nil {
		return ""
	}

	// 从电表 label 提取房间号
	// label 格式：不含 "电"/"水" 后缀（如 "C11-132"）
	// 或含后缀（如 "C11-132电表" → 提取 "132"）
	roomNum := extractRoomNumber(lk.Opt.Label)
	if roomNum == "" {
		logger.Debug("无法从电表 label 提取房间号", "label", lk.Opt.Label)
		return ""
	}

	// 用房间号精确匹配水表：label 含 "水" 且含该房间号
	// 不依赖 floor，building + 房间号已足够唯一定位
	// 兼容 "C11-132水" 和 "C11-132水表" 两种 label 格式
	var waterOpt model.DormOption
	err := model.DB.
		Where("level = ? AND building = ? AND label LIKE ? AND label LIKE ?",
			model.OptionLevelRoom, lk.Building, "%水%", "%"+roomNum+"%").
		First(&waterOpt).Error
	if err != nil {
		logger.Debug("未找到对应水表", "electric", electricDrceng, "room", roomNum,
			"building", lk.Building, "floor", lk.Floor)
		return ""
	}
	return waterOpt.DrcengValue
}

// extractRoomNumber 从 label（如 "C11-132"、"C14-328水表"、"132电"）中提取纯数字房间号
func extractRoomNumber(label string) string {
	// 去掉常见后缀
	clean := strings.TrimSuffix(label, "电")
	clean = strings.TrimSuffix(clean, "电表")
	clean = strings.TrimSuffix(clean, "水")
	clean = strings.TrimSuffix(clean, "水表")
	clean = strings.TrimSpace(clean)
	// 从尾部提取纯数字
	var digits strings.Builder
	for i := len(clean) - 1; i >= 0; i-- {
		if clean[i] >= '0' && clean[i] <= '9' {
			digits.WriteByte(clean[i])
		} else {
			break
		}
	}
	// 反转
	b := []byte(digits.String())
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

// QueryAndSavePower 查询指定宿舍的电量和水费并保存
//
// 接收电表 drceng_value 和水表 drceng_value（如需查水），分别查询并保存。
// waterDormRoom 为空时自动通过 ResolveWaterDormRoom 解析（兼容 v2.0.2 行为）。
func QueryAndSavePower(ctx context.Context, dormRoom, waterDormRoom string, appCfg *config.AppConfig) (*checker.PowerResult, error) {
	if dormRoom == "" {
		return nil, fmt.Errorf("dormRoom 不能为空")
	}

	// 自动解析水表：waterDormRoom 为空时根据楼栋类型推断
	if waterDormRoom == "" {
		waterDormRoom = ResolveWaterDormRoom(dormRoom)
	}

	// 1. 查电表 DormOption（直接用 drceng_value 查）
	var elecOpt model.DormOption
	if err := model.DB.Where("level = ? AND drceng_value = ?",
		model.OptionLevelRoom, dormRoom).First(&elecOpt).Error; err != nil {
		return nil, fmt.Errorf("宿舍号未找到映射: %s（请先执行同步）", dormRoom)
	}

	building := elecOpt.Building

	logger.Debug("宿舍查询",
		"dorm_room", dormRoom,
		"water_dorm_room", waterDormRoom,
		"label", elecOpt.Label,
		"building", building)

	chk := checker.NewChecker(appCfg)
	today := time.Now().Format("2006-01-02")

	// 2. 查询电表（CheckPower 同时返回电和水数据）
	elecResult, err := chk.CheckPower(ctx, building, elecOpt.Floor, dormRoom)
	if err != nil {
		return nil, fmt.Errorf("查询失败 dorm=%s: %w", dormRoom, err)
	}
	elecResult.DormRoom = dormRoom

	// 保存电表记录
	elecLog := &model.ElectricityLog{
		DormRoom:     dormRoom,
		RecordDate:   today,
		RemainingKwh: elecResult.RemainingKwh,
		QueriedAt:   time.Now().Format(time.RFC3339),
	}
	if err := model.DB.Where(model.ElectricityLog{DormRoom: dormRoom, RecordDate: today}).
		Assign(model.ElectricityLog{RemainingKwh: elecLog.RemainingKwh, QueriedAt: elecLog.QueriedAt}).
		FirstOrCreate(elecLog).Error; err != nil {
		return elecResult, fmt.Errorf("保存电表记录失败: %w", err)
	}

	// 3. 处理水表
	if waterDormRoom == "" {
		// 未配置水宿舍，跳过
	} else if waterDormRoom == dormRoom {
		// 水电合一：同一 drceng_value 同时含水电数据
		waterLog := &model.WaterLog{
			DormRoom:       dormRoom,
			RecordDate:     today,
			RemainingWater: elecResult.WaterAmount,
			QueriedAt:      time.Now().Format(time.RFC3339),
		}
		if err := model.DB.Where(model.WaterLog{DormRoom: dormRoom, RecordDate: today}).
			Assign(model.WaterLog{RemainingWater: waterLog.RemainingWater, QueriedAt: waterLog.QueriedAt}).
			FirstOrCreate(waterLog).Error; err != nil {
			return elecResult, fmt.Errorf("保存水表记录失败: %w", err)
		}
	} else {
		// 水电分房：用精确的 waterDormRoom 查询水表
		var waterOpt model.DormOption
		if err := model.DB.Where("level = ? AND drceng_value = ?",
			model.OptionLevelRoom, waterDormRoom).First(&waterOpt).Error; err != nil {
			logger.Warn("未找到水宿舍映射，跳过水表查询",
				"water_dorm_room", waterDormRoom, "hint", "请确认 dorm_options 中存在对应记录")
		} else {
			waterResult, werr := chk.CheckPower(ctx, building, waterOpt.Floor, waterDormRoom)
			if werr != nil {
				logger.Warn("水表查询失败，跳过", "water_drceng", waterDormRoom, "error", werr)
			} else {
				waterLog := &model.WaterLog{
					DormRoom:       waterDormRoom,
					RecordDate:     today,
					RemainingWater: waterResult.WaterAmount,
					QueriedAt:      time.Now().Format(time.RFC3339),
				}
				if err := model.DB.Where(model.WaterLog{DormRoom: waterDormRoom, RecordDate: today}).
					Assign(model.WaterLog{RemainingWater: waterLog.RemainingWater, QueriedAt: waterLog.QueriedAt}).
					FirstOrCreate(waterLog).Error; err != nil {
					return elecResult, fmt.Errorf("保存水表记录失败: %w", err)
				}
				elecResult.WaterAmount = waterResult.WaterAmount
				elecResult.WaterF = waterResult.WaterF
			}
		}
	}

	// 4. 阈值告警（仅对电表触发）
	threshold := appCfg.Scheduler.AlertThreshold
	if elecResult.RemainingF < threshold && elecResult.RemainingF > 0 {
		go alertUsersForDorm(dormRoom, elecResult.RemainingKwh, threshold)
	}

	return elecResult, nil
}

// alertUsersForDorm 向绑定了指定宿舍的用户发送低电量告警
func alertUsersForDorm(dormRoom, remaining string, threshold float64) {
	var users []model.User
	if err := model.DB.Where("dorm_room = ?", dormRoom).Find(&users).Error; err != nil {
		return
	}

	subject := "电量告警 | 剩余电量过低"
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
// electricDormRoom: 电表 drceng_value
// waterDormRoom: 水表 drceng_value（可为空）
// 逻辑：分别查电表历史和水表历史，按日期合并到同一行返回
func GetPowerHistory(electricDormRoom, waterDormRoom string, limit int) ([]model.PowerLog, error) {
	// 1. 查电表历史
	var elecLogs []model.ElectricityLog
	q := model.DB.Where("dorm_room = ?", electricDormRoom).Order("record_date DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&elecLogs).Error; err != nil {
		return nil, fmt.Errorf("查询电表历史失败: %w", err)
	}

	// 2. 没有水表时直接返回电表记录
	if waterDormRoom == "" {
		return elecLogsToPowerLogs(elecLogs), nil
	}

	// 3. 查水表历史（水电合一和水电分房统一走合并逻辑）
	var waterLogs []model.WaterLog
	q2 := model.DB.Where("dorm_room = ?", waterDormRoom).Order("record_date DESC")
	if limit > 0 {
		q2 = q2.Limit(limit)
	}
	if err := q2.Find(&waterLogs).Error; err != nil {
		return elecLogsToPowerLogs(elecLogs), nil
	}

	// 4. 按日期合并
	merged := make(map[string]*model.PowerLog)

	for i := range elecLogs {
		t, _ := time.Parse(time.RFC3339, elecLogs[i].QueriedAt)
		merged[elecLogs[i].RecordDate] = &model.PowerLog{
			DormRoom:     elecLogs[i].DormRoom,
			RecordDate:   elecLogs[i].RecordDate,
			RemainingKwh: elecLogs[i].RemainingKwh,
			QueriedAt:    t,
		}
	}

	for _, wl := range waterLogs {
		t, _ := time.Parse(time.RFC3339, wl.QueriedAt)
		if existing, ok := merged[wl.RecordDate]; ok {
			existing.RemainingWater = wl.RemainingWater
		} else {
			merged[wl.RecordDate] = &model.PowerLog{
				DormRoom:       wl.DormRoom,
				RecordDate:     wl.RecordDate,
				RemainingWater: wl.RemainingWater,
				QueriedAt:      t,
			}
		}
	}

	// 5. 转回 slice 并按日期降序排序
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

// elecLogsToPowerLogs 将 ElectricityLog 列表转换为 PowerLog 列表（兼容旧接口）
func elecLogsToPowerLogs(elecLogs []model.ElectricityLog) []model.PowerLog {
	result := make([]model.PowerLog, len(elecLogs))
	for i, el := range elecLogs {
		t, _ := time.Parse(time.RFC3339, el.QueriedAt)
		result[i] = model.PowerLog{
			DormRoom:     el.DormRoom,
			RecordDate:   el.RecordDate,
			RemainingKwh: el.RemainingKwh,
			QueriedAt:    t,
		}
	}
	return result
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
		waterDorm := ResolveWaterDormRoom(u.DormRoom)
		logs, err := GetPowerHistory(u.DormRoom, waterDorm, 7)
		if err != nil || len(logs) == 0 {
			continue
		}

		var ch model.UserChannel
		if err := model.DB.Where("user_id = ?", u.ID).First(&ch).Error; err != nil {
			continue
		}

		subject := "宿舍用电周报"
		body := buildWeeklyReportBody(u.DormRoom, logs)
		notifier.SendToUser(ch.Email, ch.WechatWebhook, subject, body)
	}
}

// ToWebValue 将 drceng_value 转换为网页 <option value> 标准格式
func ToWebValue(drcengValue string) string {
	return drcengValue
}

// ResolveWaterDormRoom 根据电表 drceng_value 判断水表 drceng_value
// 用于 Admin/Internal API 需要同时查水电的场景
// 返回值与 QueryAndSavePower 的 waterDormRoom 参数语义一致：
//   - 水电合一楼栋（C13/C14）：返回与 electricDrceng 相同的值
//   - 水电分房楼栋（C11/C12）：返回独立水表 drceng_value
//   - 无水表：返回空字符串
func ResolveWaterDormRoom(electricDrceng string) string {
	if electricDrceng == "" {
		return ""
	}
	lk := LookupByFormValue(electricDrceng)
	if lk == nil {
		return ""
	}
	// 检查该楼栋是否有独立水宿舍
	waterDrceng := LookupWaterFormValue(electricDrceng)
	if waterDrceng != "" {
		return waterDrceng // 水电分房
	}
	// 无独立水宿舍：判断是否为水电合一楼栋（同一 drceng_value 返回电水数据）
	// 简单判断：drceng_value 不含水标记且楼栋非空
	if checker.IsC13OrC14(lk.Building) {
		return electricDrceng // 水电合一
	}
	return ""
}

// buildWeeklyReportBody 构造水电双报文本
func buildWeeklyReportBody(dormRoom string, logs []model.PowerLog) string {
	displayName := dormRoom
	var opt model.DormOption
	if err := model.DB.Where("drceng_value = ?", dormRoom).First(&opt).Error; err == nil && opt.Label != "" {
		displayName = opt.Label
	}

	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("宿舍 %s 最近 7 天用电记录：\n", displayName))
	buf.WriteString("----------------------------\n")
	hasWater := false
	for i, l := range logs {
		if l.RemainingWater != "" {
			hasWater = true
		}
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
		buf.WriteString(fmt.Sprintf("%s | 剩余: %s 度 | 当日消耗: %s\n",
			l.RecordDate, l.RemainingKwh, consumption))
	}

	if hasWater {
		buf.WriteString("\n宿舍 " + displayName + " 最近 7 天用水记录：\n")
		buf.WriteString("----------------------------\n")
		for i, l := range logs {
			consumption := "暂无数据"
			if l.RemainingWater != "" && i < len(logs)-1 {
				var curr, prev float64
				fmt.Sscanf(l.RemainingWater, "%f", &curr)
				fmt.Sscanf(logs[i+1].RemainingWater, "%f", &prev)
				delta := curr - prev
				if delta > 0 {
					consumption = fmt.Sprintf("+%.2f 吨", delta)
				} else {
					consumption = fmt.Sprintf("%.2f 吨", delta)
				}
			}
			waterDisplay := "暂无数据"
			if l.RemainingWater != "" {
				waterDisplay = l.RemainingWater + " 吨"
			}
			buf.WriteString(fmt.Sprintf("%s | 剩余: %s | 当日消耗: %s\n",
				l.RecordDate, waterDisplay, consumption))
		}
	}

	return buf.String()
}
