package store

import (
	"path/filepath"
	"testing"

	"github.com/difyz9/ytb2bili/internal/core/types"
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
