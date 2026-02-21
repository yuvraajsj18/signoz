package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
)

func newDashboardTemplateCommand(flags *globalFlags) *cobra.Command {
	var resource string
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Generate runnable dashboard payload template",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tpl, err := dashboardTemplate(strings.ToLower(strings.TrimSpace(resource)))
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, tpl)
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "create", "resource: create|update|public-create|public-update")
	return cmd
}

func newDashboardSchemaCommand(flags *globalFlags) *cobra.Command {
	var resource string
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Show dashboard payload schema idea",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := dashboardSchema(strings.ToLower(strings.TrimSpace(resource)))
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, s)
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "create", "resource: create|update|public-create|public-update")
	return cmd
}

func newDashboardValidateCommand(flags *globalFlags) *cobra.Command {
	var resource string
	var filePath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate dashboard payload file locally",
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := loadPayloadFile(filePath)
			if err != nil {
				return err
			}
			if err := validateDashboardPayload(strings.ToLower(strings.TrimSpace(resource)), payload); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{"valid": true, "resource": resource, "file": filePath})
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "create", "resource: create|update|public-create|public-update")
	cmd.Flags().StringVar(&filePath, "file", "", "JSON payload file")
	return cmd
}

func newAlertsTemplateCommand(flags *globalFlags) *cobra.Command {
	var resource string
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Generate runnable alerts payload template",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tpl, err := alertsTemplate(strings.ToLower(strings.TrimSpace(resource)))
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, tpl)
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "rule", "resource: rule|channel|route-policy|downtime")
	return cmd
}

func newAlertsSchemaCommand(flags *globalFlags) *cobra.Command {
	var resource string
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Show alerts payload schema idea",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := alertsSchema(strings.ToLower(strings.TrimSpace(resource)))
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, s)
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "rule", "resource: rule|channel|route-policy|downtime")
	return cmd
}

func newAlertsValidateCommand(flags *globalFlags) *cobra.Command {
	var resource string
	var filePath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate alerts payload file locally",
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := loadPayloadFile(filePath)
			if err != nil {
				return err
			}
			if err := validateAlertsPayload(strings.ToLower(strings.TrimSpace(resource)), payload); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{"valid": true, "resource": resource, "file": filePath})
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "rule", "resource: rule|channel|route-policy|downtime")
	cmd.Flags().StringVar(&filePath, "file", "", "JSON payload file")
	return cmd
}

func newIAMTemplateCommand(flags *globalFlags) *cobra.Command {
	var resource string
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Generate runnable IAM payload template",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tpl, err := iamTemplate(strings.ToLower(strings.TrimSpace(resource)))
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, tpl)
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "invite", "resource: invite|role|api-key")
	return cmd
}

func newIAMSchemaCommand(flags *globalFlags) *cobra.Command {
	var resource string
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Show IAM payload schema idea",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := iamSchema(strings.ToLower(strings.TrimSpace(resource)))
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, s)
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "invite", "resource: invite|role|api-key")
	return cmd
}

func newIAMValidateCommand(flags *globalFlags) *cobra.Command {
	var resource string
	var filePath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate IAM payload file locally",
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := loadPayloadFile(filePath)
			if err != nil {
				return err
			}
			if err := validateIAMPayload(strings.ToLower(strings.TrimSpace(resource)), payload); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{"valid": true, "resource": resource, "file": filePath})
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "invite", "resource: invite|role|api-key")
	cmd.Flags().StringVar(&filePath, "file", "", "JSON payload file")
	return cmd
}

