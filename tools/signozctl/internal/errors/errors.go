package errors

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Class string

const (
	ClassAuthRequired         Class = "auth_required"
	ClassAuthInsufficientRole Class = "auth_insufficient_role"
	ClassInputValidation      Class = "input_validation"
	ClassResourceNotFound     Class = "resource_not_found"
	ClassAPIUnavailable       Class = "api_unavailable"
	ClassRateLimited          Class = "rate_limited"
	ClassUnknown              Class = "unknown_error"
)

type APIError struct {
	Status  int
	Class   Class
	Code    string
	Message string
	Hint    string
	RawBody string
}

func (e *APIError) Error() string {
	parts := []string{
		fmt.Sprintf("class=%s", e.Class),
		fmt.Sprintf("status=%d", e.Status),
	}
	if e.Code != "" {
		parts = append(parts, fmt.Sprintf("code=%s", e.Code))
	}
	if e.Message != "" {
		parts = append(parts, fmt.Sprintf("message=%s", e.Message))
	}
	if e.Hint != "" {
		parts = append(parts, fmt.Sprintf("hint=%s", e.Hint))
	}
	return strings.Join(parts, " ")
}

func ParseAPIError(status int, body []byte) *APIError {
	err := &APIError{
		Status:  status,
		Class:   classifyStatus(status),
		RawBody: string(body),
	}

	var decoded struct {
		Status string `json:"status"`
		Error  struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &decoded) == nil {
		err.Code = decoded.Error.Code
		if decoded.Error.Message != "" {
			err.Message = decoded.Error.Message
		} else if decoded.Message != "" {
			err.Message = decoded.Message
		}
	}

	if err.Message == "" {
		err.Message = strings.TrimSpace(string(body))
	}
	err.Hint = hintForClass(err.Class)
	return err
}

func classifyStatus(status int) Class {
	switch status {
	case 400:
		return ClassInputValidation
	case 401:
		return ClassAuthRequired
	case 403:
		return ClassAuthInsufficientRole
	case 404:
		return ClassResourceNotFound
	case 429:
		return ClassRateLimited
	case 500, 502, 503, 504:
		return ClassAPIUnavailable
	default:
		return ClassUnknown
	}
}

func hintForClass(class Class) string {
	switch class {
	case ClassAuthRequired:
		return "run `signozctl auth login --host <url> --email <email> --password <password> --profile <name>`"
	case ClassAuthInsufficientRole:
		return "verify your user role or use an admin profile (`signozctl auth status --profile <name>`)"
	case ClassInputValidation:
		return "check request payload and required flags (`--help` and examples in tools/signozctl/examples)"
	case ClassResourceNotFound:
		return "verify resource ID and profile host"
	case ClassRateLimited:
		return "retry with backoff and reduce request frequency"
	case ClassAPIUnavailable:
		return "check SigNoz server health (`signozctl system health --host <url> --output json`)"
	default:
		return "run the command with valid profile/flags and inspect response details"
	}
}
