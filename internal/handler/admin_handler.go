package handler

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	"electricquery/internal/config"
	dormsyncer "electricquery/internal/dormsyncer"
	"electricquery/internal/logger"
	"electricquery/internal/model"
	"electricquery/internal/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// AdminHandler 管理后台接口处理器
type AdminHandler struct {
	syncer *dormsyncer.Syncer
}

// NewAdminHandler 创建管理后台处理器
func NewAdminHandler(syncer *dormsyncer.Syncer) *AdminHandler {
	return &AdminHandler{syncer: syncer}
}

// ---- 用户管理 ----

// UserAdminDTO 管理后台用户展示 DTO（不含密码）
type UserAdminDTO struct {
	ID             string     `json:"id"`
	Username       string     `json:"username"`
	Name           string     `json:"name"`
	StudentID      *string    `json:"student_id"`
	DormRoom       string     `json:"dorm_room"`        // 原始 FormValue（内部用）
	DormLabel      string     `json:"dorm_label"`       // 格式化展示名，如 "C11-132"
	WaterDormLabel string     `json:"water_dorm_label"` // 格式化水宿舍展示名
	Class          string     `json:"class"`
	TOTPEnabled    bool       `json:"totp_enabled"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// GET /api/admin/users
// Query: page=1, size=20, search=<username|name|student_id>
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size

	query := model.DB.Model(&model.User{})
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("username LIKE ? OR name LIKE ? OR student_id LIKE ?", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		logger.Error("admin list users count failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "查询用户列表失败，请稍后重试"})
		return
	}

	var users []model.User
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&users).Error; err != nil {
		logger.Error("admin list users query failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "查询用户列表失败，请稍后重试"})
		return
	}

	// 批量收集所有 FormValue，一次性 IN 查询所有 Label，消除 N+1
	formValues := make([]string, 0, len(users)*2)
	seen := make(map[string]bool)
	for _, u := range users {
		if u.DormRoom != "" && !seen[u.DormRoom] {
			formValues = append(formValues, u.DormRoom)
			seen[u.DormRoom] = true
		}
		if u.WaterDormRoom != "" && !seen[u.WaterDormRoom] {
			formValues = append(formValues, u.WaterDormRoom)
			seen[u.WaterDormRoom] = true
		}
	}

	labelMap := make(map[string]string, len(formValues))
	if len(formValues) > 0 {
		var opts []model.DormOption
		if err := model.DB.
			Where("(form_value IN ? OR drceng_value IN ?) AND level = ?", formValues, formValues, model.OptionLevelRoom).
			Find(&opts).Error; err == nil {
			for _, o := range opts {
				labelMap[o.FormValue] = o.Label
				labelMap[o.DrcengValue] = o.Label
			}
		}
	}

	dtos := make([]UserAdminDTO, 0, len(users))
	for _, u := range users {
		var deletedAt *time.Time
		if u.DeletedAt.Valid {
			t := u.DeletedAt.Time
			deletedAt = &t
		}

		dtos = append(dtos, UserAdminDTO{
			ID:             u.ID,
			Username:       u.Username,
			Name:           u.Name,
			StudentID:      u.StudentID,
			DormRoom:       u.DormRoom,
			DormLabel:      labelMap[u.DormRoom],
			WaterDormLabel: labelMap[u.WaterDormRoom],
			Class:          u.Class,
			TOTPEnabled:    u.TOTPEnabled,
			CreatedAt:      u.CreatedAt,
			UpdatedAt:      u.UpdatedAt,
			DeletedAt:      deletedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"users": dtos,
			"total": total,
			"page":  page,
			"size":  size,
			"pages": (int(total) + size - 1) / size,
		},
	})
}

// DELETE /api/admin/users/:id  软删除用户
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "缺少用户 ID"})
		return
	}

	result := model.DB.Delete(&model.User{}, "id = ?", userID)
	if result.Error != nil {
		logger.Error("admin delete user failed", "user_id", userID, "err", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "删除用户失败，请稍后重试"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"msg": "用户不存在或已删除"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "用户已删除"})
}

// POST /api/admin/users/:id/reset-password  重置用户密码为随机12位密码
func (h *AdminHandler) ResetPassword(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "缺少用户 ID"})
		return
	}

	// 生成12位随机密码（base64 编码后截取，保证字符多样性）
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "生成密码失败"})
		return
	}
	rawPwd := base64.RawURLEncoding.EncodeToString(buf)[:12]

	// bcrypt 哈希
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPwd), 12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "哈希失败"})
		return
	}

	result := model.DB.Model(&model.User{}).Where("id = ?", userID).
		Update("password", string(hash))
	if result.Error != nil {
		logger.Error("admin reset password failed", "user_id", userID, "err", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "重置密码失败，请稍后重试"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"msg": "用户不存在或已删除"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"msg":          "密码已重置，请通过安全渠道（如当面、加密消息）告知用户新密码",
		"user_id":      userID,
		"new_password": rawPwd,
		"data": gin.H{
			"user_id":      userID,
			"new_password": rawPwd,
		},
	})
}

// POST /api/admin/users/:id/disable-totp  管理员强制关闭用户的两步验证
func (h *AdminHandler) ForceDisableTOTP(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "缺少用户 ID"})
		return
	}

	if err := service.ForceDisableTOTP(userID); err != nil {
		logger.Warn("admin force disable totp failed", "user_id", userID, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"msg": "操作失败，请稍后重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "已强制关闭该用户的两步验证"})
}

// ---- 同步状态与触发 ----

// GET /api/admin/sync/status  查询当前同步状态
func (h *AdminHandler) GetSyncStatus(c *gin.Context) {
	st := h.syncer.Status()
	c.JSON(http.StatusOK, gin.H{"data": st})
}

// POST /api/admin/sync/trigger  手动触发全量同步（异步）
func (h *AdminHandler) TriggerSync(c *gin.Context) {
	st := h.syncer.Status()
	if st.IsRunning {
		c.JSON(http.StatusConflict, gin.H{"msg": "同步任务正在进行中，请稍后再试"})
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("admin TriggerSync goroutine panic recovered", "panic", r)
			}
		}()
		if err := h.syncer.SyncAll(); err != nil {
			logger.Error("admin TriggerSync failed", "err", err)
		}
	}()
	c.JSON(http.StatusOK, gin.H{"msg": "同步任务已触发，请稍后查看状态"})
}

// ---- 宿舍电量手动查询 ----

// AdminQueryPowerRequest 管理员手动查询请求体
type AdminQueryPowerRequest struct {
	// DormRoom 支持任意格式输入，由后端统一解析为 FormValue：
	//   - FormValue 格式（推荐）：6位纯数字（如 "140328"）或三段式（如 "11|1101|132水表"）
	//   - Label 格式（友好名）：如 "C14-328"，会通过 DormOption 表自动反查到 FormValue
	// 严禁直接传中文楼栋名或拼接后的非标准字符串。
	DormRoom string `json:"dorm_room" binding:"required"`
}

// POST /api/admin/power/query  管理员手动触发指定宿舍的电量查询
//
// 本接口是所有管理员手动触发查询的统一入口，内置参数预处理层：
//  1. LookupByFormValue：将任意格式输入（FormValue 或 Label）查 DormOption 表
//  2. QueryAndSavePower：执行查询并落库（内部再次 LookupByFormValue 保证一致性）
//
// 核心原则：物理ID是绝对真理。AI 不得对 DrcengValue 做任何数学推算。
func (h *AdminHandler) QueryPower(c *gin.Context) {
	var req AdminQueryPowerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "参数错误，请检查请求格式"})
		return
	}

	// Step 1：预查映射表，给出友好错误（而非让 QueryAndSavePower 内部报错）
	lk := service.LookupByFormValue(req.DormRoom)
	if lk == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"msg":  "宿舍号未找到映射关系: " + req.DormRoom + "（请先执行同步）",
			"hint": "支持：FormValue（如 140328、11|1101|110169）或 Label（如 C14 328、C11 132水）",
			"raw":  req.DormRoom,
		})
		return
	}

	// Step 2：查询 —— 使用 LookupByFormValue 解析出的 drceng_value
	cfg := config.Load()
	waterDorm := service.ResolveWaterDormRoom(lk.DrcengValue)
	result, err := service.QueryAndSavePower(c.Request.Context(), lk.DrcengValue, waterDorm, cfg)
	if err != nil {
		logger.Error("admin power query failed", "dorm", req.DormRoom, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"msg":         "查询电量失败，请稍后重试",
			"dorm_raw":    req.DormRoom,
			"physical_id": lk.DrcengValue,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"dorm_room":     result.DormRoom,
			"label":         lk.Opt.Label,
			"physical_id":   lk.DrcengValue,
			"remaining_kwh": result.RemainingKwh,
			"remaining_f":   result.RemainingF,
			"water_amount":  result.WaterAmount,
			"water_f":       result.WaterF,
		},
	})
}
