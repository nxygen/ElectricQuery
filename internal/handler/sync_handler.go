package handler

import (
	"net/http"

	"electricquery/internal/model"
	dormsyncer "electricquery/internal/dormsyncer"

	"github.com/gin-gonic/gin"
)

// SyncHandler 同步相关请求处理
type SyncHandler struct {
	syncer *dormsyncer.Syncer
}

// NewSyncHandler 创建同步处理器
func NewSyncHandler(syncer *dormsyncer.Syncer) *SyncHandler {
	return &SyncHandler{syncer: syncer}
}

// POST /api/sync/dorm-options 触发同步官网页面下拉选项
func (h *SyncHandler) SyncDormOptions(c *gin.Context) {
	go func() {
		h.syncer.SyncAll()
	}()
	c.JSON(http.StatusOK, gin.H{
		"msg": "同步任务已启动，请稍后刷新页面查看结果",
	})
}

// GET /api/sync/dorm-options 获取下拉选项
// Query params:
//   - building: 楼栋筛选（可选）
//   - floor: 楼层筛选（可选，配合 building）
//   - level: 层级筛选 building/floor/room
func (h *SyncHandler) GetDormOptions(c *gin.Context) {
	building := c.Query("building")
	floor := c.Query("floor")
	level := c.Query("level")

	query := model.DB.Model(&model.DormOption{})

	if level != "" {
		query = query.Where("level = ?", level)
	}
	if building != "" {
		query = query.Where("building = ?", building)
		if floor != "" {
			query = query.Where("floor = ?", floor)
		}
	}

	var options []model.DormOption
	if err := query.Order("label").Find(&options).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "查询失败"})
		return
	}

	// 按层级分组返回
	type OptionGroup struct {
		Level   model.OptionLevel      `json:"level"`
		Options []model.DormOptionDTO `json:"options"`
	}
	type OptionsResp struct {
		Buildings []model.DormOptionDTO `json:"buildings"`
		Floors    []model.DormOptionDTO `json:"floors"`
		Rooms     []model.DormOptionDTO `json:"rooms"`
	}

	resp := OptionsResp{
		Buildings: []model.DormOptionDTO{},
		Floors:    []model.DormOptionDTO{},
		Rooms:     []model.DormOptionDTO{},
	}

	for _, opt := range options {
		dto := model.DormOptionDTO{
			Building:    opt.Building,
			Floor:       opt.Floor,
			RoomSuffix:  opt.RoomSuffix,
			DrcengValue: opt.DrcengValue,
			FormValue:   opt.FormValue,
			Label:       opt.Label,
		}
		switch opt.Level {
		case model.OptionLevelBuilding:
			resp.Buildings = append(resp.Buildings, dto)
		case model.OptionLevelFloor:
			resp.Floors = append(resp.Floors, dto)
		case model.OptionLevelRoom:
			resp.Rooms = append(resp.Rooms, dto)
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}
