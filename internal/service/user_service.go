// Package service 实现用户相关业务逻辑
package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"electricquery/internal/cache"
	"electricquery/internal/checker"
	"electricquery/internal/config"
	"electricquery/internal/cryptoutil"
	"electricquery/internal/logger"
	"electricquery/internal/middleware"
	"electricquery/internal/model"
	"electricquery/internal/notifier"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// bcrypt cost=12（比默认 10 更强，单次 hash ~300ms，暴力破解成本极高）
const bcryptCost = 12

// ---- 请求 / 响应结构体 ----

// RegisterInput 注册请求参数
type RegisterInput struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=8"` // 最短 8 位
	Name     string `json:"name"`                              // 选填，可后续在个人资料中补充
}

// LoginInput 登录请求参数
type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	TOTPCode string `json:"totp_code"` // 可选；若用户开启了 TOTP 则必须提供
}

// LoginResult 登录结果（两步验证支持）
type LoginResult struct {
	Token        string        `json:"token,omitempty"`         // 登录成功时有值
	User         *UserResponse `json:"user,omitempty"`          // 登录成功时有值
	RequiresTOTP bool          `json:"requires_totp,omitempty"` // TOTP 待验证时有值
	Username     string        `json:"username,omitempty"`      // RequiresTOTP=true 时返回，用于二次提交
	Msg          string        `json:"msg,omitempty"`           // 提示信息
}

// BindStudentIDInput 绑定学号请求参数
type BindStudentIDInput struct {
	StudentID string `json:"student_id" binding:"required,min=6,max=32"`
}

// UpdateProfileInput 更新个人信息请求参数
// 使用指针类型区分"未传"（nil）和"传了空值"（*string=""）：允许清空字段
type UpdateProfileInput struct {
	Name           *string `json:"name"`
	StudentID      *string `json:"student_id"` // nil=未传，""=清空，非空=设置学号
	Building       *string `json:"building"`
	DormRoom       *string `json:"dorm_room"`
	DormFloor      *string `json:"dorm_floor"`
	WaterDormRoom  *string `json:"water_dorm_room"`
	WaterDormFloor *string `json:"water_dorm_floor"`
	Class          *string `json:"class"`
}

// UpdateChannelInput 更新通知渠道请求参数
type UpdateChannelInput struct {
	WechatWebhook string `json:"wechat_webhook"`
	Email         string `json:"email"`
	TestChannel   any    `json:"test_channel"` // 前端可传 bool 或 "true" 字符串
}

// UserResponse 返回给前端的用户信息（不含密码）
type UserResponse struct {
	ID             string    `json:"id"` // UUID
	Username       string    `json:"username"`
	StudentID      *string   `json:"student_id"` // nil 表示未绑定
	Name           string    `json:"name"`
	Building       string    `json:"building"`
	DormRoom       string    `json:"dorm_room"`
	DormFloor      string    `json:"dorm_floor"`
	DormLabel      string    `json:"dorm_label"` // 映射表返回的标准 Label（如 C10-207）
	WaterDormRoom  string    `json:"water_dorm_room"`
	WaterDormFloor string    `json:"water_dorm_floor"`
	WaterDormLabel string    `json:"water_dorm_label"` // 映射表返回的标准 Label（如 C13-1301水）
	Class          string    `json:"class"`
	TOTPEnabled    bool      `json:"totp_enabled"`
	CreatedAt      time.Time `json:"created_at"`
}

// ChannelResponse 返回给前端的通知渠道信息
type ChannelResponse struct {
	WechatWebhook string `json:"wechat_webhook"`
	Email         string `json:"email"`
}

// ---- 服务方法 ----

// Register 注册新用户
// 不做 SELECT 预判重，直接 INSERT，依赖数据库唯一索引报错，杜绝竞态条件
func Register(input RegisterInput) (*UserResponse, error) {
	if err := validatePasswordComplexity(input.Password); err != nil {
		return nil, err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	user := model.User{
		Username: input.Username,
		Password: string(hashed),
		Name:     input.Name,
	}
	if err := model.DB.Create(&user).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("用户名已被占用: %w", err)
		}
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return toUserResponse(&user), nil
}

