package store

import (
	"github.com/ZhantaoLi/ytb2bili/internal/core/models"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"gorm.io/gorm"
)

// MigrateDatabase 自动迁移数据库表
func MigrateDatabase(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.SavedVideo{},
		&model.TaskStep{},
		&model.AccountBinding{},
		&models.TBUser{}, // 管理员用户表
	)
}
