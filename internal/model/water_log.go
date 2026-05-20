package model

type WaterLog struct {
	BaseModel
	DormRoom       string `gorm:"column:dorm_room;type:text;not null;uniqueIndex:idx_water_dorm_date" json:"dorm_room"`
	RecordDate     string `gorm:"column:record_date;type:text;not null;uniqueIndex:idx_water_dorm_date" json:"record_date"`
	RemainingWater string `gorm:"column:remaining_water;type:text" json:"remaining_water"`
	QueriedAt      string `gorm:"column:queried_at;type:text" json:"queried_at"`
}

func (WaterLog) TableName() string {
	return "water_logs"
}