// Login 用户登录，返回 JWT token（payload 携带 UUID）
// 两步验证流程：第一步密码验证成功后若 TOTP 未开启则直接返回 token；否则返回 RequiresTOTP=true，前端弹出验证码框二次提交
func Login(input LoginInput) (*LoginResult, error) {
	var user model.User
	if err := model.DB.Where("username = ?", input.Username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("用户名或密码错误")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return nil, fmt.Errorf("用户名或密码错误")
	}

	// 若用户已开启 TOTP
	if user.TOTPEnabled {
		if input.TOTPCode == "" {
			// 密码正确但未提供验证码 → 返回 RequiresTOTP=true，引导前端展示验证码输入框
			return &LoginResult{
				RequiresTOTP: true,
				Username:     input.Username,
				Msg:          "请输入两步验证码",
			}, nil
		}
		// TOTP Secret 加密存储在此，验证前先解密
		secret, decErr := decryptTOTPSecret(user.TOTPSecret)
		if decErr != nil || secret == "" {
			logger.Warn("TOTP secret decryption failed, user may need to re-setup", "user_id", user.ID, "err", decErr)
			return nil, fmt.Errorf("两步验证配置异常，请重新设置两步验证")
		}
		// 已提供验证码，验证之
		if !totp.Validate(input.TOTPCode, secret) {
			return nil, fmt.Errorf("两步验证码错误")
		}
	}

	// token payload: UUID + username（username 仅用于日志，不作为鉴权依据）
	token, err := middleware.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, fmt.Errorf("生成 Token 失败: %w", err)
	}

	return &LoginResult{
		Token: token,
		User:  toUserResponse(&user),
	}, nil
}

// BindStudentID 绑定/更换学号（全局唯一性由数据库保证）
func BindStudentID(userID string, input BindStudentIDInput) (*UserResponse, error) {
	var user model.User
	if err := model.DB.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 检查学号是否被其他用户占用
	var existing model.User
	if err := model.DB.Where("student_id = ? AND id != ?", input.StudentID, userID).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("该学号已被其他账号绑定")
	}

	sid := input.StudentID
	if err := model.DB.Model(&user).Update("student_id", &sid).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("该学号已被其他账号绑定")
		}
		return nil, fmt.Errorf("绑定学号失败: %w", err)
	}
	// 删除缓存，下次获取时重新缓存最新数据
	cache.Delete("user:" + userID)

	user.StudentID = &sid
	return toUserResponse(&user), nil
}

// ChangePasswordInput 修改密码请求参数
type ChangePasswordInput struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ChangePassword 修改登录密码（需验证旧密码）
func ChangePassword(userID string, input ChangePasswordInput) error {
	if err := validatePasswordComplexity(input.NewPassword); err != nil {
		return err
	}
	var user model.User
	if err := model.DB.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("用户不存在")
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.OldPassword)); err != nil {
		return fmt.Errorf("旧密码不正确")
	}

	// 新密码 hash
	hashed, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	if err := model.DB.Model(&user).Update("password", string(hashed)).Error; err != nil {
		return fmt.Errorf("保存新密码失败: %w", err)
	}

	logger.Info("用户修改了登录密码", "user_id", userID)
	return nil
}

// SetupTOTP 为用户生成 TOTP 密钥，返回 TOTP URI（用于生成二维码）
// 密钥保存在 DB 的 totp_secret 字段，TOTPEnabled 暂不激活
func SetupTOTP(userID string) (totpURI string, err error) {
	var user model.User
	if err := model.DB.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("用户不存在")
		}
		return "", fmt.Errorf("查询用户失败: %w", err)
	}

	// 已开启则直接返回已有 URI（不重复生成）
	if user.TOTPEnabled && user.TOTPSecret != "" {
		secret, decErr := decryptTOTPSecret(user.TOTPSecret)
		if decErr == nil && secret != "" {
			uri, _ := totp.Generate(totp.GenerateOpts{
				Issuer:      "ElectricQuery",
				AccountName: user.Username,
				Secret:      []byte(secret),
			})
			if uri != nil {
				return uri.String(), nil
			}
		}
	}

	// 生成新密钥
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "ElectricQuery",
		AccountName: user.Username,
		SecretSize:  20,
		Period:      30,
	})
	if err != nil {
		return "", fmt.Errorf("生成 TOTP 密钥失败: %w", err)
	}

	// 加密存储密钥（暂不激活）
	encrypted, err := encryptTOTPSecret(key.Secret())
	if err != nil {
		return "", fmt.Errorf("加密 TOTP 密钥失败: %w", err)
	}
	if err := model.DB.Model(&user).Updates(map[string]interface{}{
		"totp_secret": encrypted,
	}).Error; err != nil {
		return "", fmt.Errorf("保存 TOTP 密钥失败: %w", err)
	}

	logger.Info("用户生成了 TOTP 密钥", "user_id", userID)
	return key.String(), nil
}

