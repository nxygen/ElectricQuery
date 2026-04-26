package handler

import (
	"net/http"
	"strconv"

	"electricquery/internal/checker"
	"electricquery/internal/config"
	"electricquery/internal/middleware"
	"electricquery/internal/service"

	"github.com/gin-gonic/gin"
)

// QueryPower POST /api/power/query
// 使用登录用户的绑定宿舍号查询电量
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询电量失败，请稍后重试"})
		return
	}

	// 将 FormValue 转为网页标准 <option value> 格式，供前端回显
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

// QueryWaterPower POST /api/power/water
// 查询水费：使用用户 profile 中绑定的 water_dorm_room（若未设则降级用 dorm_room）
// 不接受客户端传入宿舍号，杜绝越权查询
func QueryWaterPower(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := service.GetProfile(userID)
	if err != nil || profile.DormRoom == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先绑定宿舍号"})
		return
	}

	// 优先用 water_dorm_room，未配置则降级用 dorm_room
	dormRoom := profile.WaterDormRoom
	if dormRoom == "" {
		dormRoom = profile.DormRoom
	}

	cfg := config.Load()
	result, err := service.QueryAndSavePower(dormRoom, cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询水费失败，请稍后重试"})
		return
	}

	// 将 FormValue 转为网页标准 <option value> 格式，供前端回显
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

// GetPowerHistory GET /api/power/history?limit=30
// 直接使用用户绑定的 dorm_room（电表）和 water_dorm_room（水表）独立查询并合并
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

	// 直接传入电表 dorm_room 和水表 water_dorm_room，各自独立查询并按日期合并
	// 不再依赖任何配对换算逻辑
	logs, err := service.GetPowerHistory(profile.DormRoom, profile.WaterDormRoom, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询历史记录失败，请稍后重试"})
		return
	}

	// 将每条历史的 dorm_room 从 FormValue 转为网页标准 option value，并附上人类可读 Label
	// 格式：直接用 Label（如 "C10-207" 或 "C10-207水表"）
	historyData := make([]map[string]any, len(logs))
	for i, l := range logs {
		lk := service.LookupByFormValue(l.DormRoom)
		var dormLabel string
		if lk != nil {
			// 直接用 Label（已是标准格式如 C10-207、C10-207水表）
			dormLabel = lk.Opt.Label
		}
		historyData[i] = map[string]any{
			"id":              l.ID,
			"dorm_room":       service.ToWebValue(l.DormRoom),
			"form_value":      l.DormRoom,
			"dorm_label":      dormLabel,
			"record_date":     l.RecordDate,
			"remaining_kwh":   l.RemainingKwh,
			"remaining_water": l.RemainingWater,
			"queried_at":      l.QueriedAt,
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": historyData})
}

// InternalQueryPower GET /api/internal/power/:dorm
// 内部接口（X-Internal-Token 鉴权），供调度器或管理员触发查询
//
// :dorm 参数可以是以下任意格式（由 QueryAndSavePower 内部调用 LookupByFormValue 解析）：
//   - FormValue 格式（标准）：6位数字 "140328" 或三段式 "11|1101|110169"
//   - Label 格式（友好名）：如 "C14 328" 或 "C11 132水"，通过 DormOption 表反查 FormValue
//
// 核心原则：物理ID是绝对真理。此接口内部严格走「映射表→物理ID→查询」流程。
func InternalQueryPower(c *gin.Context) {
	rawDorm := c.Param("dorm")
	if rawDorm == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少宿舍号参数"})
		return
	}

	// 先查映射表，给出友好错误信息（而不是让 QueryAndSavePower 内部报错）
	lk := service.LookupByFormValue(rawDorm)
	if lk == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "宿舍号未找到映射关系: " + rawDorm + "（请先执行同步，或确认格式正确）",
			"hint": "支持格式：FormValue（如 140328、11|1101|110169）或 Label（如 C14 328、C11 132水）",
		})
		return
	}

	cfg := config.Load()
	result, err := service.QueryAndSavePower(rawDorm, cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "内部查询失败，请稍后重试"})
		return
	}

	parts := checker.ParseDorm(result.DormRoom)
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"dorm_room":     service.ToWebValue(result.DormRoom),
			"form_value":    result.DormRoom,
			"label":        lk.Opt.Label,      // 物理ID对应的友好显示名
			"physical_id":  lk.DrcengValue,   // 实际发给网页的物理ID（不透明）
			"remaining_kwh": result.RemainingKwh,
			"remaining_f":   result.RemainingF,
			"building":      parts.Building,
			"floor":         parts.Floor,
			"room":          parts.Room,
			"water_amount":  result.WaterAmount,
			"water_f":       result.WaterF,
		},
	})
}
