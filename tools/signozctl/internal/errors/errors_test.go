package errors

import "testing"

func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		status int
		want   Class
	}{
		{status: 400, want: ClassInputValidation},
		{status: 401, want: ClassAuthRequired},
		{status: 403, want: ClassAuthInsufficientRole},
		{status: 404, want: ClassResourceNotFound},
		{status: 429, want: ClassRateLimited},
		{status: 500, want: ClassAPIUnavailable},
		{status: 503, want: ClassAPIUnavailable},
		{status: 418, want: ClassUnknown},
	}

	for _, tc := range cases {
		t.Run(string(tc.want), func(t *testing.T) {
			if got := classifyStatus(tc.status); got != tc.want {
				t.Fatalf("status=%d got=%s want=%s", tc.status, got, tc.want)
			}
		})
	}
}

func TestParseAPIErrorIncludesCodeMessageAndHint(t *testing.T) {
	body := []byte(`{"status":"error","error":{"code":"invalid_input","message":"bad payload"}}`)
	err := ParseAPIError(400, body)

	if err.Code != "invalid_input" {
		t.Fatalf("unexpected code: %s", err.Code)
	}
	if err.Message != "bad payload" {
		t.Fatalf("unexpected message: %s", err.Message)
	}
	if err.Class != ClassInputValidation {
		t.Fatalf("unexpected class: %s", err.Class)
	}
	if err.Hint == "" {
		t.Fatalf("expected non-empty hint")
	}
}

