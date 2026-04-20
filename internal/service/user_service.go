// Package service 实现用户相关业务逻辑
package service

import (
	"errors"
	"fmt"
	"log"
	"time"

	"electricquery/internal/checker"
	"electricquery/internal/config"
	"electricquery/internal/middleware"
	"electricquery/internal/model"
	"electricquery/internal/notifier"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// RegisterInput 注册请求参数（注册时无需学号）
type RegisterInput struct {
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name"`
}

// LoginInput 登录请求参数
type LoginInput struct {
	StudentID string `json:"student_id" binding:"required"`
	Password  string `json:"password"   binding:"required"`
}

// BindStudentIDInput 绑定学号请求参数
type BindStudentIDInput struct {
	StudentID string `json:"student_id" binding:"required,min=6,max=32"`
}

// UpdateProfileInput 更新个人信息请求参数
type UpdateProfileInput struct {
	Name          string `json:"name"`
	Building      string `json:"building"`
	DormRoom      string `json:"dorm_room"`       // 电费宿舍（默认字段）
	WaterDormRoom string `json:"water_dorm_room"` // 水费宿舍
	Class         string `json:"class"`
}

// UpdateChannelInput 更新通知渠道请求参数
type UpdateChannelInput struct {
	WechatWebhook string `json:"wechat_webhook"`
	Email         string `json:"email"`
	TestChannel   any    `json:"test_channel"` // 前端传布尔值或字符串，均支持
}

// UserResponse 返回给前端的用户信息（不含密码）
type UserResponse struct {
	ID             uint      `json:"id"`
	StudentID      string    `json:"student_id"`
	Name           string    `json:"name"`
	Building       string    `json:"building"`
	DormRoom       string    `json:"dorm_room"`        // 电费宿舍（默认字段）
	WaterDormRoom  string    `json:"water_dorm_room"` // 水费宿舍
	Class          string    `json:"class"`
	CreatedAt      time.Time `json:"created_at"`
}

// ChannelResponse 返回给前端的通知渠道信息
type ChannelResponse struct {
	WechatWebhook string `json:"wechat_webhook"`
	Email         string `json:"email"`
}

// Register 注册新用户（无需学号，初始学号为空）
func Register(input RegisterInput) (*UserResponse, error) {
	// bcrypt 加密密码
	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	user := model.User{
		Password: string(hashed),
		Name:     input.Name,
	}
	if err := model.DB.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return toUserResponse(&user), nil
}

// BindStudentID 绑定/修改学号（唯一性校验）
func BindStudentID(userID uint, input BindStudentIDInput) (*UserResponse, error) {
	var user model.User
	if err := model.DB.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	// 学号已被其他用户占用
	var existing model.User
	if err := model.DB.Where("student_id = ? AND id != ?", input.StudentID, userID).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("该学号已被其他账号绑定")
	}

	if err := model.DB.Model(&user).Update("student_id", input.StudentID).Error; err != nil {
		return nil, fmt.Errorf("绑定学号失败: %w", err)
	}

	user.StudentID = input.StudentID
	return toUserResponse(&user), nil
}

// Login 用户登录，返回 JWT token
func Login(input LoginInput) (string, *UserResponse, error) {
	var user model.User
	if err := model.DB.Where("student_id = ?", input.StudentID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, fmt.Errorf("学号或密码错误")
		}
		return "", nil, fmt.Errorf("查询用户失败: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return "", nil, fmt.Errorf("学号或密码错误")
	}

	token, err := middleware.GenerateToken(user.ID, user.StudentID)
	if err != nil {
		return "", nil, fmt.Errorf("生成 Token 失败: %w", err)
	}

	return token, toUserResponse(&user), nil
}

// GetProfile 获取用户个人信息
func GetProfile(userID uint) (*UserResponse, error) {
	var user model.User
	if err := model.DB.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	return toUserResponse(&user), nil
}

// UpdateProfile 更新用户个人信息（宿舍楼/宿舍号/班级）
func UpdateProfile(userID uint, input UpdateProfileInput) (*UserResponse, error) {
	var user model.User
	if err := model.DB.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	updates := map[string]interface{}{}
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Building != "" {
		updates["building"] = input.Building
	}
	if input.DormRoom != "" {
		updates["dorm_room"] = input.DormRoom
	}
	if input.WaterDormRoom != "" {
		updates["water_dorm_room"] = input.WaterDormRoom
	}
	if input.Class != "" {
		updates["class"] = input.Class
	}

	if len(updates) > 0 {
		if err := model.DB.Model(&user).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("更新失败: %w", err)
		}
	}

	// 重新查询返回最新数据
	model.DB.First(&user, userID)
	return toUserResponse(&user), nil
}

