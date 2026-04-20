package handler

import (
	"net/http"
	"strconv"

	"electricquery/internal/config"
	"electricquery/internal/middleware"
	"electricquery/internal/service"

	"github.com/gin-gonic/gin"
)

// QueryPower POST /api/power/query
// 用户主动触发查询当前宿舍电量（使用登录用户的宿舍号）
func QueryPower(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := service.GetProfile(userID)
	if err != nil || profile.DormRoom == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先绑定宿舍号"})
		return
	}

	cfg := config.Load()
	result, err := service.QueryAndSavePower(profile.DormRoom, cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"dorm_room":     result.DormRoom,
			"remaining_kwh": result.RemainingKwh,
			"remaining_f":   result.RemainingF,
			// C13/C14 水电同页时，爬取结果中已包含水量
			"water_amount": result.WaterAmount,
			"water_f":      result.WaterF,
		},
	})
}

// QueryWaterPower POST /api/power/water
// 查询指定宿舍号的水费（独立于电费宿舍号）
func QueryWaterPower(c *gin.Context) {
	var input struct {
		DormRoom string `json:"dorm_room" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少 dorm_room 参数"})
		return
	}

	cfg := config.Load()
	result, err := service.QueryAndSavePower(input.DormRoom, cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}

	// C13/C14 水电同页：爬虫已同时返回水电，
	// remaining_kwh=电量，water_f=水量；
	// 水费接口应返回水量（water_f）而非电量作为 remaining_kwh
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"dorm_room":     result.DormRoom,
			"remaining_kwh": result.WaterAmount,
			"remaining_f":   result.WaterF,
			"water_amount": result.WaterAmount,
			"water_f":      result.WaterF,
		},
	})
}

// GetPowerHistory GET /api/power/history?limit=30
func GetPowerHistory(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := service.GetProfile(userID)
	if err != nil || profile.DormRoom == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先绑定宿舍号"})
		return
	}

	limit := 30
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	logs, err := service.GetPowerHistory(profile.DormRoom, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": logs})
}

// InternalQueryPower GET /api/internal/power/:dorm
// 内部接口，供调度器或管理员触发查询
func InternalQueryPower(c *gin.Context) {
	dormRoom := c.Param("dorm")
	if dormRoom == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少宿舍号参数"})
		return
	}

	cfg := config.Load()
	result, err := service.QueryAndSavePower(dormRoom, cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"dorm_room":     result.DormRoom,
			"remaining_kwh": result.RemainingKwh,
			"remaining_f":   result.RemainingF,
			"building":      result.Building,
			"floor":         result.Floor,
			"room":          result.Room,
			"water_amount":  result.WaterAmount,
			"water_f":       result.WaterF,
		},
	})
}
