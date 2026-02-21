package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

type encryptedFile struct {
	Format     string `json:"format"`
	KDF        string `json:"kdf"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
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

	dec, err := maybeDecryptConfig(b)
	if err != nil {
		return nil, err
	}
	if dec != nil {
		b = dec
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
	enc, err := maybeEncryptConfig(b)
	if err != nil {
		return err
	}
	if enc != nil {
		b = enc
	}
	return os.WriteFile(path, b, 0o600)
}

func maybeEncryptConfig(plain []byte) ([]byte, error) {
	passphrase := os.Getenv("SIGNOZCTL_CONFIG_PASSPHRASE")
	if passphrase == "" {
		return nil, nil
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, plain, nil)
	wrapped := encryptedFile{
		Format:     "signozctl.v1.encrypted",
		KDF:        "sha256(passphrase+salt)",
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
	return json.MarshalIndent(wrapped, "", "  ")
}

func maybeDecryptConfig(raw []byte) ([]byte, error) {
	var wrapped encryptedFile
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, nil
	}
	if wrapped.Format != "signozctl.v1.encrypted" {
		return nil, nil
	}
	passphrase := os.Getenv("SIGNOZCTL_CONFIG_PASSPHRASE")
	if passphrase == "" {
		return nil, fmt.Errorf("config is encrypted; set SIGNOZCTL_CONFIG_PASSPHRASE")
	}
	salt, err := base64.StdEncoding.DecodeString(wrapped.Salt)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(wrapped.Nonce)
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(wrapped.Ciphertext)
	if err != nil {
		return nil, err
	}
	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}
	return plain, nil
}

func deriveKey(passphrase string, salt []byte) []byte {
	h := sha256.Sum256(append([]byte(passphrase), salt...))
	return h[:]
}
