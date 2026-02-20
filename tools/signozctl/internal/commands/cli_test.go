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
	"time"
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

func TestQueryTracesLastFlagOverridesStartEnd(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	payloadPath := filepath.Join(tmpDir, "trace-query.json")

	if err := os.WriteFile(payloadPath, []byte(`{"start":1,"end":2,"requestType":"trace","compositeQuery":{"queries":[]}}`), 0o644); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	var gotStart float64
	var gotEnd float64
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v5/query_range", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		gotStart, _ = payload["start"].(float64)
		gotEnd, _ = payload["end"].(float64)

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"result":[]}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, stderr, err := runCLI(
		t,
		"--config", cfgPath,
		"--output", "json",
		"query", "traces",
		"--file", payloadPath,
		"--last", "5m",
	)
	if err != nil {
		t.Fatalf("expected query traces to succeed, err=%v stderr=%s", err, stderr)
	}

	if gotEnd <= gotStart {
		t.Fatalf("expected end > start, got start=%v end=%v", gotStart, gotEnd)
	}
	diff := time.Duration(int64(gotEnd-gotStart)) * time.Millisecond
	if diff < 4*time.Minute || diff > 6*time.Minute {
		t.Fatalf("expected ~5m diff, got %v", diff)
	}
}

func TestQueryTracesLastFlagRejectsInvalidDuration(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	payloadPath := filepath.Join(tmpDir, "trace-query.json")

	if err := os.WriteFile(payloadPath, []byte(`{"requestType":"trace","compositeQuery":{"queries":[]}}`), 0o644); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"http://example.com","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, stderr, err := runCLI(
		t,
		"--config", cfgPath,
		"--output", "json",
		"query", "traces",
		"--file", payloadPath,
		"--last", "banana",
	)
	if err == nil {
		t.Fatalf("expected invalid --last to fail")
	}
	if !strings.Contains(stderr, "invalid --last value") {
		t.Fatalf("expected invalid --last error, got stderr=%q", stderr)
	}
}

func TestQueryTemplateTracesProducesRunnablePayload(t *testing.T) {
	stdout, stderr, err := runCLI(t, "--output", "json", "query", "template", "--signal", "traces")
	if err != nil {
		t.Fatalf("expected template command to succeed, err=%v stderr=%s", err, stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected json payload, got %q: %v", stdout, err)
	}
	if payload["requestType"] != "trace" {
		t.Fatalf("expected requestType=trace, got %#v", payload["requestType"])
	}
	start, ok := payload["start"].(float64)
	if !ok {
		t.Fatalf("expected numeric start")
	}
	end, ok := payload["end"].(float64)
	if !ok {
		t.Fatalf("expected numeric end")
	}
	if end <= start {
		t.Fatalf("expected end > start, got start=%v end=%v", start, end)
	}
}

func TestQuerySchemaMetricsReturnsFields(t *testing.T) {
	stdout, stderr, err := runCLI(t, "--output", "json", "query", "schema", "--signal", "metrics")
	if err != nil {
		t.Fatalf("expected schema command to succeed, err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"signal":"metrics"`) {
		t.Fatalf("expected metrics signal in schema output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"requestType":"time_series"`) {
		t.Fatalf("expected time_series request type in schema output, got %q", stdout)
	}
}

func TestQueryValidateRejectsMissingCompositeQuery(t *testing.T) {
	tmpDir := t.TempDir()
	payloadPath := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(payloadPath, []byte(`{"schemaVersion":"v1","requestType":"trace","compositeQuery":{}}`), 0o644); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	_, stderr, err := runCLI(t, "--output", "json", "query", "validate", "--file", payloadPath)
	if err == nil {
		t.Fatalf("expected validate to fail for invalid payload")
	}
	if !strings.Contains(stderr, "compositeQuery.queries") {
		t.Fatalf("expected compositeQuery validation error, got stderr=%q", stderr)
	}
}

func TestQueryValidateAcceptsValidPayload(t *testing.T) {
	tmpDir := t.TempDir()
	payloadPath := filepath.Join(tmpDir, "good.json")
	if err := os.WriteFile(payloadPath, []byte(`{"schemaVersion":"v1","start":10,"end":20,"requestType":"trace","compositeQuery":{"queries":[{"type":"builder_query","spec":{"name":"A","signal":"traces","aggregations":[{"expression":"count()"}]}}]}}`), 0o644); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	stdout, stderr, err := runCLI(t, "--output", "json", "query", "validate", "--file", payloadPath)
	if err != nil {
		t.Fatalf("expected validate to pass, err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"valid":true`) {
		t.Fatalf("expected valid=true output, got %q", stdout)
	}
}

func TestQueryTraceWaterfallPostsTraceIDPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	payloadPath := filepath.Join(tmpDir, "waterfall.json")
	if err := os.WriteFile(payloadPath, []byte(`{"start":1,"end":2}`), 0o644); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	var hitPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/traces/waterfall/trace-123", func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"spans":[]}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, stderr, err := runCLI(
		t, "--config", cfgPath, "--output", "json",
		"query", "trace-waterfall", "trace-123", "--file", payloadPath,
	)
	if err != nil {
		t.Fatalf("expected trace-waterfall to succeed, err=%v stderr=%s", err, stderr)
	}
	if hitPath != "/api/v2/traces/waterfall/trace-123" {
		t.Fatalf("unexpected endpoint path: %s", hitPath)
	}
}

func TestQueryTraceWaterfallWithoutFileSendsEmptyBody(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	var body string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/traces/waterfall/trace-123", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"spans":[]}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, stderr, err := runCLI(
		t, "--config", cfgPath, "--output", "json",
		"query", "trace-waterfall", "trace-123",
	)
	if err != nil {
		t.Fatalf("expected trace-waterfall without file to succeed, err=%v stderr=%s", err, stderr)
	}
	if strings.TrimSpace(body) != "{}" {
		t.Fatalf("expected empty object body, got %q", body)
	}
}

func TestQueryTraceFlamegraphWithoutFileSendsEmptyBody(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	var body string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/traces/flamegraph/trace-123", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"flamegraph":[]}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, stderr, err := runCLI(
		t, "--config", cfgPath, "--output", "json",
		"query", "trace-flamegraph", "trace-123",
	)
	if err != nil {
		t.Fatalf("expected trace-flamegraph without file to succeed, err=%v stderr=%s", err, stderr)
	}
	if strings.TrimSpace(body) != "{}" {
		t.Fatalf("expected empty object body, got %q", body)
	}
}

