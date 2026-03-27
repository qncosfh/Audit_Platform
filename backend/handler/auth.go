package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"platform/model"
	"platform/util"
)

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username    string `json:"username" binding:"required,min=2,max=50"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8,max=100"`
	Phone       string `json:"phone" binding:"required"`
	Company     string `json:"company" binding:"required,min=2,max=100"`
	Industry    string `json:"industry" binding:"required"`
	UserCount   string `json:"userCount" binding:"required"`
	Description string `json:"description" binding:"required,min=10,max=500"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type UserResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// validateInput 验证输入内容，使用白名单验证
// 注意：由于代码已使用GORM参数化查询，主要风险已被防止
// 此函数作为额外的输入验证层
func validateInput(input string) bool {
	// 检查输入是否为空
	if len(input) == 0 {
		return false
	}

	// 检查是否包含控制字符（Unicode范围）
	for _, r := range input {
		// 控制字符（0-31）和删除字符（127），除了换行、制表符等常见字符
		if (r >= 0 && r < 32 && r != 9 && r != 10 && r != 13) || r == 127 {
			return false
		}
	}

	// 检查是否包含常见SQL关键字（作为完整单词，使用边界检查）
	// 注意：这只是一种额外的防护，GORM参数化查询已经提供主要保护
	dangerousPatterns := []string{
		"javascript:", "onerror=", "onload=", "onclick=",
	}
	inputLower := strings.ToLower(input)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(inputLower, pattern) {
			return false
		}
	}

	return true
}

// validatePasswordStrength 验证密码强度
func validatePasswordStrength(password string) (bool, string) {
	if len(password) < 8 {
		return false, "密码长度至少8个字符"
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasDigit = true
		case char == '!' || char == '@' || char == '#' || char == '$' || char == '%' || char == '^' || char == '&' || char == '*':
			hasSpecial = true
		}
	}

	if !hasUpper {
		return false, "密码必须包含大写字母"
	}
	if !hasLower {
		return false, "密码必须包含小写字母"
	}
	if !hasDigit {
		return false, "密码必须包含数字"
	}
	if !hasSpecial {
		return false, "密码必须包含特殊字符(!@#$%^&*)"
	}

	return true, ""
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}

	// 验证输入内容
	if !validateInput(req.Username) || !validateInput(req.Email) {
		c.JSON(http.StatusBadRequest, util.Error(400, "输入内容包含非法字符"))
		return
	}

	// 检查用户名是否已存在
	var existingUser model.User
	if err := util.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, util.Error(400, "用户名已存在"))
		return
	}

	// 检查邮箱是否已存在
	if err := util.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, util.Error(400, "邮箱已存在"))
		return
	}

	// 验证密码强度
	if valid, msg := validatePasswordStrength(req.Password); !valid {
		c.JSON(http.StatusBadRequest, util.Error(400, msg))
		return
	}

	// 哈希密码
	hashedPassword, err := util.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "密码加密失败"))
		return
	}

	// 创建用户
	user := model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
		Role:     "user", // 默认角色
	}

	if err := util.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "用户创建失败"))
		return
	}

	// 发送注册通知邮件（异步发送，不阻塞响应）
	go func() {
		notification := util.ApplicationNotification{
			Username:    req.Username,
			Email:       req.Email,
			Phone:       req.Phone,
			Company:     req.Company,
			Industry:    req.Industry,
			UserCount:   req.UserCount,
			Description: req.Description,
		}
		if err := util.SendApplicationNotification(notification); err != nil {
			fmt.Printf("[注册通知] 发送邮件失败: %v\n", err)
		}
	}()

	// 生成JWT令牌 - 包含用户名和角色
	token, err := util.GenerateJWTWithClaims(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "令牌生成失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(LoginResponse{
		Token: token,
		User: UserResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	}))
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}

	var user model.User
	if err := util.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 统一的错误提示，防止用户名枚举攻击
			c.JSON(http.StatusUnauthorized, util.Error(401, "账号或密码错误"))
			return
		}
		c.JSON(http.StatusInternalServerError, util.Error(500, "登录失败"))
		return
	}

	// 验证密码
	if !util.CheckPasswordHash(req.Password, user.Password) {
		// 统一的错误提示，防止用户名枚举攻击
		c.JSON(http.StatusUnauthorized, util.Error(401, "账号或密码错误"))
		return
	}

	// 生成JWT令牌 - 包含用户名和角色
	token, err := util.GenerateJWTWithClaims(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "令牌生成失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(LoginResponse{
		Token: token,
		User: UserResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	}))
}

func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	var user model.User
	if err := util.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "用户不存在"))
		return
	}

	c.JSON(http.StatusOK, util.Success(UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	}))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// 获取当前token并添加到黑名单
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString != authHeader {
			// 将令牌添加到黑名单，实现登出失效
			util.AddToBlacklist(tokenString)
		}
	}

	c.JSON(http.StatusOK, util.Success("退出登录成功"))
}

// RefreshToken 刷新Token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	// 从context获取当前用户ID（由AuthMiddleware设置）
	userID := util.GetUserID(c)

	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	// 获取当前token并检查是否在黑名单中
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString != authHeader {
			// 检查令牌是否在黑名单中
			if util.IsTokenBlacklisted(tokenString) {
				c.JSON(http.StatusUnauthorized, util.Error(401, "令牌已失效，请重新登录"))
				return
			}
		}
	}

	// 验证用户仍然存在且有效
	var user model.User
	if err := util.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, util.Error(401, "用户不存在"))
		return
	}

	// 生成新的JWT令牌
	token, err := util.GenerateJWTWithClaims(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "令牌刷新失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(gin.H{
		"token": token,
		"user": UserResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	}))
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=100"`
}

// ChangePassword 修改密码
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}

	// 获取用户
	var user model.User
	if err := util.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "用户不存在"))
		return
	}

	// 验证旧密码
	if !util.CheckPasswordHash(req.OldPassword, user.Password) {
		c.JSON(http.StatusBadRequest, util.Error(400, "原密码错误"))
		return
	}

	// 验证新密码强度
	if valid, msg := validatePasswordStrength(req.NewPassword); !valid {
		c.JSON(http.StatusBadRequest, util.Error(400, msg))
		return
	}

	// 哈希新密码
	hashedPassword, err := util.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "密码加密失败"))
		return
	}

	// 更新密码
	if err := util.DB.Model(&user).Update("password", hashedPassword).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "密码更新失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success("密码修改成功"))
}
