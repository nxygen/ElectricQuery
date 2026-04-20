package service

import (
	"fmt"
	"time"

	"electricquery/internal/checker"
	"electricquery/internal/config"
	"electricquery/internal/model"
	"electricquery/internal/notifier"
)

// QueryAndSavePower 查询指定宿舍电量并保存到数据库
// 如果低于阈值，向所有绑定该宿舍的用户发送告警通知
func QueryAndSavePower(dormRoom string, appCfg *config.AppConfig) (*checker.PowerResult, error) {
	chk := checker.NewChecker(appCfg)
	result, err := chk.CheckPowerByDorm(dormRoom)
	if err != nil {
		return nil, fmt.Errorf("查询电量失败 dorm=%s: %w", dormRoom, err)
	}

	// 保存到数据库（同一天同一宿舍号只保存一次，重复时覆盖）
	today := time.Now().Format("2006-01-02")
	log := &model.PowerLog{
		DormRoom:       dormRoom,
		RecordDate:     today,
		DormRoomIdx:    dormRoom,
		RemainingKwh:  result.RemainingKwh,
		RemainingWater: result.WaterAmount, // C13/C14 楼水电同页，水量同步保存
	}
	if err := model.DB.Where(model.PowerLog{DormRoom: dormRoom, RecordDate: today}).
		Assign(model.PowerLog{RemainingKwh: result.RemainingKwh, RemainingWater: result.WaterAmount, DormRoomIdx: dormRoom}).
		FirstOrCreate(log).Error; err != nil {
		return result, fmt.Errorf("保存电量记录失败: %w", err)
	}

	// 阈值告警：查找绑定该宿舍的所有用户并推送
	threshold := appCfg.Scheduler.AlertThreshold
	if result.RemainingF < threshold && result.RemainingF > 0 {
		go alertUsersForDorm(dormRoom, result.RemainingKwh, threshold)
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

	for _, user := range users {
		var ch model.UserChannel
		if err := model.DB.Where("user_id = ?", user.ID).First(&ch).Error; err != nil {
			continue
		}
		notifier.SendToUser(ch.Email, ch.WechatWebhook, subject, body)
	}
}

// GetPowerHistory 获取指定宿舍的电量历史记录
func GetPowerHistory(dormRoom string, limit int) ([]model.PowerLog, error) {
	var logs []model.PowerLog
	query := model.DB.Where("dorm_room = ?", dormRoom).
		Order("record_date DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("查询历史记录失败: %w", err)
	}
	return logs, nil
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

	for _, user := range users {
		logs, err := GetPowerHistory(user.DormRoom, 7)
		if err != nil || len(logs) == 0 {
			continue
		}

		var ch model.UserChannel
		if err := model.DB.Where("user_id = ?", user.ID).First(&ch).Error; err != nil {
			continue
		}

		subject := "📊 宿舍用电周报"
		body := buildWeeklyReportBody(user.DormRoom, logs)
		notifier.SendToUser(ch.Email, ch.WechatWebhook, subject, body)
	}
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
