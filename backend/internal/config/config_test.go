package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
server:
  listen: ":9999"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Listen != ":9999" {
		t.Errorf("listen = %q, want :9999", cfg.Server.Listen)
	}
	if cfg.RateLimit.User.RequestsPerMinute != 100 {
		t.Errorf("default rpm = %d, want 100", cfg.RateLimit.User.RequestsPerMinute)
	}
	if cfg.RateLimit.User.Burst != 20 {
		t.Errorf("default burst = %d, want 20", cfg.RateLimit.User.Burst)
	}
	if cfg.RateLimit.Global.ConcurrentStreams != 500 {
		t.Errorf("default concurrent = %d, want 500", cfg.RateLimit.Global.ConcurrentStreams)
	}
	if cfg.Routing.HealthCheckInterval == 0 {
		t.Error("default health_check_interval should be set")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
security:
  jwt_secret_env: TEST_JWT_SECRET
`), 0o644); err != nil {
		t.Fatal(err)
	}
	// env not set → should fail (JWT secret is required in 1.0)
	if _, err := Load(path); err == nil {
		t.Error("expected error when JWT secret env not set")
	}

	// set env → should succeed
	t.Setenv("TEST_JWT_SECRET", "abcdef0123456789")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Security.JWTSecretEnv != "TEST_JWT_SECRET" {
		t.Errorf("jwt_secret_env = %q", cfg.Security.JWTSecretEnv)
	}
}

func TestLoad_MasterKey_Optional(t *testing.T) {
	// 1.0: master_key_env 可不配，配了才校验
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
server:
  listen: ":9000"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Security.MasterKeyEnv != "" {
		t.Errorf("master_key_env should be empty, got %q", cfg.Security.MasterKeyEnv)
	}
}

func TestMasterKey(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`
security:
  master_key_env: AI_ALL_IN_ONE_MASTER_KEY
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AI_ALL_IN_ONE_MASTER_KEY", "test-key-bytes")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	key, err := cfg.MasterKey()
	if err != nil {
		t.Fatal(err)
	}
	if string(key) != "test-key-bytes" {
		t.Errorf("key = %q", string(key))
	}
}
