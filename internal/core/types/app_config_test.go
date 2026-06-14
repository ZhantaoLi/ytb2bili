package types

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDefaultConfigLeavesAPIAuthUnconfigured(t *testing.T) {
	config := NewDefaultConfig()

	if config.APIAuth.AppID != "" {
		t.Fatalf("default API auth AppID = %q, want empty", config.APIAuth.AppID)
	}
	if config.APIAuth.AppSecret != "" {
		t.Fatalf("default API auth AppSecret = %q, want empty", config.APIAuth.AppSecret)
	}
	if config.APIAuth.CookiesDecryptKey != "" {
		t.Fatalf("default cookies decrypt key = %q, want empty", config.APIAuth.CookiesDecryptKey)
	}
}

func TestLoadConfigUsesSelfConfiguredAPIAuth(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	configText := `
listen = ":8096"
environment = "development"
debug = false

[api_auth]
app_id = "local-app"
app_secret = "local-secret"
cookies_decrypt_key = "local-cookie-key"
`
	if err := os.WriteFile(configPath, []byte(configText), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() returned error: %v", err)
	}

	if config.APIAuth.AppID != "local-app" {
		t.Fatalf("API auth AppID = %q, want local-app", config.APIAuth.AppID)
	}
	if config.APIAuth.AppSecret != "local-secret" {
		t.Fatalf("API auth AppSecret = %q, want local-secret", config.APIAuth.AppSecret)
	}
	if config.APIAuth.CookiesDecryptKey != "local-cookie-key" {
		t.Fatalf("cookies decrypt key = %q, want local-cookie-key", config.APIAuth.CookiesDecryptKey)
	}
}

func TestLoadConfigUsesSelfConfiguredDeepLX(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	configText := `
listen = ":8096"
environment = "development"
debug = false

[DeepLXConfig]
enabled = true
endpoint = "https://example.test/translate"
source_lang = "EN"
target_lang = "ZH"
timeout = 15
`
	if err := os.WriteFile(configPath, []byte(configText), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() returned error: %v", err)
	}

	if config.DeepLXConfig == nil {
		t.Fatal("DeepLXConfig is nil")
	}
	if !config.DeepLXConfig.Enabled {
		t.Fatal("DeepLXConfig.Enabled = false, want true")
	}
	if config.DeepLXConfig.Endpoint != "https://example.test/translate" {
		t.Fatalf("DeepLXConfig.Endpoint = %q", config.DeepLXConfig.Endpoint)
	}
	if config.DeepLXConfig.SourceLang != "EN" {
		t.Fatalf("DeepLXConfig.SourceLang = %q, want EN", config.DeepLXConfig.SourceLang)
	}
	if config.DeepLXConfig.TargetLang != "ZH" {
		t.Fatalf("DeepLXConfig.TargetLang = %q, want ZH", config.DeepLXConfig.TargetLang)
	}
	if config.DeepLXConfig.Timeout != 15 {
		t.Fatalf("DeepLXConfig.Timeout = %d, want 15", config.DeepLXConfig.Timeout)
	}
}

func TestLoadConfigUsesSelfConfiguredMimoASR(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	configText := `
listen = ":8096"
environment = "development"
debug = false

[MimoASRConfig]
enabled = true
api_key = "test-key"
base_url = "https://example.test/v1"
model = "mimo-v2.5-asr"
language = "ko"
segment_seconds = 120
timeout = 180
`
	if err := os.WriteFile(configPath, []byte(configText), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() returned error: %v", err)
	}

	if config.MimoASRConfig == nil {
		t.Fatal("MimoASRConfig is nil")
	}
	if !config.MimoASRConfig.Enabled {
		t.Fatal("MimoASRConfig.Enabled = false, want true")
	}
	if config.MimoASRConfig.APIKey != "test-key" {
		t.Fatalf("MimoASRConfig.APIKey = %q, want test-key", config.MimoASRConfig.APIKey)
	}
	if config.MimoASRConfig.BaseURL != "https://example.test/v1" {
		t.Fatalf("MimoASRConfig.BaseURL = %q", config.MimoASRConfig.BaseURL)
	}
	if config.MimoASRConfig.Model != "mimo-v2.5-asr" {
		t.Fatalf("MimoASRConfig.Model = %q", config.MimoASRConfig.Model)
	}
	if config.MimoASRConfig.Language != "ko" {
		t.Fatalf("MimoASRConfig.Language = %q, want ko", config.MimoASRConfig.Language)
	}
	if config.MimoASRConfig.SegmentSeconds != 120 {
		t.Fatalf("MimoASRConfig.SegmentSeconds = %d, want 120", config.MimoASRConfig.SegmentSeconds)
	}
	if config.MimoASRConfig.Timeout != 180 {
		t.Fatalf("MimoASRConfig.Timeout = %d, want 180", config.MimoASRConfig.Timeout)
	}
}
