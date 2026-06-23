package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	gormlogger "gorm.io/gorm/logger"
)

func TestNewDatabaseSupportsSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ytb2bili.db")
	config := types.NewDefaultConfig()
	config.Debug = false
	config.Database = types.Database{
		Type: "sqlite",
		Host: dbPath,
	}

	db, err := NewDatabase(config)
	if err != nil {
		t.Fatalf("NewDatabase() with sqlite returned error: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() returned error: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("SQLite database ping failed: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("failed to close SQLite database: %v", err)
	}
}

func TestNewDatabaseCreatesSQLiteParentDirectory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "data", "ytb2bili.db")
	config := types.NewDefaultConfig()
	config.Debug = false
	config.Database = types.Database{
		Type:     "sqlite",
		Database: dbPath,
	}

	db, err := NewDatabase(config)
	if err != nil {
		t.Fatalf("NewDatabase() with nested sqlite path returned error: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() returned error: %v", err)
	}
	defer sqlDB.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected sqlite file to exist at %q: %v", dbPath, err)
	}
}

func TestGormLogLevelAvoidsSQLSpamInDebugMode(t *testing.T) {
	if got := gormLogLevel(true); got != gormlogger.Warn {
		t.Fatalf("gormLogLevel(true) = %v, want %v", got, gormlogger.Warn)
	}
	if got := gormLogLevel(false); got != gormlogger.Silent {
		t.Fatalf("gormLogLevel(false) = %v, want %v", got, gormlogger.Silent)
	}
}
