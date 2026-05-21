package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"electricquery/internal/cache"
	"electricquery/internal/checker"
	"electricquery/internal/config"
	"electricquery/internal/logger"
	"electricquery/internal/model"
	"electricquery/internal/notifier"
)

type DormLookupResult struct {
	Opt         model.DormOption
	Building    string
	Floor       string
	DrcengValue string
}

func LookupByFormValue(formValue string) *DormLookupResult {
	formValue = strings.TrimSpace(formValue)
	if formValue == "" {
		return nil
	}
	if cached := getCachedDormLookup(formValue); cached != nil {
		return cached
	}

	if opt, ok := lookupRoomByDrceng(formValue); ok {
		logger.Debug("宿舍号通过 drceng_value 命中映射", "drceng_value", formValue)
		return cacheDormLookup(formValue, dormLookupFromOption(opt))
	}

	if opt, ok := lookupLegacyJoinedFormValue(formValue); ok {
		logger.Warn("修正旧版拼接宿舍号", "input", formValue, "drceng_value", opt.DrcengValue, "label", opt.Label)
		return cacheDormLookup(formValue, dormLookupFromOption(opt))
	}

	var opt model.DormOption
	query := model.DB.Where("level = ?", model.OptionLevelRoom)

	if err := query.Where("form_value = ?", formValue).First(&opt).Error; err != nil {
		if err := query.Where("label = ?", formValue).First(&opt).Error; err != nil {
			logger.Debug("宿舍号未命中 DormOption 映射", "input", formValue)
			return nil
		}
		logger.Debug("宿舍号通过 label 命中映射", "label", formValue, "drceng_value", opt.DrcengValue)
	}

	return cacheDormLookup(formValue, dormLookupFromOption(opt))
}

func dormLookupFromOption(opt model.DormOption) *DormLookupResult {
	return &DormLookupResult{
		Opt:         opt,
		Building:    opt.Building,
		Floor:       opt.Floor,
		DrcengValue: opt.DrcengValue,
	}
}

func getCachedDormLookup(input string) *DormLookupResult {
	if value, found := cache.Get("dorm_lookup:" + input); found {
		if result, ok := value.(DormLookupResult); ok {
			return &result
		}
	}
	return nil
}

func cacheDormLookup(input string, result *DormLookupResult) *DormLookupResult {
	if result != nil {
		cache.Set("dorm_lookup:"+input, *result, time.Hour)
	}
	return result
}

func lookupRoomByDrceng(drcengValue string) (model.DormOption, bool) {
	var opt model.DormOption
	err := model.DB.Where("level = ? AND drceng_value = ?", model.OptionLevelRoom, drcengValue).First(&opt).Error
	return opt, err == nil
}

func lookupLegacyJoinedFormValue(value string) (model.DormOption, bool) {
	if len(value) < 7 || !isASCIIAllDigits(value) {
		return model.DormOption{}, false
	}

	building := value[:2]
	floorValue := value[2:4]
	roomNum := value[4:]
	floorDisplay := strings.TrimLeft(floorValue, "0")
	if floorDisplay == "" {
		floorDisplay = "0"
	}
	if !strings.HasPrefix(roomNum, floorDisplay) {
		return model.DormOption{}, false
	}

	roomTail := roomNum[len(floorDisplay):]
	if roomTail == "" {
		return model.DormOption{}, false
	}
	candidate := building + floorValue + roomTail
	return lookupRoomByDrceng(candidate)
}

func isASCIIAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func LookupWaterFormValue(electricDrceng string) string {
	lk := LookupByFormValue(electricDrceng)
	if lk == nil {
		return ""
	}
	cacheKey := "water_lookup:" + lk.DrcengValue
	if value, found := cache.GetString(cacheKey); found {
		return value
	}

	roomNum := extractRoomNumber(lk.Opt.Label)
	if roomNum == "" {
		logger.Debug("无法从电表 label 提取房间号", "label", lk.Opt.Label)
		return ""
	}

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
	cache.Set(cacheKey, waterOpt.DrcengValue, time.Hour)
	return waterOpt.DrcengValue
}

