package handler

import (
	"net/http"
	"strings"
	"time"

	"electricquery/internal/cache"
	"electricquery/internal/config"
	"electricquery/internal/middleware"
	"electricquery/internal/service"

	"github.com/gin-gonic/gin"
)

func mustUserID(c *gin.Context) (string, bool) {
	id := middleware.GetUserID(c)
	if id == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "无法获取用户身份"})
		return "", false
	}
	return id, true
}

func Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误，请检查输入格式"})
		return
	}

	user, err := service.Register(input)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 201, "msg": "注册成功", "data": user})
}

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
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": errMsg})
		case strings.Contains(errMsg, "两步验证配置"):
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": errMsg})
		default:
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "用户名或密码错误"})
		}
		return
	}

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

	// 生成 CSRF Token
	csrfToken, err := middleware.GenerateCSRF()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "生成 CSRF Token 失败"})
		return
	}

	// 设置 CSRF Token 到 Cookie
	c.SetCookie(
		"csrf_token",  // Cookie 名称
		csrfToken,    // Cookie 值
		3600*24*7,    // 有效期（7 天）
		"/",           // 路径
		"",            // 域名（空 = 当前域名）
		false,         // Secure（开发环境设为 false）
		false,         // HttpOnly（设为 false，允许 JavaScript 读取）
	)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "登录成功",
		"data": gin.H{
			"token": result.Token,
			"user":  result.User,
		},
	})
}

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

func GetSystemConfig(c *gin.Context) {
	// 尝试从缓存获取
	if cached, found := cache.Get("system_config"); found {
		if data, ok := cached.(gin.H); ok {
			c.JSON(http.StatusOK, gin.H{"code": 200, "data": data})
			return
		}
	}

	cfg := config.Load()
	data := gin.H{
		"alert_threshold":       cfg.Scheduler.AlertThreshold,
		"weekly_report_weekday": cfg.Scheduler.WeeklyReportWeekday,
		"weekly_report_hour":    cfg.Scheduler.WeeklyReportHour,
	}

	// 缓存结果（5 分钟）
	cache.Set("system_config", data, 5*time.Minute)

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": data})
}
