package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"github.com/glebarez/sqlite"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// NewDatabase 创建数据库连接
func NewDatabase(config *types.AppConfig) (*gorm.DB, error) {
	// GORM配置
	gormConfig := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "tb_", // crypto_wallet prefix
			SingularTable: false,
		},
	}

	// debug 模式保留数据库警告/错误，避免调度器轮询 SQL 淹没业务日志。
	gormConfig.Logger = logger.Default.LogMode(gormLogLevel(config.Debug))

	// 根据数据库类型创建连接
	var db *gorm.DB
	var err error

	switch config.Database.Type {
	case "postgres", "postgresql":
		dsn := config.Database.GetDSN()
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
		}
	case "mysql":

		dsn := config.Database.GetDSN()
		db, err = gorm.Open(mysql.Open(dsn), gormConfig)

		if err != nil {
			return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
		}
	case "sqlite", "sqlite3":
		dsn := config.Database.GetDSN()
		if err := ensureSQLiteParentDir(dsn); err != nil {
			return nil, fmt.Errorf("failed to prepare SQLite path: %w", err)
		}
		db, err = gorm.Open(sqlite.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to SQLite: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported database type: %s (supported: postgres, mysql, sqlite)", config.Database.Type)
	}

	// 获取底层的sql.DB对象
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func gormLogLevel(debug bool) logger.LogLevel {
	if debug {
		return logger.Warn
	}
	return logger.Silent
}

func ensureSQLiteParentDir(dsn string) error {
	if dsn == "" || dsn == ":memory:" || strings.Contains(dsn, "mode=memory") || strings.Contains(dsn, "::memory:") {
		return nil
	}

	path := dsn
	if strings.HasPrefix(path, "file:") {
		path = strings.TrimPrefix(path, "file:")
		if idx := strings.IndexAny(path, "?#"); idx >= 0 {
			path = path[:idx]
		}
	}

	if path == "" || path == "." || path == ":memory:" {
		return nil
	}

	dir := filepath.Dir(filepath.FromSlash(path))
	if dir == "" || dir == "." {
		return nil
	}

	return os.MkdirAll(dir, 0o755)
}

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	// 导入所有模型并执行迁移
	return db.AutoMigrate(
		&model.User{},
		&model.SavedVideo{},
	)
}
