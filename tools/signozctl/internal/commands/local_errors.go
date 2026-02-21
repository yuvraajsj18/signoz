package commands

import (
	"fmt"

	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
)

func errInvalidOption(field, value, allowed string) error {
	return signozerrors.NewInputValidationError("invalid_option", fmt.Sprintf("unsupported %s %q: use %s", field, value, allowed))
}

func errProfileNotFound(name string) error {
	return signozerrors.NewInputValidationError("profile_not_found", fmt.Sprintf("profile not found: %s", name))
}

func errMissingHost() error {
	return signozerrors.NewInputValidationError("missing_host", "missing host: use --host or authenticate with a profile")
}

func errInvalidJSONPayload(err error) error {
	return signozerrors.NewInputValidationError("invalid_json_payload", fmt.Sprintf("invalid JSON payload: %v", err))
}