// ValidateDormRoom 校验宿舍号是否真实存在（通过爬取验证）
func ValidateDormRoom(dormRoom string, appCfg *config.AppConfig) (bool, string) {
	chk := checker.NewChecker(appCfg)
	result, err := chk.CheckPowerByDorm(dormRoom)
	if err != nil {
		return false, fmt.Sprintf("查询失败: %v", err)
	}
	if result == nil {
		return false, "未获取到有效数据"
	}
	return true, fmt.Sprintf("查询成功，当前剩余 %.2f 度", result.RemainingF)
}

// GetChannel 获取用户通知渠道配置
func GetChannel(userID uint) (*ChannelResponse, error) {
	var ch model.UserChannel
	if err := model.DB.Where("user_id = ?", userID).First(&ch).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &ChannelResponse{}, nil
		}
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	return &ChannelResponse{
		WechatWebhook: ch.WechatWebhook,
		Email:         ch.Email,
	}, nil
}

// UpdateChannel 保存或更新用户通知渠道配置
func UpdateChannel(userID uint, input UpdateChannelInput) (*ChannelResponse, error) {
	var ch model.UserChannel
	result := model.DB.Where("user_id = ?", userID).First(&ch)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		ch = model.UserChannel{
			UserID:        userID,
			WechatWebhook: input.WechatWebhook,
			Email:         input.Email,
		}
		if err := model.DB.Create(&ch).Error; err != nil {
			return nil, fmt.Errorf("创建渠道配置失败: %w", err)
		}
	} else {
		if err := model.DB.Model(&ch).Updates(map[string]interface{}{
			"wechat_webhook": input.WechatWebhook,
			"email":          input.Email,
		}).Error; err != nil {
			return nil, fmt.Errorf("更新渠道配置失败: %w", err)
		}
	}

	// 发送测试通知（同步执行，错误返回给前端）
	// 支持前端传布尔值 true 或字符串 "true" 两种格式
	doTest := false
	switch v := input.TestChannel.(type) {
	case bool:
		doTest = v
	case string:
		doTest = v == "true" || v == "1" || v == "on"
	case nil:
		doTest = false
	}
	log.Printf("[notifier] 测试通知检查: TestChannel=%v doTest=%v webhook=%s email=%s",
		input.TestChannel, doTest, input.WechatWebhook, input.Email)
	if doTest && (input.WechatWebhook != "" || input.Email != "") {
		subject := "✅ ElectricQuery 测试通知"
		body := "您好！这是 ElectricQuery 宿舍电量查询系统的测试通知。\n" +
			"如果您收到此消息，说明您的通知渠道配置正确，后续将正常接收电量告警和周报。"
		if err := notifier.SendToUserSynced(input.WechatWebhook, input.Email, subject, body); err != nil {
			log.Printf("[notifier] 测试通知发送失败: %v", err)
			return nil, fmt.Errorf("测试通知发送失败: %v", err)
		}
		log.Printf("[notifier] 测试通知发送成功")
	}

	// 重新获取最新数据
	model.DB.Where("user_id = ?", userID).First(&ch)
	return &ChannelResponse{
		WechatWebhook: ch.WechatWebhook,
		Email:         ch.Email,
	}, nil
}

// sendTestNotification 发送测试通知
func sendTestNotification(userID uint) {
	var ch model.UserChannel
	if err := model.DB.Where("user_id = ?", userID).First(&ch).Error; err != nil {
		return
	}
	subject := "✅ ElectricQuery 测试通知"
	body := "您好！这是 ElectricQuery 宿舍电量查询系统的测试通知。\n" +
		"如果您收到此消息，说明您的通知渠道配置正确，后续将正常接收电量告警和周报。"
	notifier.SendToUser(ch.Email, ch.WechatWebhook, subject, body)
}

// toUserResponse 将 model.User 转为 API 响应结构
func toUserResponse(u *model.User) *UserResponse {
	return &UserResponse{
		ID:             u.ID,
		StudentID:      u.StudentID,
		Name:           u.Name,
		Building:       u.Building,
		DormRoom:       u.DormRoom,
		WaterDormRoom:  u.WaterDormRoom,
		Class:          u.Class,
		CreatedAt:      u.CreatedAt,
	}
}
