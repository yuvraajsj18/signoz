package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
)

func newQueryTemplateCommand(flags *globalFlags) *cobra.Command {
	var signal string
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Generate runnable payload template for a signal",
		RunE: func(cmd *cobra.Command, _ []string) error {
			template, err := buildQueryTemplate(strings.ToLower(strings.TrimSpace(signal)))
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, template)
		},
	}
	cmd.Flags().StringVar(&signal, "signal", "traces", "signal to generate template for: traces|logs|metrics")
	return cmd
}

func newQuerySchemaCommand(flags *globalFlags) *cobra.Command {
	var signal string
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Show payload schema idea for a signal",
		RunE: func(cmd *cobra.Command, _ []string) error {
			schema, err := buildQuerySchema(strings.ToLower(strings.TrimSpace(signal)))
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, schema)
		},
	}
	cmd.Flags().StringVar(&signal, "signal", "traces", "signal to show schema for: traces|logs|metrics")
	return cmd
}

func newQueryValidateCommand(flags *globalFlags) *cobra.Command {
	var filePath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate query payload file locally",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filePath == "" {
				return signozerrors.NewMissingRequiredFlagError("--file")
			}
			raw, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			payload, err := decodePayload(raw)
			if err != nil {
				return err
			}
			if err := validateQueryPayload(payload); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"valid": true,
				"file":  filePath,
			})
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "JSON payload file")
	return cmd
}

func buildQueryTemplate(signal string) (map[string]any, error) {
	now := time.Now().UnixMilli()
	start := now - int64((5 * time.Minute).Milliseconds())

	switch signal {
	case "traces":
		return map[string]any{
			"schemaVersion": "v1",
			"start":         start,
			"end":           now,
			"requestType":   "trace",
			"compositeQuery": map[string]any{
				"queries": []any{
					map[string]any{
						"type": "builder_query",
						"spec": map[string]any{
							"name":         "A",
							"signal":       "traces",
							"stepInterval": 60,
							"aggregations": []any{map[string]any{"expression": "count()"}},
							"order":        []any{map[string]any{"key": map[string]any{"name": "timestamp"}, "direction": "desc"}},
							"limit":        20,
						},
					},
				},
			},
		}, nil
	case "logs":
		return map[string]any{
			"schemaVersion": "v1",
			"start":         start,
			"end":           now,
			"requestType":   "raw",
			"compositeQuery": map[string]any{
				"queries": []any{
					map[string]any{
						"type": "builder_query",
						"spec": map[string]any{
							"name":         "A",
							"signal":       "logs",
							"stepInterval": 60,
							"aggregations": []any{map[string]any{"expression": "count()"}},
							"limit":        50,
						},
					},
				},
			},
		}, nil
	case "metrics":
		return map[string]any{
			"schemaVersion": "v1",
			"start":         start,
			"end":           now,
			"requestType":   "time_series",
			"compositeQuery": map[string]any{
				"queries": []any{
					map[string]any{
						"type": "builder_query",
						"spec": map[string]any{
							"name":         "A",
							"signal":       "metrics",
							"stepInterval": 60,
							"aggregations": []any{
								map[string]any{
									"metricName":       "signoz_calls_total",
									"temporality":      "",
									"timeAggregation":  "sum",
									"spaceAggregation": "sum",
								},
							},
						},
					},
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported signal %q: use traces|logs|metrics", signal)
	}
}

func buildQuerySchema(signal string) (map[string]any, error) {
	switch signal {
	case "traces":
		return map[string]any{
			"signal":      "traces",
			"requestType": "trace",
			"required":    []string{"schemaVersion", "requestType", "compositeQuery.queries"},
			"optional":    []string{"start", "end", "variables", "formatOptions"},
			"specHints": map[string]any{
				"type":   "builder_query",
				"signal": "traces",
				"commonFields": []string{
					"name", "stepInterval", "aggregations", "filter", "order", "limit", "groupBy",
				},
			},
		}, nil
	case "logs":
		return map[string]any{
			"signal":      "logs",
			"requestType": "raw",
			"required":    []string{"schemaVersion", "requestType", "compositeQuery.queries"},
			"optional":    []string{"start", "end", "variables", "formatOptions"},
			"specHints": map[string]any{
				"type":   "builder_query",
				"signal": "logs",
				"commonFields": []string{
					"name", "stepInterval", "aggregations", "filter", "order", "limit", "groupBy", "cursor",
				},
			},
		}, nil
	case "metrics":
		return map[string]any{
			"signal":      "metrics",
			"requestType": "time_series",
			"required":    []string{"schemaVersion", "requestType", "compositeQuery.queries"},
			"optional":    []string{"start", "end", "variables", "formatOptions"},
			"specHints": map[string]any{
				"type":   "builder_query",
				"signal": "metrics",
				"commonFields": []string{
					"name", "stepInterval", "aggregations[].metricName", "aggregations[].timeAggregation", "aggregations[].spaceAggregation", "filter", "order", "groupBy",
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported signal %q: use traces|logs|metrics", signal)
	}
}

func decodePayload(raw []byte) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON payload: %w", err)
	}
	return payload, nil
}

func validateQueryPayload(payload map[string]any) error {
	if _, ok := payload["schemaVersion"].(string); !ok {
		return signozerrors.NewInputValidationError("invalid_payload", "schemaVersion is required and must be a string")
	}
	if _, ok := payload["requestType"].(string); !ok {
		return signozerrors.NewInputValidationError("invalid_payload", "requestType is required and must be a string")
	}
	composite, ok := payload["compositeQuery"].(map[string]any)
	if !ok {
		return signozerrors.NewInputValidationError("invalid_payload", "compositeQuery is required and must be an object")
	}
	queries, ok := composite["queries"].([]any)
	if !ok || len(queries) == 0 {
		return signozerrors.NewInputValidationError("invalid_payload", "compositeQuery.queries is required and must be a non-empty array")
	}

	startVal, hasStart := numberFromAny(payload["start"])
	endVal, hasEnd := numberFromAny(payload["end"])
	if hasStart && hasEnd && startVal >= endVal {
		return signozerrors.NewInputValidationError("invalid_time_range", "start must be before end")
	}
	return nil
}

func numberFromAny(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	default:
		return 0, false
	}
}
