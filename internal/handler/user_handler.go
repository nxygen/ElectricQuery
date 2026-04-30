// Package handler 实现 Gin HTTP 处理器
package handler

import (
	"net/http"
	"strings"

	"electricquery/internal/config"
	"electricquery/internal/middleware"
	"electricquery/internal/service"

	"github.com/gin-gonic/gin"
)

// mustUserID 获取当前用户 UUID，若为空则中止请求（理论上不会发生，因为已过 JWTAuth）
func mustUserID(c *gin.Context) (string, bool) {
	id := middleware.GetUserID(c)
	if id == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "无法获取用户身份"})
		return "", false
	}
	return id, true
}

// Register POST /api/auth/register
func Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误，请检查输入格式"})
		return
	}

	user, err := service.Register(input)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()}) // 用户名冲突等业务错误可返回具体信息
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 201, "msg": "注册成功", "data": user})
}

// Login POST /api/auth/login
func Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误，请检查输入格式"})
		return
	}

	result, err := service.Login(input)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "验证码"):
			// TOTP 验证码错误：返回具体提示，不影响用户名枚举防护
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": errMsg})
		case strings.Contains(errMsg, "两步验证配置"):
			// TOTP 配置异常：引导用户重新设置
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": errMsg})
		default:
			// 用户名不存在或密码错误：模糊提示防枚举
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "用户名或密码错误"})
		}
		return
	}

	// 两步验证中间状态：返回 RequiresTOTP 引导前端展示验证码输入框
	if result.RequiresTOTP {
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  result.Msg,
			"data": gin.H{
				"requires_totp": true,
				"username":      result.Username,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "登录成功",
		"data": gin.H{
			"token": result.Token,
			"user":  result.User,
		},
	})
}

// GetProfile GET /api/user/profile
func GetProfile(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		return
	}
	user, err := service.GetProfile(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "用户信息不存在"})
		return
	}

	// 附加 dorm_label：直接从映射表查 Label
	if user.DormRoom != "" {
		lk := service.LookupByFormValue(user.DormRoom)
		if lk != nil {
			user.DormLabel = lk.Opt.Label
		}
	}
	if user.WaterDormRoom != "" {
		lk := service.LookupByFormValue(user.WaterDormRoom)
		if lk != nil {
			user.WaterDormLabel = lk.Opt.Label
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": user})
}

// UpdateProfile PATCH /api/user/profile
func UpdateProfile(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		return
	}
	var input service.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误，请检查输入格式"})
		return
	}

	user, err := service.UpdateProfile(userID, input)
	if err != nil {
		// 区分唯一性冲突（409）和其他错误（500）
		errMsg := err.Error()
		if strings.Contains(errMsg, "已被") || strings.Contains(errMsg, "冲突") || strings.Contains(errMsg, "unique") {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新个人信息失败，请稍后重试"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功", "data": user})
}

// BindStudentID POST /api/user/student-id
// 独立绑定学号，全局唯一性由数据库唯一索引保证
func BindStudentID(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		return
	}
	var input service.BindStudentIDInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误，请检查输入格式"})
		return
	}

	user, err := service.BindStudentID(userID, input)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "学号绑定成功", "data": user})
}

// ValidateDorm POST /api/user/validate-dorm
// 实时校验宿舍号是否真实存在（调用爬虫验证）
func ValidateDorm(c *gin.Context) {
	if _, ok := mustUserID(c); !ok {
		return
	}
	var input struct {
		DormRoom string `json:"dorm_room" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少 dorm_room 参数"})
		return
	}

	cfg := config.Load()
	valid, msg := service.ValidateDormRoom(c.Request.Context(), input.DormRoom, cfg)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"valid":   valid,
			"message": msg,
		},
	})
}

// GetChannel GET /api/user/channel
func GetChannel(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		return
	}
	ch, err := service.GetChannel(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取通知渠道失败，请稍后重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": ch})
}

// UpdateChannel PUT /api/user/channel
// 支持 test_channel=true 时触发测试通知
func UpdateChannel(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		return
	}
	var input service.UpdateChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误，请检查输入格式"})
		return
	}

	ch, err := service.UpdateChannel(userID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新通知渠道失败，请稍后重试"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "通知渠道已更新", "data": ch})
}

// POST /api/user/change-password  修改登录密码
func ChangePassword(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		return
	}
	var input service.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误，请检查输入格式"})
		return
	}
	if err := service.ChangePassword(userID, input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "密码修改成功"})
}

// GET /api/user/totp/setup  生成 TOTP 密钥，返回二维码 URI
func SetupTOTP(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		return
	}
	uri, err := service.SetupTOTP(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "生成两步验证密钥失败，请稍后重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"totp_uri": uri}})
}

// POST /api/user/totp/enable  验证后激活 TOTP
func EnableTOTP(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		return
	}
	var input service.EnableTOTPInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误，请检查输入格式"})
		return
	}
	if err := service.EnableTOTP(userID, input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "两步验证已启用，下次登录时将需要输入验证码"})
}

// POST /api/user/totp/disable  验证密码后关闭 TOTP
func DisableTOTP(c *gin.Context) {
	userID, ok := mustUserID(c)
	if !ok {
		return
	}
	var input service.DisableTOTPInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误，请检查输入格式"})
		return
	}
	if err := service.DisableTOTP(userID, input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "两步验证已关闭"})
}

// GetSystemConfig GET /api/system/config
// 返回系统配置（告警阈值、周报时间等），供前端动态显示
func GetSystemConfig(c *gin.Context) {
	cfg := config.Load()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"alert_threshold":        cfg.Scheduler.AlertThreshold,
		"weekly_report_weekday":  cfg.Scheduler.WeeklyReportWeekday,
		"weekly_report_hour":     cfg.Scheduler.WeeklyReportHour,
	}})
}
