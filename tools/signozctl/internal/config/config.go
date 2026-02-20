package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Profile struct {
	Host         string `json:"host"`
	Email        string `json:"email,omitempty"`
	OrgID        string `json:"orgId,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

type File struct {
	ActiveProfile string             `json:"activeProfile"`
	Profiles      map[string]Profile `json:"profiles"`
}

func DefaultPath() (string, error) {
	if p := os.Getenv("SIGNOZCTL_CONFIG"); p != "" {
		return p, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "signozctl", "config.json"), nil
}

func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{Profiles: map[string]Profile{}}, nil
		}
		return nil, err
	}

	var out File
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if out.Profiles == nil {
		out.Profiles = map[string]Profile{}
	}
	return &out, nil
}

func Save(path string, cfg *File) error {
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
