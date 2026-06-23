package types

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestNewDefaultConfigLeavesJWTSecretUnconfigured(t *testing.T) {
	config := NewDefaultConfig()

	if config.Auth.JWTSecret != "" {
		t.Fatalf("default JWT secret = %q, want empty so JWTManager can generate a process-local secret", config.Auth.JWTSecret)
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

func TestLoadConfigPreservesDefaultsWhenFieldsAreOmitted(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	configText := `
[DeepLXConfig]
enabled = true
endpoint = "https://example.test/translate"
`
	if err := os.WriteFile(configPath, []byte(configText), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() returned error: %v", err)
	}

	if config.Listen != ":8096" {
		t.Fatalf("Listen = %q, want default :8096", config.Listen)
	}
	if config.Environment != "development" {
		t.Fatalf("Environment = %q, want development", config.Environment)
	}
	if config.DataPath != "./data" {
		t.Fatalf("DataPath = %q, want ./data", config.DataPath)
	}
	if config.Database.Type != "postgres" {
		t.Fatalf("Database.Type = %q, want postgres", config.Database.Type)
	}
}

func TestSaveConfigWritesOwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file permissions do not map cleanly to Unix mode bits")
	}

	config := NewDefaultConfig()
	config.Path = filepath.Join(t.TempDir(), "config.toml")

	if err := SaveConfig(config); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	info, err := os.Stat(config.Path)
	if err != nil {
		t.Fatalf("stat saved config: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0600); got != want {
		t.Fatalf("saved config permissions = %o, want %o", got, want)
	}
}
