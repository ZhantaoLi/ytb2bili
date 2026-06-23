package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ZhantaoLi/ytb2bili/internal/core/types"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestCORSMiddlewareRejectsUntrustedCredentialOrigin(t *testing.T) {
	server := NewServer(types.NewDefaultConfig(), zap.NewNop().Sugar())
	server.Init(nil)
	server.Engine.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	server.Engine.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty for untrusted origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want empty for untrusted origin", got)
	}
}