// EnableTOTPInput 激活 TOTP 请求参数
type EnableTOTPInput struct {
	TOTPCode string `json:"totp_code" binding:"required,len=6"`
}

// EnableTOTP 验证 TOTP 码后正式激活两步验证
func EnableTOTP(userID string, input EnableTOTPInput) error {
	var user model.User
	if err := model.DB.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("用户不存在")
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}
	if user.TOTPSecret == "" {
		return fmt.Errorf("请先设置 TOTP（点击「启用两步验证」）")
	}
	secret, decErr := decryptTOTPSecret(user.TOTPSecret)
	if decErr != nil || secret == "" {
		return fmt.Errorf("TOTP 密钥无效，请重新设置两步验证")
	}
	if !totp.Validate(input.TOTPCode, secret) {
		return fmt.Errorf("验证码错误，请确认 Authenticator 时间准确")
	}

	if err := model.DB.Model(&user).Updates(map[string]interface{}{
		"totp_enabled": true,
	}).Error; err != nil {
		return fmt.Errorf("激活失败: %w", err)
	}
	// 删除缓存，下次获取时重新缓存最新数据
	cache.Delete("user:" + userID)

	logger.Info("用户启用了 TOTP 两步验证", "user_id", userID)
	return nil
}

// DisableTOTPInput 关闭 TOTP 请求参数
type DisableTOTPInput struct {
	Password string `json:"password" binding:"required"` // 验证身份
}

// DisableTOTP 验证密码后关闭两步验证
func DisableTOTP(userID string, input DisableTOTPInput) error {
	var user model.User
	if err := model.DB.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("用户不存在")
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return fmt.Errorf("密码不正确")
	}

	if err := model.DB.Model(&user).Updates(map[string]interface{}{
		"totp_enabled": false,
		"totp_secret":  "",
	}).Error; err != nil {
		return fmt.Errorf("关闭失败: %w", err)
	}
	// 删除缓存，下次获取时重新缓存最新数据
	cache.Delete("user:" + userID)

	logger.Info("用户关闭了 TOTP 两步验证", "user_id", userID)
	return nil
}

// ForceDisableTOTP 管理员强制关闭指定用户的 TOTP（无需密码验证）
func ForceDisableTOTP(userID string) error {
	var user model.User
	if err := model.DB.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("用户不存在")
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}

	if !user.TOTPEnabled {
		return nil // 已经是关闭状态，无需操作
	}

	if err := model.DB.Model(&user).Updates(map[string]interface{}{
		"totp_enabled": false,
		"totp_secret":  "",
	}).Error; err != nil {
		return fmt.Errorf("关闭失败: %w", err)
	}
	// 删除缓存，下次获取时重新缓存最新数据
	cache.Delete("user:" + userID)

	logger.Info("管理员强制关闭了用户 TOTP 两步验证", "user_id", userID, "operator", "admin")
	return nil
}

// GetProfile 获取用户个人信息（带缓存）
func GetProfile(userID string) (*UserResponse, error) {
	// 尝试从缓存获取
	if cached, found := cache.Get("user:" + userID); found {
		if resp, ok := cached.(*UserResponse); ok {
			return resp, nil
		}
	}

	var user model.User
	if err := model.DB.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	resp := toUserResponse(&user)

	// 缓存结果（5 分钟）
	cache.Set("user:"+userID, resp, 5*time.Minute)

	return resp, nil
}

