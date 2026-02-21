package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadPlainConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := &File{
		ActiveProfile: "local",
		Profiles: map[string]Profile{
			"local": {
				Host:        "http://localhost:8080",
				AccessToken: "access-1",
			},
		},
	}
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatalf("save plain config failed: %v", err)
	}
	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load plain config failed: %v", err)
	}
	if loaded.Profiles["local"].AccessToken != "access-1" {
		t.Fatalf("unexpected loaded token: %#v", loaded.Profiles["local"])
	}
}

func TestSaveLoadEncryptedConfigWithPassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	t.Setenv("SIGNOZCTL_CONFIG_PASSPHRASE", "top-secret-passphrase")

	cfg := &File{
		ActiveProfile: "local",
		Profiles: map[string]Profile{
			"local": {
				Host:         "http://localhost:8080",
				AccessToken:  "access-1",
				RefreshToken: "refresh-1",
			},
		},
	}
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatalf("save encrypted config failed: %v", err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read encrypted config failed: %v", err)
	}
	if string(raw) == "" || string(raw) == "{}" {
		t.Fatalf("unexpected empty encrypted config")
	}
	if strings.Contains(string(raw), "access-1") {
		t.Fatalf("expected encrypted file contents, got %q", string(raw))
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load encrypted config failed: %v", err)
	}
	if loaded.Profiles["local"].AccessToken != "access-1" || loaded.Profiles["local"].RefreshToken != "refresh-1" {
		t.Fatalf("unexpected loaded profile: %#v", loaded.Profiles["local"])
	}
}

func TestLoadEncryptedConfigFailsWithoutPassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	t.Setenv("SIGNOZCTL_CONFIG_PASSPHRASE", "top-secret-passphrase")

	cfg := &File{
		ActiveProfile: "local",
		Profiles: map[string]Profile{
			"local": {Host: "http://localhost:8080", AccessToken: "access-1"},
		},
	}
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatalf("save encrypted config failed: %v", err)
	}

	t.Setenv("SIGNOZCTL_CONFIG_PASSPHRASE", "")
	if _, err := Load(cfgPath); err == nil {
		t.Fatalf("expected encrypted load to fail without passphrase")
	}
}
