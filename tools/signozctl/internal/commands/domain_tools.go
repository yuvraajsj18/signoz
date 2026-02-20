package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

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
		return map[string]any{"isEnabled": true}, nil
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
		return map[string]any{"resource": resource, "required": []string{}, "optional": []string{"isEnabled", "title", "description"}}, nil
	default:
		return nil, fmt.Errorf("unsupported dashboard resource %q", resource)
	}
}

func validateDashboardPayload(resource string, payload map[string]any) error {
	switch resource {
	case "create", "update":
		if _, ok := payload["title"].(string); !ok {
			return errors.New("title is required and must be a string")
		}
	case "public-create", "public-update":
		return nil
	default:
		return fmt.Errorf("unsupported dashboard resource %q", resource)
	}
	return nil
}

func alertsTemplate(resource string) (map[string]any, error) {
	switch resource {
	case "rule":
		return map[string]any{"alert": "cpu high", "name": "rule-1"}, nil
	case "channel":
		return map[string]any{"name": "email-channel", "type": "email"}, nil
	case "route-policy":
		return map[string]any{"name": "default-route"}, nil
	case "downtime":
		return map[string]any{"name": "maintenance-window"}, nil
	default:
		return nil, fmt.Errorf("unsupported alerts resource %q", resource)
	}
}

func alertsSchema(resource string) (map[string]any, error) {
	switch resource {
	case "rule", "channel", "route-policy", "downtime":
		return map[string]any{"resource": resource, "required": []string{"name"}, "optional": []string{"description", "labels"}}, nil
	default:
		return nil, fmt.Errorf("unsupported alerts resource %q", resource)
	}
}

func validateAlertsPayload(resource string, payload map[string]any) error {
	switch resource {
	case "rule", "channel", "route-policy", "downtime":
		if _, ok := payload["name"].(string); !ok {
			return errors.New("name is required and must be a string")
		}
	default:
		return fmt.Errorf("unsupported alerts resource %q", resource)
	}
	return nil
}

func iamTemplate(resource string) (map[string]any, error) {
	switch resource {
	case "invite":
		return map[string]any{"email": "user@example.com", "role": "VIEWER"}, nil
	case "role":
		return map[string]any{"name": "custom-role"}, nil
	case "api-key":
		return map[string]any{"name": "agent-key", "role": "ADMIN"}, nil
	default:
		return nil, fmt.Errorf("unsupported iam resource %q", resource)
	}
}

func iamSchema(resource string) (map[string]any, error) {
	switch resource {
	case "invite":
		return map[string]any{"resource": resource, "required": []string{"email"}, "optional": []string{"role"}}, nil
	case "role", "api-key":
		return map[string]any{"resource": resource, "required": []string{"name"}, "optional": []string{"role"}}, nil
	default:
		return nil, fmt.Errorf("unsupported iam resource %q", resource)
	}
}

func validateIAMPayload(resource string, payload map[string]any) error {
	switch resource {
	case "invite":
		if _, ok := payload["email"].(string); !ok {
			return errors.New("email is required and must be a string")
		}
	case "role", "api-key":
		if _, ok := payload["name"].(string); !ok {
			return errors.New("name is required and must be a string")
		}
	default:
		return fmt.Errorf("unsupported iam resource %q", resource)
	}
	return nil
}

func loadPayloadFile(filePath string) (map[string]any, error) {
	if filePath == "" {
		return nil, errors.New("missing required flag: --file")
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
