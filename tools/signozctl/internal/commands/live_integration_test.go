package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runLiveCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := NewRootCommand()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func requireLiveEnv(t *testing.T) (string, string) {
	t.Helper()
	if os.Getenv("SIGNOZCTL_E2E") != "1" {
		t.Skip("set SIGNOZCTL_E2E=1 to run live integration tests")
	}
	profile := os.Getenv("SIGNOZCTL_E2E_PROFILE")
	if profile == "" {
		profile = "local"
	}

	cfgPath := os.Getenv("SIGNOZCTL_E2E_CONFIG")
	if cfgPath == "" {
		var err error
		cfgPath, err = configPathFromDefault()
		if err != nil {
			t.Fatalf("failed to resolve default config path: %v", err)
		}
	}
	return cfgPath, profile
}

func configPathFromDefault() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "signozctl", "config.json"), nil
}

func mustJSONMap(t *testing.T, s string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("expected json object, got %q: %v", s, err)
	}
	return out
}

func TestLiveAuthStatus(t *testing.T) {
	cfgPath, profile := requireLiveEnv(t)
	stdout, stderr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"auth", "status",
	)
	if err != nil {
		t.Fatalf("auth status failed: %v stderr=%s", err, stderr)
	}
	resp := mustJSONMap(t, stdout)
	if resp["authenticated"] != true {
		t.Fatalf("expected authenticated profile, got: %v", resp)
	}
}

func TestLiveQueryTraces(t *testing.T) {
	cfgPath, profile := requireLiveEnv(t)
	tmpDir := t.TempDir()
	payloadPath := filepath.Join(tmpDir, "query-traces.json")
	now := time.Now().UnixMilli()
	start := now - int64((30 * time.Minute).Milliseconds())

	payload := fmt.Sprintf(`{
  "schemaVersion": "v1",
  "start": %d,
  "end": %d,
  "requestType": "trace",
  "compositeQuery": {
    "queries": [
      {
        "type": "builder_query",
        "spec": {
          "name": "A",
          "signal": "traces",
          "stepInterval": 60,
          "aggregations": [{"expression": "count()"}],
          "order": [{"key": {"name": "timestamp"}, "direction": "desc"}],
          "limit": 5
        }
      }
    ]
  }
}`, start, now)

	if err := os.WriteFile(payloadPath, []byte(payload), 0o644); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	stdout, stderr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"query", "traces",
		"--file", payloadPath,
	)
	if err != nil {
		t.Fatalf("query traces failed: %v stderr=%s", err, stderr)
	}
	resp := mustJSONMap(t, stdout)
	if status, ok := resp["status"].(string); !ok || status == "" {
		t.Fatalf("unexpected query response: %v", resp)
	}
}

func TestLiveDashboardCreateAndDelete(t *testing.T) {
	cfgPath, profile := requireLiveEnv(t)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "dashboard.json")
	title := fmt.Sprintf("signozctl-e2e-%d", time.Now().UnixNano())

	body := fmt.Sprintf(`{
  "title": %q,
  "description": "signozctl e2e test",
  "widgets": [],
  "layout": [],
  "variables": {}
}`, title)
	if err := os.WriteFile(filePath, []byte(body), 0o644); err != nil {
		t.Fatalf("failed to write dashboard payload: %v", err)
	}

	createOut, createErr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"dashboard", "create",
		"--file", filePath,
	)
	if err != nil {
		t.Fatalf("dashboard create failed: %v stderr=%s", err, createErr)
	}

	resp := mustJSONMap(t, createOut)
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected create response data: %v", resp)
	}
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatalf("dashboard id missing in create response: %v", resp)
	}

	_, deleteErr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"dashboard", "delete", id,
	)
	if err != nil {
		t.Fatalf("dashboard delete failed for id=%s: %v stderr=%s", id, err, deleteErr)
	}
}
