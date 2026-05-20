// Package model 定义所有 GORM 数据模型
// 主键统一使用 UUID v7（string, size:36）
// - 时序前缀：插入顺序性与自增 ID 相当，B树碎片化极低
// - 不可枚举：杜绝 ID 可猜测/遍历的安全风险
// - 全局唯一：支持未来分布式扩展
// 所有表均含 created_at / updated_at / deleted_at（软删除），与 gorm.Model 等价但主键类型为 UUID
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel 替代 gorm.Model，主键改为 UUID string
type BaseModel struct {
	ID        string         `gorm:"primaryKey;size:36"   json:"id"`
	CreatedAt time.Time      `                            json:"created_at"`
	UpdatedAt time.Time      `                            json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"                json:"-"`
}

// BeforeCreate 自动为所有嵌入 BaseModel 的记录生成 UUID v7（时序版）
// v7 保留时间前缀，插入顺序性与自增 ID 相当，同时不可枚举
func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		b.ID = id.String()
	}
	return nil
}

// User 用户表
// - Username      登录账号（唯一，3~32 字符）
// - StudentID     学号（绑定后全局唯一，未绑定为 NULL）
// - Password      bcrypt 哈希，cost=12
// - Building      ⚠ 已废弃，请使用 DormRoom + checker.ParseDorm 推算（前端回填用，后端不再依赖此字段）
// - DormRoom      电费宿舍号（如 "C14-328" 或 "1301电"）
// - WaterDormRoom 水费宿舍号（C13/C14 与电费同宿舍；其他楼按实际填写）
// - Class         班级（如 "高分子2301"）
type User struct {
	BaseModel
	Username      string  `gorm:"uniqueIndex;not null;size:64;default:''"  json:"username"`
	StudentID     *string `gorm:"uniqueIndex;size:32"           json:"student_id"`   // 指针类型，未绑定时为 NULL（不触发 UNIQUE）
	Password      string  `gorm:"not null"                      json:"-"`
	Name          string  `gorm:"size:64"                       json:"name"`
	Building          string  `gorm:"size:32;-;default:''"         json:"building"`      // ⚠ 已废弃，保留仅用于前端兼容，后续移除
	DormRoom          string  `gorm:"size:64;index"                 json:"dorm_room"`
	DormFloor         string  `gorm:"size:16;-;default:''"          json:"dorm_floor"`
	WaterDormRoom     string  `gorm:"size:64;index"                 json:"water_dorm_room"`
	WaterDormFloor    string  `gorm:"size:16;-;default:''"          json:"water_dorm_floor"`
	Class       string `gorm:"size:64"          json:"class"`
	TOTPSecret  string `gorm:"size:128"          json:"-"` // TOTP 密钥（加密存储，为空表示未启用）
	TOTPEnabled bool   `gorm:"default:false"     json:"totp_enabled"` // 是否已激活两步验证
}

// UserChannel 用户通知渠道配置表（每用户一条，1:1）
type UserChannel struct {
	BaseModel
	UserID        string `gorm:"uniqueIndex;not null;size:36"  json:"user_id"`
	User          User   `gorm:"foreignKey:UserID"             json:"-"`
	WechatWebhook string `gorm:"size:512"                      json:"wechat_webhook"`
	Email         string `gorm:"size:256;index"                json:"email"`
}

// PowerLog 宿舍水电历史记录表
// - DormRoom      宿舍号
// - RecordDate    记录日期（YYYY-MM-DD），与 DormRoom 联合唯一
// - RemainingKwh  剩余电量字符串（保留原始精度）
// - RemainingWater 剩余水量字符串（仅 C13/C14 楼有值）
type PowerLog struct {
	BaseModel
	DormRoom       string    `gorm:"size:64;uniqueIndex:udx_log;not null"  json:"dorm_room"`
	RecordDate     string    `gorm:"size:16;uniqueIndex:udx_log;not null"  json:"record_date"`
	RemainingKwh   string    `gorm:"size:32"                               json:"remaining_kwh"`
	RemainingWater string    `gorm:"size:32"                               json:"remaining_water"`
	QueriedAt      time.Time `gorm:"autoCreateTime"                        json:"queried_at"`
}

// OptionLevel 下拉选项层级
type OptionLevel string

const (
	OptionLevelBuilding OptionLevel = "building" // 楼栋下拉
	OptionLevelFloor   OptionLevel = "floor"     // 楼层下拉
	OptionLevelRoom    OptionLevel = "room"      // 房间下拉
)

// DormOption 官网页面下拉选项缓存表
// 用于前端三级联动下拉（楼栋→楼层→房间），以及反向查找实际表单值
//
// key = building + "_" + floor + "_" + drceng_value
// - building:     楼栋代码，如 "11"
// - floor:        ablou 表单值，如 "1101"（C11 第1层）
// - drceng_value: drceng 下拉框的原始值，如 "132水表"（直接 POST 给网站）
// - form_value:   前端/数据库存储的宿舍标识，对普通房间与 drceng_value 相同；
//                 对水电分房楼栋，格式为 building+"|"+floor+"|"+drceng_value（如 "11|1101|132水表"）
//                 用于 checker.ParseDorm 能正确反解出三段参数
type DormOption struct {
	BaseModel
	Key         string      `gorm:"uniqueIndex;size:128;not null" json:"key"`          // building_floor_drcengvalue
	Level       OptionLevel `gorm:"size:16;index;not null"        json:"level"`
	Building    string      `gorm:"size:8;index;not null"         json:"building"`
	Floor       string      `gorm:"size:16;index;default:''"      json:"floor"`
	RoomSuffix  string      `gorm:"size:32;index;default:''"      json:"room_suffix"`  // 从 h6/drceng 提取的房间后缀，如 "132水表"
	DrcengValue string      `gorm:"size:64;not null;default:''"   json:"drceng_value"` // drceng 下拉框原始值（爬虫 POST 时使用）
	FormValue   string      `gorm:"size:128;not null"             json:"form_value"`   // 存储/查询标识：统一为纯数字（去中文字符）
	Label       string      `gorm:"size:128;not null"             json:"label"`        // 前端显示标签（官网原版显示名）
}

// BeforeCreate 生成复合主键和 Key
func (d *DormOption) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		d.ID = id.String()
	}
	if d.Key == "" {
		d.Key = d.Building + "_" + d.Floor + "_" + d.DrcengValue
	}
	return nil
}

// DormOptionDTO 前端下拉选项数据传输对象（不含内部字段）
type DormOptionDTO struct {
	Building    string `json:"building"`
	Floor       string `json:"floor"`
	RoomSuffix  string `json:"room_suffix"`
	DrcengValue string `json:"drceng_value"` // drceng 原始值（前端不直接使用，仅供调试）
	FormValue   string `json:"form_value"`   // 存储/查询标识（前端三级联动存此值）
	Label       string `json:"label"`
}

// SyncMeta 同步元数据表（单行）
// 用于持久化存储最近一次同步时间，使服务重启后管理后台仍能显示正确的时间戳
type SyncMeta struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	LastSyncAt  *time.Time `gorm:"size:36"           json:"last_sync_at"`  // nil 表示从未同步
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// BeforeCreate 为 SyncMeta 生成固定 ID（始终只有一条记录，ID 固定为 "sync-meta"）
func (s *SyncMeta) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = "sync-meta"
	}
	return nil
}
