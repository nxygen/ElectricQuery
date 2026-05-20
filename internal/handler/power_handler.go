package handler

import (
	"net/http"
	"strconv"

	"electricquery/internal/config"
	"electricquery/internal/middleware"
	"electricquery/internal/service"

	"github.com/gin-gonic/gin"
)

func QueryPower(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := service.GetProfile(userID)
	if err != nil || profile.DormRoom == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先绑定宿舍号"})
		return
	}

	cfg := config.Load()
	waterDorm := service.ResolveWaterDormRoom(profile.DormRoom)
	result, err := service.QueryAndSavePower(c.Request.Context(), profile.DormRoom, waterDorm, cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询电量失败，请稍后重试"})
		return
	}

	webValue := service.ToWebValue(result.DormRoom)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"dorm_room":     webValue,
			"form_value":    result.DormRoom,
			"remaining_kwh": result.RemainingKwh,
			"remaining_f":   result.RemainingF,
			"water_amount":  result.WaterAmount,
			"water_f":       result.WaterF,
		},
	})
}

func QueryWaterPower(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := service.GetProfile(userID)
	if err != nil || profile.DormRoom == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先绑定宿舍号"})
		return
	}

	cfg := config.Load()
	waterDorm := service.ResolveWaterDormRoom(profile.DormRoom)
	result, err := service.QueryAndSavePower(c.Request.Context(), profile.DormRoom, waterDorm, cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询水费失败，请稍后重试"})
		return
	}

	webValue := service.ToWebValue(result.DormRoom)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"dorm_room":    webValue,
			"form_value":   result.DormRoom,
			"water_amount": result.WaterAmount,
			"water_f":      result.WaterF,
		},
	})
}

func GetPowerHistory(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := service.GetProfile(userID)
	if err != nil || profile.DormRoom == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先绑定宿舍号"})
		return
	}

	limit := 30
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 365 {
			limit = v
		}
	}

	waterDorm := service.ResolveWaterDormRoom(profile.DormRoom)
	logs, err := service.GetPowerHistory(profile.DormRoom, waterDorm, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询历史记录失败，请稍后重试"})
		return
	}

	historyData := make([]map[string]any, len(logs))
	for i, l := range logs {
		lk := service.LookupByFormValue(l.DormRoom)
		var dormLabel string
		if lk != nil {
			dormLabel = lk.Opt.Label
		}
		historyData[i] = map[string]any{
			"id":             l.ID,
			"dorm_room":      service.ToWebValue(l.DormRoom),
			"form_value":     l.DormRoom,
			"dorm_label":     dormLabel,
			"record_date":    l.RecordDate,
			"remaining_kwh":  l.RemainingKwh,
			"remaining_water": l.RemainingWater,
			"queried_at":     l.QueriedAt,
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": historyData})
}

func InternalQueryPower(c *gin.Context) {
	rawDorm := c.Param("dorm")
	if rawDorm == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少宿舍号参数"})
		return
	}

	lk := service.LookupByFormValue(rawDorm)
	if lk == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "宿舍号未找到映射关系: " + rawDorm + "（请先执行同步）",
		})
		return
	}

	cfg := config.Load()
	waterDorm := service.ResolveWaterDormRoom(lk.DrcengValue)
	result, err := service.QueryAndSavePower(c.Request.Context(), lk.DrcengValue, waterDorm, cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "内部查询失败，请稍后重试"})
		return
	}


	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"dorm_room":      result.DormRoom,
			"label":          lk.Opt.Label,
			"remaining_kwh":  result.RemainingKwh,
			"remaining_f":    result.RemainingF,
			"water_amount":  result.WaterAmount,
			"water_f":        result.WaterF,
		},
	})
}