// UpdateProfile 更新用户个人信息
// 宿舍号映射层：若传入的 dorm_room 与 DormOption.FormValue 精确匹配，则直接用；
// 若未匹配（用户手填或旧格式），则原样存储（兼容旧数据），并在日志中标记。
func UpdateProfile(userID string, input UpdateProfileInput) (*UserResponse, error) {
	var user model.User
	if err := model.DB.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 学号唯一性检查
	if input.StudentID != nil && *input.StudentID != "" {
		var existing model.User
		if err := model.DB.Where("student_id = ? AND id != ?", *input.StudentID, userID).First(&existing).Error; err == nil {
			return nil, fmt.Errorf("该学号已被其他账号绑定")
		}
	}

	updates := map[string]interface{}{}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.StudentID != nil {
		// nil = 未传（不更新），"" = 清空，其他 = 设置学号
		if *input.StudentID == "" {
			updates["student_id"] = nil // SQL NULL
		} else {
			sid := *input.StudentID
			updates["student_id"] = &sid
		}
	}
	if input.Building != nil {
		updates["building"] = *input.Building
	}
	if input.DormFloor != nil {
		updates["dorm_floor"] = *input.DormFloor
	}
	if input.WaterDormFloor != nil {
		updates["water_dorm_floor"] = *input.WaterDormFloor
	}
	if input.DormRoom != nil {
		// 映射层：将传入值与 DormOption 中的 drceng_value 对齐
		dormRoom := normalizeDormRoom(*input.DormRoom)
		updates["dorm_room"] = dormRoom
		// 自动设置 dorm_floor（从前端传来的 input.DormFloor）
		if input.DormFloor != nil {
			updates["dorm_floor"] = *input.DormFloor
		}
		// 自动关联水宿舍：前端未显式传入 water_dorm_room 时，从 dorm_options 自动查找
		// C11/C12 水电分房楼栋：同 building+floor，Label 含"水"的记录即为水宿舍
		// C13/C14 水电合一楼栋：无独立水宿舍，设为 NULL（水电同页抓取，water_logs 用相同 dorm_room）
		if input.WaterDormRoom == nil {
			if waterFormValue := LookupWaterFormValue(dormRoom); waterFormValue != "" {
				updates["water_dorm_room"] = waterFormValue
			} else {
				updates["water_dorm_room"] = nil
			}
		}
	}
	if input.WaterDormRoom != nil {
		waterRoom := normalizeDormRoom(*input.WaterDormRoom)
		updates["water_dorm_room"] = waterRoom
		// 自动设置 water_dorm_floor
		if input.WaterDormFloor != nil {
			updates["water_dorm_floor"] = *input.WaterDormFloor
		}
	}
	if input.Class != nil {
		updates["class"] = *input.Class
	}

	if len(updates) > 0 {
		if err := model.DB.Model(&user).UpdateColumns(updates).Error; err != nil {
			if isUniqueViolation(err) {
				return nil, fmt.Errorf("该学号已被其他账号绑定")
			}
			return nil, fmt.Errorf("更新失败: %w", err)
		}
		// 删除缓存，下次获取时重新缓存最新数据
		cache.Delete("user:" + userID)
	}

	// 重新查询返回最新数据
	model.DB.First(&user, "id = ?", userID)
	return toUserResponse(&user), nil
}

// normalizeDormRoom 将宿舍 drceng_value 归一化：
//   - 若在 DormOption 表中能精确匹配 drceng_value，则返回 drceng_value（确保值正确）
//   - 否则原样返回（兼容手动输入）
func normalizeDormRoom(dormRoom string) string {
	dormRoom = strings.TrimSpace(dormRoom)
	if dormRoom == "" {
		return dormRoom
	}

	// 精确匹配 drceng_value
	var opt model.DormOption
	if err := model.DB.Where("drceng_value = ? AND level = ?", dormRoom, model.OptionLevelRoom).
		First(&opt).Error; err == nil {
		return opt.DrcengValue // 已在数据库中，值正确
	}

	// 未匹配：原样返回（手动输入或旧格式）
	logger.Info("宿舍号未在 DormOption 中匹配，原样存储", "dorm_room", dormRoom)
	return dormRoom
}

// ValidateDormRoom 校验宿舍号是否真实存在（通过爬取验证）
func ValidateDormRoom(ctx context.Context, dormRoom string, appCfg *config.AppConfig) (bool, string) {
	chk := checker.NewChecker(appCfg)
	result, err := chk.CheckPowerByDorm(ctx, dormRoom)
	if err != nil {
		return false, fmt.Sprintf("查询失败: %v", err)
	}
	if result == nil {
		return false, "未获取到有效数据"
	}
	return true, fmt.Sprintf("查询成功，当前剩余 %.2f 度", result.RemainingF)
}

