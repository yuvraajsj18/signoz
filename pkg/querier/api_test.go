package querier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SigNoz/signoz/pkg/analytics"
	"github.com/SigNoz/signoz/pkg/factory/factorytest"
	"github.com/SigNoz/signoz/pkg/types/authtypes"
	qbtypes "github.com/SigNoz/signoz/pkg/types/querybuildertypes/querybuildertypesv5"
	"github.com/SigNoz/signoz/pkg/valuer"
)

type mockQuerier struct{}

func (m *mockQuerier) QueryRange(_ context.Context, _ valuer.UUID, _ *qbtypes.QueryRangeRequest) (*qbtypes.QueryRangeResponse, error) {
	return &qbtypes.QueryRangeResponse{}, nil
}

func (m *mockQuerier) QueryRawStream(_ context.Context, _ valuer.UUID, _ *qbtypes.QueryRangeRequest, _ *qbtypes.RawStream) {
}

func withClaims(req *http.Request) *http.Request {
	ctx := authtypes.NewContextWithClaims(req.Context(), authtypes.Claims{
		UserID: "user-1",
		OrgID:  "019c7cee-c246-7b4b-b487-e9c2a709c64e",
	})
	return req.WithContext(ctx)
}

func TestQueryRangeInvalidJSONBodyIsActionable(t *testing.T) {
	api := NewAPI(factorytest.NewSettings(), &mockQuerier{}, analytics.NewNoop())
	req := httptest.NewRequest(http.MethodPost, "/api/v5/query_range", strings.NewReader(`{"schemaVersion":"v1",`))
	req = withClaims(req)
	rr := httptest.NewRecorder()

	api.QueryRange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "\"code\":\"invalid_input\"") {
		t.Fatalf("expected invalid_input code, body=%s", body)
	}
	if !strings.Contains(body, "invalid JSON request body") {
		t.Fatalf("expected actionable JSON message, body=%s", body)
	}
}

func TestQueryRangeRejectsTrailingTokens(t *testing.T) {
	api := NewAPI(factorytest.NewSettings(), &mockQuerier{}, analytics.NewNoop())
	req := httptest.NewRequest(http.MethodPost, "/api/v5/query_range", strings.NewReader(`{} {}`))
	req = withClaims(req)
	rr := httptest.NewRecorder()

	api.QueryRange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "\"code\":\"invalid_input\"") {
		t.Fatalf("expected invalid_input code, body=%s", body)
	}
	if !strings.Contains(body, "single JSON object") {
		t.Fatalf("expected trailing-token guidance, body=%s", body)
	}
}
