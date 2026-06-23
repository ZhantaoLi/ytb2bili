package services

import (
	"testing"

	"go.uber.org/zap"
)

func TestBilibiliAccountServiceUsesConfiguredEncryptionKey(t *testing.T) {
	key := "12345678901234567890123456789012"
	t.Setenv("YTB2BILI_ACCOUNT_ENCRYPTION_KEY", key)

	service := NewBilibiliAccountService(nil, zap.NewNop().Sugar())
	encrypted, err := service.encrypt("cookie-secret")
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}

	configuredKeyService := &BilibiliAccountService{encryptionKey: []byte(key)}
	decrypted, err := configuredKeyService.decrypt(encrypted)
	if err != nil {
		t.Fatalf("configured key failed to decrypt ciphertext: %v", err)
	}
	if decrypted != "cookie-secret" {
		t.Fatalf("decrypted = %q, want cookie-secret", decrypted)
	}
}

func TestBilibiliAccountServiceDoesNotUseLegacyHardcodedKeyByDefault(t *testing.T) {
	t.Setenv("YTB2BILI_ACCOUNT_ENCRYPTION_KEY", "")

	service := NewBilibiliAccountService(nil, zap.NewNop().Sugar())
	encrypted, err := service.encrypt("cookie-secret")
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}

	legacyKeyService := &BilibiliAccountService{encryptionKey: []byte("a463b25e5f694b8f85bd805f272723e8")}
	if decrypted, err := legacyKeyService.decrypt(encrypted); err == nil && decrypted == "cookie-secret" {
		t.Fatal("ciphertext encrypted without configuration was decrypted by the legacy hardcoded key")
	}
}
