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

func TestLiveTraceRootWaterfallFlamegraph(t *testing.T) {
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
          "limit": 1
        }
      }
    ]
  }
}`, start, now)

	if err := os.WriteFile(payloadPath, []byte(payload), 0o644); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	out, stderr, err := runLiveCLI(
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
	resp := mustJSONMap(t, out)
	traceID := extractTraceIDFromQueryResponse(resp)
	if traceID == "" {
		t.Skip("no trace id found in recent trace query results; ensure traffic generation is active")
	}

	rootOut, rootErr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"query", "trace-root", traceID,
	)
	if err != nil {
		t.Fatalf("trace-root failed for traceID=%s: %v stderr=%s", traceID, err, rootErr)
	}
	rootResp := mustJSONMap(t, rootOut)
	rootSpanID, _ := rootResp["rootSpanId"].(string)
	if rootSpanID == "" {
		t.Fatalf("trace-root did not return rootSpanId: %v", rootResp)
	}

	waterfallOut, waterfallErr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"query", "trace-waterfall", traceID,
		"--expand-all",
	)
	if err != nil {
		t.Fatalf("trace-waterfall failed for traceID=%s: %v stderr=%s", traceID, err, waterfallErr)
	}
	if !strings.Contains(waterfallOut, `"spans"`) {
		t.Fatalf("unexpected trace-waterfall response: %s", waterfallOut)
	}

	flameOut, flameErr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"query", "trace-flamegraph", traceID,
		"--selected-span-id", rootSpanID,
	)
	if err != nil {
		t.Fatalf("trace-flamegraph failed for traceID=%s rootSpanID=%s: %v stderr=%s", traceID, rootSpanID, err, flameErr)
	}
	if !strings.Contains(flameOut, "{") {
		t.Fatalf("unexpected trace-flamegraph response: %s", flameOut)
	}
}

func TestLiveDashboardPublicCreateAndDelete(t *testing.T) {
	cfgPath, profile := requireLiveEnv(t)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "dashboard.json")
	title := fmt.Sprintf("signozctl-public-e2e-%d", time.Now().UnixNano())

	body := fmt.Sprintf(`{
  "title": %q,
  "description": "signozctl public e2e test",
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

	_, publicCreateErr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"dashboard", "public-create", id,
		"--enabled",
	)
	if err != nil {
		_, _, _ = runLiveCLI(
			t,
			"--config", cfgPath,
			"--profile", profile,
			"--output", "json",
			"dashboard", "delete", id,
		)
		if isSkippableLiveError(publicCreateErr) {
			t.Skipf("skipping public dashboard e2e due to environment/license: %s", publicCreateErr)
		}
		t.Fatalf("dashboard public-create failed for id=%s: %v stderr=%s", id, err, publicCreateErr)
	}

	_, publicDeleteErr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"dashboard", "public-delete", id,
	)
	if err != nil {
		if isSkippableLiveError(publicDeleteErr) {
			t.Skipf("skipping public-delete due to environment/license: %s", publicDeleteErr)
		}
		t.Fatalf("dashboard public-delete failed for id=%s: %v stderr=%s", id, err, publicDeleteErr)
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

func TestLiveSystemApdexReadWriteRoundTrip(t *testing.T) {
	cfgPath, profile := requireLiveEnv(t)

	getOut, getErr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"system", "apdex",
	)
	if err != nil {
		if isSkippableLiveError(getErr) {
			t.Skipf("skipping apdex round-trip due to environment/permission: %s", getErr)
		}
		t.Fatalf("system apdex failed: %v stderr=%s", err, getErr)
	}

	payloadPath := filepath.Join(t.TempDir(), "apdex.json")
	if err := os.WriteFile(payloadPath, []byte(getOut), 0o644); err != nil {
		t.Fatalf("failed to write apdex payload: %v", err)
	}

	_, setErr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"system", "set-apdex",
		"--file", payloadPath,
	)
	if err != nil {
		if isSkippableLiveError(setErr) {
			t.Skipf("skipping set-apdex due to environment/permission: %s", setErr)
		}
		t.Fatalf("system set-apdex failed: %v stderr=%s", err, setErr)
	}
}

func TestLiveIAMAPIKeyCreateAndRevoke(t *testing.T) {
	cfgPath, profile := requireLiveEnv(t)
	tmpDir := t.TempDir()
	payloadPath := filepath.Join(tmpDir, "api-key.json")
	payload := `{"name":"signozctl-e2e-key","role":"ADMIN","expiresInDays":1}`
	if err := os.WriteFile(payloadPath, []byte(payload), 0o644); err != nil {
		t.Fatalf("failed to write api key payload: %v", err)
	}

	createOut, createErr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"iam", "api-keys", "create",
		"--file", payloadPath,
	)
	if err != nil {
		if isSkippableLiveError(createErr) {
			t.Skipf("skipping api-key create due to environment/permission: %s", createErr)
		}
		t.Fatalf("api-key create failed: %v stderr=%s", err, createErr)
	}
	resp := mustJSONMap(t, createOut)
	data, _ := resp["data"].(map[string]any)
	id, _ := data["id"].(string)
	if id == "" {
		t.Skipf("api-key create returned no id (response shape may differ): %v", resp)
	}

	_, revokeErr, err := runLiveCLI(
		t,
		"--config", cfgPath,
		"--profile", profile,
		"--output", "json",
		"iam", "api-keys", "revoke", id,
	)
	if err != nil {
		if isSkippableLiveError(revokeErr) {
			t.Skipf("skipping api-key revoke due to environment/permission: %s", revokeErr)
		}
		t.Fatalf("api-key revoke failed for id=%s: %v stderr=%s", id, err, revokeErr)
	}
}

func TestLiveAlertsEndpointsBasicCoverage(t *testing.T) {
	cfgPath, profile := requireLiveEnv(t)

	commands := [][]string{
		{"alerts", "list"},
		{"alerts", "rules", "list"},
		{"alerts", "channels", "list"},
		{"alerts", "route-policies", "list"},
		{"alerts", "downtime", "list"},
	}
	for _, c := range commands {
		args := []string{"--config", cfgPath, "--profile", profile, "--output", "json"}
		args = append(args, c...)
		_, stderr, err := runLiveCLI(t, args...)
		if err != nil {
			if isSkippableLiveError(stderr) {
				t.Skipf("skipping alerts coverage due to environment/permission: %s", stderr)
			}
			t.Fatalf("alerts command %v failed: %v stderr=%s", c, err, stderr)
		}
	}
}

func extractTraceIDFromQueryResponse(resp map[string]any) string {
	data, _ := resp["data"].(map[string]any)
	dataObj, _ := data["data"].(map[string]any)
	results, _ := dataObj["results"].([]any)
	if len(results) == 0 {
		return ""
	}
	firstResult, _ := results[0].(map[string]any)
	rows, _ := firstResult["rows"].([]any)
	if len(rows) == 0 {
		return ""
	}
	firstRow, _ := rows[0].(map[string]any)
	rowData, _ := firstRow["data"].(map[string]any)
	traceID, _ := rowData["trace_id"].(string)
	return traceID
}

func isSkippableLiveError(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "class=auth_insufficient_role") ||
		strings.Contains(lower, "class=auth_required") ||
		strings.Contains(lower, "class=api_unavailable") ||
		strings.Contains(lower, "status=403") ||
		strings.Contains(lower, "status=404") ||
		strings.Contains(lower, "status=451") ||
		strings.Contains(lower, "license_unavailable")
}