func extractRoomNumber(label string) string {
	clean := strings.TrimSuffix(label, "电")
	clean = strings.TrimSuffix(clean, "电表")
	clean = strings.TrimSuffix(clean, "水")
	clean = strings.TrimSuffix(clean, "水表")
	clean = strings.TrimSpace(clean)
	var digits strings.Builder
	for i := len(clean) - 1; i >= 0; i-- {
		if clean[i] >= '0' && clean[i] <= '9' {
			digits.WriteByte(clean[i])
		} else {
			break
		}
	}
	b := []byte(digits.String())
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

func QueryAndSavePower(ctx context.Context, dormRoom, waterDormRoom string, appCfg *config.AppConfig) (*checker.PowerResult, error) {
	dormRoom = strings.TrimSpace(dormRoom)
	if dormRoom == "" {
		return nil, fmt.Errorf("dormRoom 不能为空")
	}

	if lk := LookupByFormValue(dormRoom); lk != nil {
		dormRoom = lk.DrcengValue
	}

	if waterDormRoom != "" {
		waterDormRoom = strings.TrimSpace(waterDormRoom)
		if lk := LookupByFormValue(waterDormRoom); lk != nil {
			waterDormRoom = lk.DrcengValue
		}
	}

	if waterDormRoom == "" {
		waterDormRoom = ResolveWaterDormRoom(dormRoom)
	}

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

	elecResult, err := chk.CheckPower(ctx, building, elecOpt.Floor, dormRoom)
	if err != nil {
		return nil, fmt.Errorf("查询失败 dorm=%s: %w", dormRoom, err)
	}
	elecResult.DormRoom = dormRoom

	elecLog := &model.ElectricityLog{
		DormRoom:     dormRoom,
		RecordDate:   today,
		RemainingKwh: elecResult.RemainingKwh,
		QueriedAt:    time.Now().Format(time.RFC3339),
	}
	if err := model.DB.Where(model.ElectricityLog{DormRoom: dormRoom, RecordDate: today}).
		Assign(model.ElectricityLog{RemainingKwh: elecLog.RemainingKwh, QueriedAt: elecLog.QueriedAt}).
		FirstOrCreate(elecLog).Error; err != nil {
		return elecResult, fmt.Errorf("保存电表记录失败: %w", err)
	}

	if waterDormRoom == "" {
		// 未配置水宿舍，跳过
	} else if waterDormRoom == dormRoom {
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

	threshold := appCfg.Scheduler.AlertThreshold
	if elecResult.RemainingF < threshold && elecResult.RemainingF > 0 {
		go alertUsersForDorm(dormRoom, elecResult.RemainingKwh, threshold)
	}

	return elecResult, nil
}

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

func GetPowerHistory(electricDormRoom, waterDormRoom string, limit int) ([]model.PowerLog, error) {
	var elecLogs []model.ElectricityLog
	q := model.DB.Where("dorm_room = ?", electricDormRoom).Order("record_date DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&elecLogs).Error; err != nil {
		return nil, fmt.Errorf("查询电表历史失败: %w", err)
	}

	if waterDormRoom == "" {
		return elecLogsToPowerLogs(elecLogs), nil
	}

	var waterLogs []model.WaterLog
	q2 := model.DB.Where("dorm_room = ?", waterDormRoom).Order("record_date DESC")
	if limit > 0 {
		q2 = q2.Limit(limit)
	}
	if err := q2.Find(&waterLogs).Error; err != nil {
		return elecLogsToPowerLogs(elecLogs), nil
	}

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

func GetAllUsersWithDorms() ([]model.User, error) {
	var users []model.User
	if err := model.DB.Where("dorm_room != ?", "").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

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

func ToWebValue(drcengValue string) string {
	return drcengValue
}

func ResolveWaterDormRoom(electricDrceng string) string {
	if electricDrceng == "" {
		return ""
	}
	lk := LookupByFormValue(electricDrceng)
	if lk == nil {
		return ""
	}
	waterDrceng := LookupWaterFormValue(electricDrceng)
	if waterDrceng != "" {
		return waterDrceng
	}
	if checker.IsC13OrC14(lk.Building) {
		return electricDrceng
	}
	return ""
}

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