func TestQueryTraceWaterfallFlagsBuildBodyWithoutFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	var payload map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/traces/waterfall/trace-123", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"spans":[]}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, stderr, err := runCLI(
		t, "--config", cfgPath, "--output", "json",
		"query", "trace-waterfall", "trace-123",
		"--selected-span-id", "abc",
		"--expand-selected",
		"--uncollapse-span", "root1",
		"--uncollapse-span", "root2",
	)
	if err != nil {
		t.Fatalf("expected trace-waterfall with flags to succeed, err=%v stderr=%s", err, stderr)
	}
	if payload["selectedSpanId"] != "abc" {
		t.Fatalf("expected selectedSpanId=abc, got %#v", payload["selectedSpanId"])
	}
	if payload["isSelectedSpanIDUnCollapsed"] != true {
		t.Fatalf("expected isSelectedSpanIDUnCollapsed=true, got %#v", payload["isSelectedSpanIDUnCollapsed"])
	}
	gotUncollapsed, ok := payload["uncollapsedSpans"].([]any)
	if !ok || len(gotUncollapsed) != 2 {
		t.Fatalf("expected 2 uncollapsed spans, got %#v", payload["uncollapsedSpans"])
	}
}

func TestQueryTraceFlamegraphSelectedSpanFlagBuildsBodyWithoutFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	var payload map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/traces/flamegraph/trace-123", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"flamegraph":[]}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, stderr, err := runCLI(
		t, "--config", cfgPath, "--output", "json",
		"query", "trace-flamegraph", "trace-123",
		"--selected-span-id", "xyz",
	)
	if err != nil {
		t.Fatalf("expected trace-flamegraph with flags to succeed, err=%v stderr=%s", err, stderr)
	}
	if payload["selectedSpanId"] != "xyz" {
		t.Fatalf("expected selectedSpanId=xyz, got %#v", payload["selectedSpanId"])
	}
}

func TestDashboardPublicCreateEnabledFlagWithoutFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	var payload map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/dashboards/dash-1/public", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"id":"pub-1"}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, stderr, err := runCLI(
		t, "--config", cfgPath, "--output", "json",
		"dashboard", "public-create", "dash-1", "--enabled",
	)
	if err != nil {
		t.Fatalf("expected dashboard public-create with --enabled to succeed, err=%v stderr=%s", err, stderr)
	}
	if payload["isEnabled"] != true {
		t.Fatalf("expected isEnabled=true, got %#v", payload["isEnabled"])
	}
}

func TestQueryTraceRootReturnsRootSpanID(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/traces/waterfall/trace-xyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"spans":[{"spanId":"root-123"}]}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	stdout, stderr, err := runCLI(
		t, "--config", cfgPath, "--output", "json",
		"query", "trace-root", "trace-xyz",
	)
	if err != nil {
		t.Fatalf("expected trace-root to succeed, err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"rootSpanId":"root-123"`) {
		t.Fatalf("expected rootSpanId in output, got %q", stdout)
	}
}

