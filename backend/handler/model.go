package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"

	"platform/model"
	"platform/util"
)

type ModelHandler struct{}

func NewModelHandler() *ModelHandler {
	return &ModelHandler{}
}

type ModelRequest struct {
	Name      string `json:"name" binding:"-"`
	Provider  string `json:"provider" binding:"-"`
	APIKey    string `json:"api_key" binding:"-"`
	BaseURL   string `json:"base_url" binding:"-"`
	Model     string `json:"model" binding:"-"`
	MaxTokens int    `json:"max_tokens" binding:"-"`
	IsActive  bool   `json:"is_active" binding:"-"`
}

func (h *ModelHandler) List(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	var models []model.ModelConfig
	if err := util.DB.Where("user_id = ?", userID).Find(&models).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "获取模型列表失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(models))
}

func (h *ModelHandler) Create(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	var req ModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}

	modelConfig := model.ModelConfig{
		UserID:    userID,
		Name:      req.Name,
		Provider:  req.Provider,
		APIKey:    req.APIKey,
		BaseURL:   req.BaseURL,
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		IsActive:  req.IsActive,
	}

	// 设置状态
	modelConfig.SetStatus()

	if err := util.DB.Create(&modelConfig).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "创建模型配置失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(modelConfig))
}

func (h *ModelHandler) Update(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var modelConfig model.ModelConfig
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&modelConfig).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "模型配置不存在"))
		return
	}

	var req ModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}

	// 更新字段
	if req.Name != "" {
		modelConfig.Name = req.Name
	}
	if req.Provider != "" {
		modelConfig.Provider = req.Provider
	}
	if req.APIKey != "" {
		modelConfig.APIKey = req.APIKey
	}
	if req.BaseURL != "" {
		modelConfig.BaseURL = req.BaseURL
	}
	if req.Model != "" {
		modelConfig.Model = req.Model
	}
	if req.MaxTokens > 0 {
		modelConfig.MaxTokens = req.MaxTokens
	}

	// 处理IsActive字段（切换开关时）
	if !req.IsActive {
		modelConfig.IsActive = false
	} else if req.IsActive {
		modelConfig.IsActive = true
	}

	// 同步Status状态
	modelConfig.SetStatus()

	if err := util.DB.Save(&modelConfig).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "更新模型配置失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(modelConfig))
}

func (h *ModelHandler) Delete(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var modelConfig model.ModelConfig
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&modelConfig).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "模型配置不存在"))
		return
	}

	if err := util.DB.Delete(&modelConfig).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "删除模型配置失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(nil))
}

// TestModel 测试模型连接
func (h *ModelHandler) TestModel(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var modelConfig model.ModelConfig
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&modelConfig).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "模型配置不存在"))
		return
	}

	// 创建OpenAI客户端进行测试
	config := openai.DefaultConfig(modelConfig.APIKey)
	if modelConfig.BaseURL != "" {
		config.BaseURL = modelConfig.BaseURL
	}
	client := openai.NewClientWithConfig(config)

	// 发送测试请求
	ctx := context.Background()
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: modelConfig.Model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Hello, this is a test message.",
				},
			},
			MaxTokens: 50,
		},
	)

	if err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, "模型连接测试失败: "+err.Error()))
		return
	}

	// 检查响应是否有效
	if len(resp.Choices) == 0 {
		c.JSON(http.StatusBadRequest, util.Error(400, "模型返回为空"))
		return
	}

	c.JSON(http.StatusOK, util.Success(gin.H{
		"success":  true,
		"message":  "模型连接成功",
		"response": resp.Choices[0].Message.Content,
	}))
}
