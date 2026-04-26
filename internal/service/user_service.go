// Package service 实现用户相关业务逻辑
package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

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
	Name     string `json:"name"`                             // 选填，可后续在个人资料中补充
}

// LoginInput 登录请求参数
type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	TOTPCode string `json:"totp_code"` // 可选；若用户开启了 TOTP 则必须提供
}

// LoginResult 登录结果（两步验证支持）
type LoginResult struct {
	Token        string         `json:"token,omitempty"`          // 登录成功时有值
	User         *UserResponse  `json:"user,omitempty"`            // 登录成功时有值
	RequiresTOTP bool           `json:"requires_totp,omitempty"` // TOTP 待验证时有值
	Username     string         `json:"username,omitempty"`       // RequiresTOTP=true 时返回，用于二次提交
	Msg          string         `json:"msg,omitempty"`            // 提示信息
}

// BindStudentIDInput 绑定学号请求参数
type BindStudentIDInput struct {
	StudentID string `json:"student_id" binding:"required,min=6,max=32"`
}

// UpdateProfileInput 更新个人信息请求参数
// 使用指针类型区分"未传"（nil）和"传了空值"（*string=""）：允许清空字段
type UpdateProfileInput struct {
	Name          *string `json:"name"`
	StudentID     *string `json:"student_id"` // nil=未传，""=清空，非空=设置学号
	Building      *string `json:"building"`
	DormRoom      *string `json:"dorm_room"`
	WaterDormRoom *string `json:"water_dorm_room"`
	Class         *string `json:"class"`
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
	DormLabel      string    `json:"dorm_label"`        // 映射表返回的标准 Label（如 C10-207）
	WaterDormRoom  string    `json:"water_dorm_room"`
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
			return nil, fmt.Errorf("用户名已被占用")
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
		secret, _ := decryptTOTPSecret(user.TOTPSecret)
		if secret == "" {
			// 解密失败（旧数据未加密）→ 跳过 TOTP 验证，引导用户重新设置
			logger.Warn("TOTP secret decryption failed, user may need to re-setup", "user_id", user.ID)
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
		return nil, fmt.Errorf("用户不存在")
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
		return fmt.Errorf("用户不存在")
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
		return "", fmt.Errorf("用户不存在")
	}

	// 已开启则直接返回已有 URI（不重复生成）
	if user.TOTPEnabled && user.TOTPSecret != "" {
		secret, _ := decryptTOTPSecret(user.TOTPSecret)
		if secret != "" {
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
		return fmt.Errorf("用户不存在")
	}
	if user.TOTPSecret == "" {
		return fmt.Errorf("请先设置 TOTP（点击「启用两步验证」）")
	}
	secret, _ := decryptTOTPSecret(user.TOTPSecret)
	if secret == "" {
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
		return fmt.Errorf("用户不存在")
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

	logger.Info("用户关闭了 TOTP 两步验证", "user_id", userID)
	return nil
}

// ForceDisableTOTP 管理员强制关闭指定用户的 TOTP（无需密码验证）
func ForceDisableTOTP(userID string) error {
	var user model.User
	if err := model.DB.First(&user, "id = ?", userID).Error; err != nil {
		return fmt.Errorf("用户不存在")
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

	logger.Info("管理员强制关闭了用户 TOTP 两步验证", "user_id", userID, "operator", "admin")
	return nil
}

// GetProfile 获取用户个人信息
func GetProfile(userID string) (*UserResponse, error) {
	var user model.User
	if err := model.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	return toUserResponse(&user), nil
}

// UpdateProfile 更新用户个人信息
// 宿舍号映射层：若传入的 dorm_room 与 DormOption.FormValue 精确匹配，则直接用；
// 若未匹配（用户手填或旧格式），则原样存储（兼容旧数据），并在日志中标记。
func UpdateProfile(userID string, input UpdateProfileInput) (*UserResponse, error) {
	var user model.User
	if err := model.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
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
	if input.DormRoom != nil {
		// 映射层：将传入值与 DormOption 中的 FormValue 对齐
		dormRoom := normalizeDormRoom(*input.DormRoom)
		updates["dorm_room"] = dormRoom
	}
	if input.WaterDormRoom != nil {
		waterRoom := normalizeDormRoom(*input.WaterDormRoom)
		updates["water_dorm_room"] = waterRoom
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
	}

	// 重新查询返回最新数据
	model.DB.First(&user, "id = ?", userID)
	return toUserResponse(&user), nil
}

// normalizeDormRoom 将宿舍号归一化：
//   - 若在 DormOption 表中能精确匹配 FormValue，则返回数据库中的 FormValue（确保格式正确）
//   - 否则原样返回（兼容手动输入）
func normalizeDormRoom(dormRoom string) string {
	dormRoom = strings.TrimSpace(dormRoom)
	if dormRoom == "" {
		return dormRoom
	}

	// 精确匹配 FormValue
	var opt model.DormOption
	if err := model.DB.Where("form_value = ? AND level = ?", dormRoom, model.OptionLevelRoom).
		First(&opt).Error; err == nil {
		return opt.FormValue // 已在数据库中，格式正确
	}

	// 未匹配：原样返回（手动输入或旧格式，ParseDorm 会尝试解析）
	logger.Info("宿舍号未在 DormOption 中匹配，原样存储", "dorm_room", dormRoom)
	return dormRoom
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

// UpdateChannel 保存或更新用户通知渠道配置
func UpdateChannel(userID string, input UpdateChannelInput) (*ChannelResponse, error) {
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
		strings.Contains(msg, "Duplicate entry")              // MySQL
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
		WaterDormRoom:  u.WaterDormRoom,
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
// 向后兼容：若解密失败（旧数据为明文），静默返回原值
func decryptTOTPSecret(ciphertext string) (string, error) {
	cfg := config.Load()
	plain, err := cryptoutil.Decrypt(ciphertext, cfg.App.JWTSecret)
	if err != nil || plain == "" {
		// 解密失败 → 可能是旧版本明文，直接返回
		return ciphertext, nil
	}
	return plain, nil
}

