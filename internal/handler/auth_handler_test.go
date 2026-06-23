package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/models"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminMiddlewareProtectsManagementRoutesRegisteredAfterAuth(t *testing.T) {
	config := types.NewDefaultConfig()
	config.Auth.JWTSecret = "test-secret"
	app := core.NewServer(config, zap.NewNop().Sugar())
	app.Init(nil)

	authHandler := NewAuthHandler(app)
	authHandler.RegisterRoutes(app)

	app.Engine.GET("/api/v1/videos", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/videos", nil)
	rec := httptest.NewRecorder()

	app.Engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminMiddlewareProtectsSubmitRouteRegisteredAfterAuth(t *testing.T) {
	config := types.NewDefaultConfig()
	config.Auth.JWTSecret = "test-secret"
	app := core.NewServer(config, zap.NewNop().Sugar())
	app.Init(nil)

	authHandler := NewAuthHandler(app)
	authHandler.RegisterRoutes(app)

	app.Engine.POST("/api/v1/submit", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/submit", nil)
	rec := httptest.NewRecorder()

	app.Engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRegisterRoutesLegacyIsDisabled(t *testing.T) {
	config := types.NewDefaultConfig()
	config.Auth.JWTSecret = "test-secret"
	app := core.NewServer(config, zap.NewNop().Sugar())
	app.Init(nil)

	authHandler := NewAuthHandler(app)

	defer func() {
		if recover() == nil {
			t.Fatal("registerRoutesLegacy() should be disabled")
		}
	}()

	authHandler.registerRoutesLegacy(app)
}

func TestEnsureAdminUserDoesNotCreateDefaultCredentialsWithoutConfiguration(t *testing.T) {
	t.Setenv("YTB2BILI_ADMIN_USERNAME", "")
	t.Setenv("YTB2BILI_ADMIN_PASSWORD", "")

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TBUser{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	handler := &AuthHandler{
		BaseHandler: BaseHandler{App: &core.AppServer{DB: db}},
	}

	if _, err := handler.ensureAdminUser(); err == nil {
		t.Fatal("ensureAdminUser() created a default admin without explicit credentials")
	}

	var count int64
	if err := db.Model(&models.TBUser{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("admin user count = %d, want 0", count)
	}
}

func TestAdminLoginRejectsLegacyDefaultCredentials(t *testing.T) {
	t.Setenv("YTB2BILI_ADMIN_USERNAME", "")
	t.Setenv("YTB2BILI_ADMIN_PASSWORD", "")

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TBUser{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	legacyAdmin := models.TBUser{
		Id:       "legacy-admin",
		Username: "admin",
		Email:    "admin@ytb2bili.local",
		Status:   "active",
	}
	if err := legacyAdmin.HashPassword("admin123"); err != nil {
		t.Fatalf("hash legacy password: %v", err)
	}
	if err := db.Create(&legacyAdmin).Error; err != nil {
		t.Fatalf("create legacy admin: %v", err)
	}

	app := core.NewServer(types.NewDefaultConfig(), zap.NewNop().Sugar())
	app.DB = db
	app.Engine.POST("/admin/login", NewAuthHandler(app).adminLogin)

	req := httptest.NewRequest(
		http.MethodPost,
		"/admin/login",
		strings.NewReader(`{"username":"admin","password":"admin123"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.Engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestEnsureAdminUserCreatesConfiguredAdmin(t *testing.T) {
	t.Setenv("YTB2BILI_ADMIN_USERNAME", "owner")
	t.Setenv("YTB2BILI_ADMIN_PASSWORD", "change-me-strong-password")
	t.Setenv("YTB2BILI_ADMIN_EMAIL", "owner@example.test")

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TBUser{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	handler := &AuthHandler{
		BaseHandler: BaseHandler{App: &core.AppServer{DB: db}},
	}

	user, err := handler.ensureAdminUser()
	if err != nil {
		t.Fatalf("ensureAdminUser() error = %v", err)
	}
	if user.Username != "owner" {
		t.Fatalf("username = %q, want owner", user.Username)
	}
	if user.Email != "owner@example.test" {
		t.Fatalf("email = %q, want owner@example.test", user.Email)
	}
	if err := user.CheckPassword("change-me-strong-password"); err != nil {
		t.Fatalf("configured password does not validate: %v", err)
	}
}

func TestEnsureAdminUserLegacyIsDisabled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TBUser{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	handler := &AuthHandler{
		BaseHandler: BaseHandler{App: &core.AppServer{DB: db}},
	}

	if _, err := handler.ensureAdminUserLegacy(); err == nil {
		t.Fatal("ensureAdminUserLegacy() should be disabled")
	}
}
