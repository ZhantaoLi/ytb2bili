package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/core"
	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"go.uber.org/zap"
)

func TestConfigDebugRouteIsNotRegistered(t *testing.T) {
	app := core.NewServer(types.NewDefaultConfig(), zap.NewNop().Sugar())
	app.Init(nil)
	NewConfigHandler(app).RegisterRoutes(app)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/debug", nil)
	rec := httptest.NewRecorder()

	app.Engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