func dashboardTemplate(resource string) (map[string]any, error) {
	switch resource {
	case "create", "update":
		return map[string]any{
			"title":       "signozctl dashboard",
			"description": "created by signozctl",
			"widgets": []any{
				map[string]any{
					"id":          "widget-1",
					"panelTypes":  "TIME_SERIES",
					"title":       "Request rate",
					"description": "sample widget",
					"query": map[string]any{
						"queryType": "builder",
						"builder": map[string]any{
							"queryData": []any{},
						},
					},
				},
			},
			"layout": []any{
				map[string]any{
					"i": "widget-1", "x": 0, "y": 0, "w": 6, "h": 4,
				},
			},
			"variables": map[string]any{
				"service": map[string]any{
					"type":        "QUERY",
					"description": "service selector",
				},
			},
		}, nil
	case "public-create", "public-update":
		return map[string]any{
			"timeRangeEnabled": true,
			"defaultTimeRange": "5m",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported dashboard resource %q", resource)
	}
}

func dashboardSchema(resource string) (map[string]any, error) {
	switch resource {
	case "create", "update":
		return map[string]any{
			"resource": resource,
			"required": []string{"title"},
			"optional": []string{
				"description",
				"widgets",
				"layout",
				"variables",
				"panelMap",
				"tags",
				"version",
			},
			"nestedHints": []string{
				"widgets[].id",
				"widgets[].panelTypes",
				"widgets[].title",
				"widgets[].query",
				"widgets[].query.queryType",
				"widgets[].query.builder.queryData",
				"layout[].i",
				"layout[].x",
				"layout[].y",
				"layout[].w",
				"layout[].h",
				"variables.<name>.type",
				"variables.<name>.description",
			},
			"compatibilityNote": "Dashboard JSON shape evolves; validate against API and existing exported dashboards.",
		}, nil
	case "public-create", "public-update":
		return map[string]any{
			"resource": resource,
			"required": []string{},
			"optional": []string{"timeRangeEnabled", "defaultTimeRange"},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported dashboard resource %q", resource)
	}
}

func validateDashboardPayload(resource string, payload map[string]any) error {
	switch resource {
	case "create", "update":
		if _, ok := payload["title"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "title is required and must be a string")
		}
	case "public-create", "public-update":
		if v, ok := payload["timeRangeEnabled"]; ok {
			if _, ok := v.(bool); !ok {
				return signozerrors.NewInputValidationError("invalid_payload", "timeRangeEnabled must be a boolean")
			}
		}
		if v, ok := payload["defaultTimeRange"]; ok {
			if _, ok := v.(string); !ok {
				return signozerrors.NewInputValidationError("invalid_payload", "defaultTimeRange must be a string")
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported dashboard resource %q", resource)
	}
	return nil
}

func alertsTemplate(resource string) (map[string]any, error) {
	switch resource {
	case "rule":
		return map[string]any{
			"alert":         "high-error-rate",
			"alertType":     "TRACES_BASED_ALERT",
			"description":   "sample alert rule from signozctl",
			"ruleType":      "threshold_rule",
			"evalWindow":    "5m",
			"frequency":     "1m",
			"schemaVersion": "v1",
		}, nil
	case "channel":
		return map[string]any{
			"name": "email-channel",
			"email_configs": []any{
				map[string]any{
					"to": "alerts@example.com",
				},
			},
		}, nil
	case "route-policy":
		return map[string]any{
			"name":        "route-by-severity",
			"description": "route critical alerts",
			"expression":  `severity == "critical"`,
			"kind":        "policy",
			"channels":    []string{"email-channel"},
			"tags":        []string{"auto-generated"},
		}, nil
	case "downtime":
		return map[string]any{
			"name":        "maintenance-window",
			"description": "planned maintenance",
			"schedule": map[string]any{
				"timezone":  "UTC",
				"startTime": "2026-02-21T02:00:00Z",
				"endTime":   "2026-02-21T03:00:00Z",
			},
			"alertIds": []string{},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported alerts resource %q", resource)
	}
}

func alertsSchema(resource string) (map[string]any, error) {
	switch resource {
	case "rule":
		return map[string]any{
			"resource": resource,
			"required": []string{"alert", "alertType"},
			"optional": []string{"description", "ruleType", "evalWindow", "frequency", "schemaVersion", "condition", "labels", "preferredChannels"},
		}, nil
	case "channel":
		return map[string]any{
			"resource": resource,
			"required": []string{"name"},
			"optional": []string{"email_configs", "slack_configs", "webhook_configs", "pagerduty_configs"},
		}, nil
	case "route-policy":
		return map[string]any{
			"resource": resource,
			"required": []string{"name", "expression", "kind", "channels"},
			"optional": []string{"description", "tags"},
		}, nil
	case "downtime":
		return map[string]any{
			"resource": resource,
			"required": []string{"name", "schedule"},
			"optional": []string{"description", "alertIds"},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported alerts resource %q", resource)
	}
}

func validateAlertsPayload(resource string, payload map[string]any) error {
	switch resource {
	case "rule":
		if _, ok := payload["alert"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "alert is required and must be a string")
		}
		if _, ok := payload["alertType"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "alertType is required and must be a string")
		}
	case "channel":
		if _, ok := payload["name"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "name is required and must be a string")
		}
	case "route-policy":
		if _, ok := payload["name"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "name is required and must be a string")
		}
		if _, ok := payload["expression"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "expression is required and must be a string")
		}
		if _, ok := payload["kind"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "kind is required and must be a string")
		}
		if _, ok := payload["channels"].([]any); !ok {
			if _, ok := payload["channels"].([]string); !ok {
				return signozerrors.NewInputValidationError("invalid_payload", "channels is required and must be an array")
			}
		}
	case "downtime":
		if _, ok := payload["name"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "name is required and must be a string")
		}
		if _, ok := payload["schedule"].(map[string]any); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "schedule is required and must be an object")
		}
	default:
		return fmt.Errorf("unsupported alerts resource %q", resource)
	}
	return nil
}

