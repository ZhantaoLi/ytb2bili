package auth

import "testing"

func TestJWTManagerDoesNotUseSharedDefaultSecret(t *testing.T) {
	first := NewJWTManager(&JWTConfig{})
	second := NewJWTManager(&JWTConfig{})

	token, err := first.GenerateToken("user-1", "user@example.test", "admin")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if _, err := second.ValidateToken(token); err == nil {
		t.Fatal("token signed by one empty-secret manager validated with another empty-secret manager")
	}
}

