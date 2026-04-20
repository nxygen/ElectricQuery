// Package model 定义所有 GORM 数据模型
// 使用 GORM 后只需修改 config.go 中的 driver，即可无缝切换 SQLite <-> MySQL
package model

import (
	"time"

	"gorm.io/gorm"
)

// User 用户表
// - StudentID         学号，唯一标识，注册后不可修改
// - Password          bcrypt 哈希后的密码
// - Building          宿舍楼（如 "C13"）
// - DormRoom          电费宿舍号（默认字段，如 "C14-328" 或 "1301电"）
// - WaterDormRoom     水费宿舍号（如水电分开则为独立房间，如 "1301水"；C13/C14 则同电费宿舍）
// - Class             班级（如 "高分子2301"）
type User struct {
	gorm.Model
	StudentID     string `gorm:"uniqueIndex;not null;size:32" json:"student_id"`
	Password      string `gorm:"not null"                     json:"-"`
	Name          string `gorm:"size:64"                      json:"name"`
	Building      string `gorm:"size:32"                      json:"building"`
	DormRoom      string `gorm:"size:64;index"                json:"dorm_room"`
	WaterDormRoom string `gorm:"size:64;index"               json:"water_dorm_room"`
	Class         string `gorm:"size:64"                      json:"class"`
}

// UserChannel 用户通知渠道配置表
// 一个用户对应一条记录，保存企业微信 Webhook URL 和邮箱地址
type UserChannel struct {
	gorm.Model
	UserID         uint   `gorm:"uniqueIndex;not null"  json:"user_id"`
	User           User   `gorm:"foreignKey:UserID"     json:"-"`
	WechatWebhook  string `gorm:"size:512"              json:"wechat_webhook"`  // 企业微信机器人 Webhook URL
	Email          string `gorm:"size:256;index"        json:"email"`           // 接收通知的邮箱
}

// PowerLog 宿舍水电历史记录表
// - DormRoom      宿舍号（与 User.DormRoom 对应）
// - RecordDate    记录日期（格式 YYYY-MM-DD），与 DormRoom 联合唯一
// - RemainingKwh  剩余电量（度），字符串存储以保留原始精度
// - RemainingWater 剩余水量（吨），字符串存储（仅 C13/C14 楼有值）
type PowerLog struct {
	gorm.Model
	DormRoom       string    `gorm:"size:64;index"                    json:"dorm_room"`
	RecordDate     string    `gorm:"size:16;uniqueIndex:udx_log"      json:"record_date"`
	DormRoomIdx    string    `gorm:"size:64;uniqueIndex:udx_log"      json:"-"` // 冗余字段，配合 RecordDate 组成联合唯一索引
	RemainingKwh  string    `gorm:"size:32"                          json:"remaining_kwh"`
	RemainingWater string    `gorm:"size:32"                          json:"remaining_water"` // 剩余水量（吨）
	QueriedAt      time.Time `gorm:"autoCreateTime"                   json:"queried_at"`
}

// BeforeCreate PowerLog 钩子：同步 DormRoomIdx
func (p *PowerLog) BeforeCreate(tx *gorm.DB) error {
	p.DormRoomIdx = p.DormRoom
	return nil
}

// BeforeSave PowerLog 钩子：同步 DormRoomIdx
func (p *PowerLog) BeforeSave(tx *gorm.DB) error {
	p.DormRoomIdx = p.DormRoom
	return nil
}
