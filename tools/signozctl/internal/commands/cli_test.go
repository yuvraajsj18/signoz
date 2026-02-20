package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := NewRootCommand()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestRootHelpListsMajorCommandGroups(t *testing.T) {
	stdout, _, err := runCLI(t, "--help")
	if err != nil {
		t.Fatalf("expected help command to succeed: %v", err)
	}

	for _, expected := range []string{"auth", "query", "dashboard", "alerts", "iam", "system", "docs"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected help output to contain %q, got:\n%s", expected, stdout)
		}
	}
}

func TestAuthLoginThenStatusWithStoredProfile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/sessions/context", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET for context, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"exists":true,"orgs":[{"id":"org-1","name":"Default"}]}}`)
	})
	mux.HandleFunc("/api/v2/sessions/email_password", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST for login, got %s", r.Method)
		}
		defer r.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode login payload: %v", err)
		}
		if payload["orgId"] != "org-1" {
			t.Fatalf("expected orgId org-1, got %#v", payload["orgId"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"accessToken":"access-1","refreshToken":"refresh-1"}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	_, stderr, err := runCLI(
		t,
		"--config", cfgPath,
		"auth", "login",
		"--host", server.URL,
		"--email", "dev@example.com",
		"--password", "secret",
		"--profile", "local",
	)
	if err != nil {
		t.Fatalf("expected login to succeed, err=%v stderr=%s", err, stderr)
	}

	stdout, stderr, err := runCLI(
		t,
		"--config", cfgPath,
		"--output", "json",
		"auth", "status",
		"--profile", "local",
	)
	if err != nil {
		t.Fatalf("expected status to succeed, err=%v stderr=%s", err, stderr)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("expected json status output, got %q: %v", stdout, err)
	}
	if out["profile"] != "local" {
		t.Fatalf("expected profile local, got %#v", out["profile"])
	}
	if out["host"] != server.URL {
		t.Fatalf("expected host %s, got %#v", server.URL, out["host"])
	}
	if out["authenticated"] != true {
		t.Fatalf("expected authenticated=true, got %#v", out["authenticated"])
	}
}

func TestQueryTracesPostsPayloadWithAuth(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	payloadPath := filepath.Join(tmpDir, "trace-query.json")

	if err := os.WriteFile(payloadPath, []byte(`{"requestType":"trace","compositeQuery":{"queries":[]}}`), 0o644); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v5/query_range", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"result":[]}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	stdout, stderr, err := runCLI(
		t,
		"--config", cfgPath,
		"--output", "json",
		"query", "traces",
		"--file", payloadPath,
	)
	if err != nil {
		t.Fatalf("expected query traces to succeed, err=%v stderr=%s", err, stderr)
	}
	if gotAuth != "Bearer token-123" {
		t.Fatalf("expected auth header, got %q", gotAuth)
	}
	if !strings.Contains(stdout, `"status":"success"`) {
		t.Fatalf("expected success response in stdout, got %q", stdout)
	}
}

func TestDashboardCreatePostsFilePayload(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	dashboardPath := filepath.Join(tmpDir, "dashboard.json")

	body := `{"title":"CLI Dashboard","description":"created by test","widgets":[],"layout":[],"variables":{}}`
	if err := os.WriteFile(dashboardPath, []byte(body), 0o644); err != nil {
		t.Fatalf("failed to write dashboard json: %v", err)
	}

	var gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/dashboards", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"id":"dash-1","data":{"title":"CLI Dashboard"}}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	stdout, stderr, err := runCLI(
		t,
		"--config", cfgPath,
		"--output", "json",
		"dashboard", "create",
		"--file", dashboardPath,
	)
	if err != nil {
		t.Fatalf("expected dashboard create to succeed, err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(gotBody, `"title":"CLI Dashboard"`) {
		t.Fatalf("expected request body to include dashboard title, got %q", gotBody)
	}
	if !strings.Contains(stdout, `"dash-1"`) {
		t.Fatalf("expected response to include dashboard id, got %q", stdout)
	}
}