// GetChannel 获取用户通知渠道配置
func GetChannel(userID string) (*ChannelResponse, error) {
	var ch model.UserChannel
	if err := model.DB.Unscoped().Where("user_id = ?", userID).First(&ch).Error; err != nil {
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

// validateWechatWebhook 校验企业微信 Webhook URL 合法性（SSRF 防护）
func validateWechatWebhook(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("webhook URL 格式无效")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("webhook 必须使用 HTTPS")
	}
	host := u.Hostname()
	if host != "qyapi.weixin.qq.com" {
		return fmt.Errorf("webhook 域名不合法，仅支持 qyapi.weixin.qq.com")
	}
	return nil
}

// validateEmail 校验邮箱格式
func validateEmail(email string) error {
	if email == "" {
		return nil
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("邮箱格式无效")
	}
	return nil
}

// UpdateChannel 保存或更新用户通知渠道配置
func UpdateChannel(userID string, input UpdateChannelInput) (*ChannelResponse, error) {
	if err := validateWechatWebhook(input.WechatWebhook); err != nil {
		return nil, err
	}
	if err := validateEmail(input.Email); err != nil {
		return nil, err
	}

	var ch model.UserChannel
	result := model.DB.Unscoped().Where("user_id = ?", userID).First(&ch)

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

	// 测试通知（支持 bool/string 两种入参）
	if shouldTest(input.TestChannel) && (input.WechatWebhook != "" || input.Email != "") {
		// 脱敏日志：只记录是否有值，不记录实际内容
		logger.Info("发送测试通知",
			"has_webhook", input.WechatWebhook != "",
			"has_email", input.Email != "")

		subject := "✅ ElectricQuery 测试通知"
		body := "您好！这是 ElectricQuery 宿舍电量查询系统的测试通知。\n" +
			"如果您收到此消息，说明您的通知渠道配置正确，后续将正常接收电量告警和周报。"
		if err := notifier.SendToUserSynced(input.WechatWebhook, input.Email, subject, body); err != nil {
			logger.Warn("测试通知发送失败", "err", err)
			return nil, fmt.Errorf("测试通知发送失败: %v", err)
		}
		logger.Info("测试通知发送成功")
	}

	model.DB.Unscoped().Where("user_id = ?", userID).First(&ch)
	return &ChannelResponse{
		WechatWebhook: ch.WechatWebhook,
		Email:         ch.Email,
	}, nil
}

// validatePasswordComplexity 校验密码复杂度
// 要求：至少 8 位，且包含 大写字母、小写字母、数字、特殊字符 中的至少 3 种
func validatePasswordComplexity(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("密码长度至少 8 位")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasUpper = true
		case ch >= 'a' && ch <= 'z':
			hasLower = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	count := 0
	if hasUpper {
		count++
	}
	if hasLower {
		count++
	}
	if hasDigit {
		count++
	}
	if hasSpecial {
		count++
	}
	if count < 3 {
		return fmt.Errorf("密码必须包含至少 3 种：大写字母、小写字母、数字、特殊字符")
	}
	return nil
}

// shouldTest 判断是否需要发送测试通知（兼容 bool/string 入参）
func shouldTest(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "1" || val == "on"
	}
	return false
}

// isUniqueViolation 检测数据库唯一索引冲突错误
func isUniqueViolation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || // SQLite
		strings.Contains(msg, "Duplicate entry") // MySQL
}

// toUserResponse 将 model.User 转为 API 响应结构
func toUserResponse(u *model.User) *UserResponse {
	return &UserResponse{
		ID:             u.ID,
		Username:       u.Username,
		StudentID:      u.StudentID,
		Name:           u.Name,
		Building:       u.Building,
		DormRoom:       u.DormRoom,
		DormFloor:      u.DormFloor,
		WaterDormRoom:  u.WaterDormRoom,
		WaterDormFloor: u.WaterDormFloor,
		Class:          u.Class,
		TOTPEnabled:    u.TOTPEnabled,
		CreatedAt:      u.CreatedAt,
	}
}

// encryptTOTPSecret 使用 JWT Secret 作为 KEK 对 TOTP Secret 加密存储
func encryptTOTPSecret(plaintext string) (string, error) {
	cfg := config.Load()
	return cryptoutil.Encrypt(plaintext, cfg.App.JWTSecret)
}

// decryptTOTPSecret 解密 TOTP Secret
// 向后兼容：若解密失败且输入非 base64（旧数据为明文），静默返回原值
// 若输入是合法 base64 但解密失败（密钥变更等），返回错误
func decryptTOTPSecret(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	cfg := config.Load()
	plain, err := cryptoutil.Decrypt(ciphertext, cfg.App.JWTSecret)
	if err != nil {
		// 判断是否为旧版明文（非 base64 数据）
		// 旧版明文是 base32 编码的 TOTP secret，不含 +/= 等 base64 字符
		if _, b64Err := base64.StdEncoding.DecodeString(ciphertext); b64Err != nil {
			// 非 base64 → 旧版明文，向后兼容
			return ciphertext, nil
		}
		// 是 base64 但解密失败 → 密钥可能变更，返回错误
		return "", fmt.Errorf("TOTP secret 解密失败，可能密钥已变更: %w", err)
	}
	return plain, nil
}
