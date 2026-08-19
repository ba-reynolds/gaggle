package config

import "testing"

func TestLoadConfigProductionRejectsDevDefaults(t *testing.T) {
	devJWT := []string{"", "secret", "dev-secret-change-me", "change-me"}
	devDB := []string{"", "teeth", "password"}

	for _, val := range devJWT {
		t.Setenv("APP_ENV", "production")
		t.Setenv("DB_PASSWORD", "a-real-prod-db-password")
		t.Setenv("JWT_SECRET", val)
		if _, err := LoadConfig(); err == nil {
			t.Errorf("JWT_SECRET=%q in production should error, got nil", val)
		}
	}

	for _, val := range devDB {
		t.Setenv("APP_ENV", "production")
		t.Setenv("JWT_SECRET", "a-real-prod-jwt-secret")
		t.Setenv("DB_PASSWORD", val)
		if _, err := LoadConfig(); err == nil {
			t.Errorf("DB_PASSWORD=%q in production should error, got nil", val)
		}
	}
}

func TestLoadConfigProductionAcceptsRealSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "a-real-prod-jwt-secret")
	t.Setenv("DB_PASSWORD", "a-real-prod-db-password")
	if _, err := LoadConfig(); err != nil {
		t.Errorf("real prod secrets should load cleanly, got: %v", err)
	}
}

func TestLoadConfigDevelopmentAllowsDevDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SECRET", "dev-secret-change-me")
	t.Setenv("DB_PASSWORD", "teeth")
	if _, err := LoadConfig(); err != nil {
		t.Errorf("APP_ENV=development with dev defaults should load, got: %v", err)
	}
}
