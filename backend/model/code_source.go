package model

import (
	"time"

	"gorm.io/gorm"
)

type CodeSourceType string

const (
	CodeSourceTypeZip CodeSourceType = "zip"
	CodeSourceTypeJar CodeSourceType = "jar"
	CodeSourceTypeGit CodeSourceType = "git"
)

type CodeSource struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"not null;index" json:"userId"`
	Type      CodeSourceType `gorm:"type:varchar(10);not null" json:"type"`
	Name      string         `gorm:"not null" json:"name"`
	Size      int64          `json:"size"`
	URL       string         `json:"url"`
	FilePath  string         `json:"filePath"`
	Path      string         `json:"path"`
	Status    string         `gorm:"type:varchar(20);default:'uploaded';index" json:"status"`
	Language  string         `gorm:"type:varchar(50)" json:"language"` // 检测到的编程语言
}

func (CodeSource) TableName() string {
	return "code_sources"
}