func TestQueryTraceWaterfallExpandAllAutoUncollapses(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	callCount := 0
	var lastUncollapsed []any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/traces/waterfall/trace-xyz", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		defer r.Body.Close()
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if items, ok := req["uncollapsedSpans"].([]any); ok {
			lastUncollapsed = items
		}

		w.Header().Set("Content-Type", "application/json")
		// first call exposes root with children, second call exposes child with children, third stabilizes
		if callCount == 1 {
			_, _ = io.WriteString(w, `{"spans":[{"spanId":"root","hasChildren":true}]}`)
			return
		}
		if callCount == 2 {
			_, _ = io.WriteString(w, `{"spans":[{"spanId":"root","hasChildren":true},{"spanId":"child","hasChildren":true}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"spans":[{"spanId":"root","hasChildren":true},{"spanId":"child","hasChildren":true},{"spanId":"leaf","hasChildren":false}]}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, stderr, err := runCLI(
		t, "--config", cfgPath, "--output", "json",
		"query", "trace-waterfall", "trace-xyz", "--expand-all",
	)
	if err != nil {
		t.Fatalf("expected trace-waterfall --expand-all to succeed, err=%v stderr=%s", err, stderr)
	}
	if callCount < 2 {
		t.Fatalf("expected multiple calls for expand-all, got %d", callCount)
	}
	if len(lastUncollapsed) == 0 {
		t.Fatalf("expected uncollapsedSpans in request during expand-all")
	}
}

func TestDashboardPublicCreatePostsPublicEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	payloadPath := filepath.Join(tmpDir, "public.json")
	if err := os.WriteFile(payloadPath, []byte(`{"enable":true}`), 0o644); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	var hitPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/dashboards/dash-1/public", func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"success","data":{"id":"pub-1"}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfgJSON := `{"activeProfile":"local","profiles":{"local":{"host":"` + server.URL + `","accessToken":"token-123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, stderr, err := runCLI(
		t, "--config", cfgPath, "--output", "json",
		"dashboard", "public-create", "dash-1", "--file", payloadPath,
	)
	if err != nil {
		t.Fatalf("expected dashboard public-create to succeed, err=%v stderr=%s", err, stderr)
	}
	if hitPath != "/api/v1/dashboards/dash-1/public" {
		t.Fatalf("unexpected endpoint path: %s", hitPath)
	}
}

func TestDashboardTemplateSchemaValidate(t *testing.T) {
	stdout, stderr, err := runCLI(t, "--output", "json", "dashboard", "template", "--resource", "create")
	if err != nil {
		t.Fatalf("dashboard template failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"title"`) {
		t.Fatalf("expected title in template output, got %q", stdout)
	}

	stdout, stderr, err = runCLI(t, "--output", "json", "dashboard", "schema", "--resource", "public-create")
	if err != nil {
		t.Fatalf("dashboard schema failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"required"`) {
		t.Fatalf("expected required in schema output, got %q", stdout)
	}

	stdout, stderr, err = runCLI(t, "--output", "json", "dashboard", "schema", "--resource", "create")
	if err != nil {
		t.Fatalf("dashboard create schema failed: %v stderr=%s", err, stderr)
	}
	for _, expected := range []string{`"widgets[].query"`, `"layout[].i"`, `"variables.\u003cname\u003e.type"`} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected %s in dashboard create schema, got %q", expected, stdout)
		}
	}

	tmpDir := t.TempDir()
	payloadPath := filepath.Join(tmpDir, "dash.json")
	if err := os.WriteFile(payloadPath, []byte(`{"title":"X","description":"Y","widgets":[],"layout":[],"variables":{}}`), 0o644); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}
	stdout, stderr, err = runCLI(t, "--output", "json", "dashboard", "validate", "--resource", "create", "--file", payloadPath)
	if err != nil {
		t.Fatalf("dashboard validate failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"valid":true`) {
		t.Fatalf("expected valid=true, got %q", stdout)
	}
}

func TestAlertsAndIAMTemplateSchemaValidate(t *testing.T) {
	stdout, stderr, err := runCLI(t, "--output", "json", "alerts", "template", "--resource", "rule")
	if err != nil {
		t.Fatalf("alerts template failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"alert"` ) && !strings.Contains(stdout, `"name"`) {
		t.Fatalf("unexpected alerts template output: %q", stdout)
	}

	stdout, stderr, err = runCLI(t, "--output", "json", "iam", "schema", "--resource", "api-key")
	if err != nil {
		t.Fatalf("iam schema failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"required"`) {
		t.Fatalf("expected required in iam schema, got %q", stdout)
	}

	tmpDir := t.TempDir()
	payloadPath := filepath.Join(tmpDir, "invite.json")
	if err := os.WriteFile(payloadPath, []byte(`{"email":"user@example.com","role":"VIEWER"}`), 0o644); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}
	stdout, stderr, err = runCLI(t, "--output", "json", "iam", "validate", "--resource", "invite", "--file", payloadPath)
	if err != nil {
		t.Fatalf("iam validate failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"valid":true`) {
		t.Fatalf("expected valid=true, got %q", stdout)
	}
}
