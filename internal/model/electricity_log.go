package model

type ElectricityLog struct {
	BaseModel
	DormRoom     string `gorm:"column:dorm_room;type:text;not null;uniqueIndex:idx_elec_dorm_date" json:"dorm_room"`
	RecordDate   string `gorm:"column:record_date;type:text;not null;uniqueIndex:idx_elec_dorm_date" json:"record_date"`
	RemainingKwh string `gorm:"column:remaining_kwh;type:text" json:"remaining_kwh"`
	QueriedAt    string `gorm:"column:queried_at;type:text" json:"queried_at"`
}

func (ElectricityLog) TableName() string {
	return "electricity_logs"
}
