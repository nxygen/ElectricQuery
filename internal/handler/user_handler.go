// Package handler 实现 Gin HTTP 处理器
package handler

import (
	"net/http"

	"electricquery/internal/config"
	"electricquery/internal/middleware"
	"electricquery/internal/service"

	"github.com/gin-gonic/gin"
)

// Register POST /api/auth/register
// 注册时无需学号，学号在个人信息页单独绑定
func Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}

	user, err := service.Register(input)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 201, "msg": "注册成功", "data": user})
}

// Login POST /api/auth/login
func Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}

	token, user, err := service.Login(input)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "登录成功",
		"data": gin.H{
			"token": token,
			"user":  user,
		},
	})
}

// GetProfile GET /api/user/profile
func GetProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	user, err := service.GetProfile(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": user})
}

// UpdateProfile PATCH /api/user/profile
func UpdateProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var input service.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}

	user, err := service.UpdateProfile(userID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "更新成功", "data": user})
}

// BindStudentID POST /api/user/student-id
// 独立绑定学号，全局唯一性校验
func BindStudentID(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var input service.BindStudentIDInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
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
// 实时校验宿舍号是否真实存在
func ValidateDorm(c *gin.Context) {
	var input struct {
		DormRoom string `json:"dorm_room" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少 dorm_room 参数"})
		return
	}

	cfg := config.Load()
	valid, msg := service.ValidateDormRoom(input.DormRoom, cfg)

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
	userID := middleware.GetUserID(c)
	ch, err := service.GetChannel(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": ch})
}

// UpdateChannel PUT /api/user/channel
// 支持 test_channel=true 时触发测试通知
func UpdateChannel(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var input service.UpdateChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}

	ch, err := service.UpdateChannel(userID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "通知渠道已更新", "data": ch})
}
