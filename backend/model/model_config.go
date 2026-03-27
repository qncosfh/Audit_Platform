package model

import (
	"time"

	"gorm.io/gorm"
)

type ModelStatus string

const (
	ModelStatusActive   ModelStatus = "active"
	ModelStatusInactive ModelStatus = "inactive"
	ModelStatusError    ModelStatus = "error"
)

type ModelConfig struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"not null" json:"userId"`
	Name      string         `gorm:"not null" json:"name"`
	Provider  string         `gorm:"not null" json:"provider"`
	APIKey    string         `json:"apiKey"`
	BaseURL   string         `json:"baseUrl" gorm:"column:base_url"`
	Model     string         `json:"model"`
	MaxTokens int            `json:"maxTokens"`
	IsActive  bool           `gorm:"default:true" json:"isActive"`
	Status    ModelStatus    `gorm:"type:varchar(20);default:'active'" json:"status"`
}

// SetStatus 根据IsActive设置Status
func (m *ModelConfig) SetStatus() {
	if m.IsActive {
		m.Status = ModelStatusActive
	} else {
		m.Status = ModelStatusInactive
	}
}

func (ModelConfig) TableName() string {
	return "model_configs"
}
