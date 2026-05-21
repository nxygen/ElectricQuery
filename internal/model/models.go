package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        string         `gorm:"primaryKey;size:36"   json:"id"`
	CreatedAt time.Time      `                            json:"created_at"`
	UpdatedAt time.Time      `                            json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"             json:"-"`
}

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

type User struct {
	BaseModel
	Username       string  `gorm:"uniqueIndex;not null;size:64;default:''"  json:"username"`
	StudentID      *string `gorm:"uniqueIndex;size:32"                json:"student_id"`
	Password       string  `gorm:"not null"                            json:"-"`
	Name           string  `gorm:"size:64"                             json:"name"`
	Building       string  `gorm:"size:32;default:''"                 json:"building"`
	DormRoom       string  `gorm:"size:64;index"                      json:"dorm_room"`
	DormFloor      string  `gorm:"size:16;default:''"                 json:"dorm_floor"`
	WaterDormRoom  string  `gorm:"size:64;index"                      json:"water_dorm_room"`
	WaterDormFloor string  `gorm:"size:16;default:''"                 json:"water_dorm_floor"`
	Class          string  `gorm:"size:64"                            json:"class"`
	TOTPSecret     string  `gorm:"size:128"                           json:"-"`
	TOTPEnabled    bool    `gorm:"default:false"                      json:"totp_enabled"`
}

type UserChannel struct {
	BaseModel
	UserID        string `gorm:"uniqueIndex;not null;size:36"  json:"user_id"`
	User          User   `gorm:"foreignKey:UserID"              json:"-"`
	WechatWebhook string `gorm:"size:512"                      json:"wechat_webhook"`
	Email         string `gorm:"size:256;index"                json:"email"`
}

type PowerLog struct {
	BaseModel
	DormRoom       string    `gorm:"size:64;uniqueIndex:udx_log;not null"  json:"dorm_room"`
	RecordDate     string    `gorm:"size:16;uniqueIndex:udx_log;not null"  json:"record_date"`
	RemainingKwh   string    `gorm:"size:32"                               json:"remaining_kwh"`
	RemainingWater string    `gorm:"size:32"                               json:"remaining_water"`
	QueriedAt      time.Time `gorm:"autoCreateTime"                         json:"queried_at"`
}

type OptionLevel string

const (
	OptionLevelBuilding OptionLevel = "building"
	OptionLevelFloor    OptionLevel = "floor"
	OptionLevelRoom     OptionLevel = "room"
)

type DormOption struct {
	BaseModel
	Key         string      `gorm:"uniqueIndex;size:128;not null"  json:"key"`
	Level       OptionLevel `gorm:"size:16;index;not null"         json:"level"`
	Building    string      `gorm:"size:8;index;not null"          json:"building"`
	Floor       string      `gorm:"size:16;index;default:''"       json:"floor"`
	RoomSuffix  string      `gorm:"size:32;index;default:''"       json:"room_suffix"`
	DrcengValue string      `gorm:"size:64;not null;default:''"   json:"drceng_value"`
	FormValue   string      `gorm:"size:128;not null"              json:"form_value"`
	Label       string      `gorm:"size:128;not null"              json:"label"`
}

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

type DormOptionDTO struct {
	Building    string `json:"building"`
	Floor       string `json:"floor"`
	RoomSuffix  string `json:"room_suffix"`
	DrcengValue string `json:"drceng_value"`
	FormValue   string `json:"form_value"`
	Label       string `json:"label"`
}

type SyncMeta struct {
	ID         string     `gorm:"primaryKey;size:36"  json:"id"`
	LastSyncAt *time.Time `gorm:"size:36"            json:"last_sync_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (s *SyncMeta) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = "sync-meta"
	}
	return nil
}
