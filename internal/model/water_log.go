// Package model 定义所有 GORM 数据模型
package model

// WaterLog 水表记录（独立表）
// 主键统一使用 UUID v7（string, size:36），存储 FormValue 格式宿舍号
type WaterLog struct {
	BaseModel
	DormRoom       string `gorm:"column:dorm_room;type:text;not null;uniqueIndex:idx_water_dorm_date" json:"dorm_room"` // 宿舍号（FormValue格式）
	RecordDate     string `gorm:"column:record_date;type:text;not null;uniqueIndex:idx_water_dorm_date" json:"record_date"` // 记录日期 YYYY-MM-DD
	RemainingWater string `gorm:"column:remaining_water;type:text" json:"remaining_water"` // 已用水量（负数，透支）
	QueriedAt      string `gorm:"column:queried_at;type:text" json:"queried_at"`       // 查询时间
}

// TableName 指定表名
func (WaterLog) TableName() string {
	return "water_logs"
}
