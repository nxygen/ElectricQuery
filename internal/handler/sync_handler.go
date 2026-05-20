package handler

import (
	"net/http"
	"time"

	"electricquery/internal/cache"
	"electricquery/internal/model"
	dormsyncer "electricquery/internal/dormsyncer"

	"github.com/gin-gonic/gin"
)

type SyncHandler struct {
	syncer *dormsyncer.Syncer
}

// OptionsResp 宿舍选项分组响应（缓存序列化用）
type OptionsResp struct {
	Buildings []model.DormOptionDTO `json:"buildings"`
	Floors    []model.DormOptionDTO `json:"floors"`
	Rooms     []model.DormOptionDTO `json:"rooms"`
}

func NewSyncHandler(syncer *dormsyncer.Syncer) *SyncHandler {
	return &SyncHandler{syncer: syncer}
}

func (h *SyncHandler) SyncDormOptions(c *gin.Context) {
	go func() {
		h.syncer.SyncAll()
	}()
	c.JSON(http.StatusOK, gin.H{
		"msg": "同步任务已启动，请稍后刷新页面查看结果",
	})
}

func (h *SyncHandler) GetDormOptions(c *gin.Context) {
	building := c.Query("building")
	floor := c.Query("floor")
	level := c.Query("level")

	// 生成缓存键
	cacheKey := "dorm_options"
	if building != "" {
		cacheKey += ":building:" + building
	}
	if floor != "" {
		cacheKey += ":floor:" + floor
	}
	if level != "" {
		cacheKey += ":level:" + level
	}

	// 尝试从缓存获取
	if cached, found := cache.Get(cacheKey); found {
		if resp, ok := cached.(OptionsResp); ok {
			c.JSON(http.StatusOK, gin.H{"data": resp})
			return
		}
	}

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

	// 缓存结果（1 小时）
	cache.Set(cacheKey, resp, 1*time.Hour)

	c.JSON(http.StatusOK, gin.H{"data": resp})
}