func iamTemplate(resource string) (map[string]any, error) {
	switch resource {
	case "invite":
		return map[string]any{
			"name":            "Agent User",
			"email":           "user@example.com",
			"role":            "ADMIN",
			"frontendBaseUrl": "http://localhost:8080",
		}, nil
	case "role":
		return map[string]any{
			"name":        "custom-observer-role",
			"description": "example custom role",
		}, nil
	case "api-key":
		return map[string]any{
			"name":          "agent-key",
			"role":          "ADMIN",
			"expiresInDays": 30,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported iam resource %q", resource)
	}
}

func iamSchema(resource string) (map[string]any, error) {
	switch resource {
	case "invite":
		return map[string]any{"resource": resource, "required": []string{"name", "email", "role"}, "optional": []string{"frontendBaseUrl"}}, nil
	case "role":
		return map[string]any{"resource": resource, "required": []string{"name"}, "optional": []string{"description"}}, nil
	case "api-key":
		return map[string]any{"resource": resource, "required": []string{"name", "role"}, "optional": []string{"expiresInDays"}}, nil
	default:
		return nil, fmt.Errorf("unsupported iam resource %q", resource)
	}
}

func validateIAMPayload(resource string, payload map[string]any) error {
	switch resource {
	case "invite":
		if _, ok := payload["name"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "name is required and must be a string")
		}
		if _, ok := payload["email"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "email is required and must be a string")
		}
		if _, ok := payload["role"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "role is required and must be a string")
		}
	case "role":
		if _, ok := payload["name"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "name is required and must be a string")
		}
	case "api-key":
		if _, ok := payload["name"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "name is required and must be a string")
		}
		if _, ok := payload["role"].(string); !ok {
			return signozerrors.NewInputValidationError("invalid_payload", "role is required and must be a string")
		}
	default:
		return fmt.Errorf("unsupported iam resource %q", resource)
	}
	return nil
}

func loadPayloadFile(filePath string) (map[string]any, error) {
	if filePath == "" {
		return nil, signozerrors.NewMissingRequiredFlagError("--file")
	}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON payload: %w", err)
	}
	return payload, nil
}
