package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
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
	var format string
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Show payload schema idea for a signal",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sig := strings.ToLower(strings.TrimSpace(signal))
			var (
				schema map[string]any
				err    error
			)
			switch strings.ToLower(strings.TrimSpace(format)) {
			case "hints", "":
				schema, err = buildQuerySchema(sig)
			case "json-schema":
				schema, err = buildQueryJSONSchema(sig)
			default:
				return errInvalidOption("format", format, "hints|json-schema")
			}
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, schema)
		},
	}
	cmd.Flags().StringVar(&signal, "signal", "traces", "signal to show schema for: traces|logs|metrics")
	cmd.Flags().StringVar(&format, "format", "hints", "schema format: hints|json-schema")
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

func newQueryFieldsCommand(flags *globalFlags) *cobra.Command {
	var signal string
	cmd := &cobra.Command{
		Use:   "fields",
		Short: "Show known field catalog for a signal",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sig := strings.ToLower(strings.TrimSpace(signal))
			fields, err := queryFieldCatalog(sig)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"signal": sig,
				"fields": fields,
			})
		},
	}
	cmd.Flags().StringVar(&signal, "signal", "traces", "signal: traces|logs|metrics")
	return cmd
}

func newQueryOperatorsCommand(flags *globalFlags) *cobra.Command {
	var signal string
	var field string
	cmd := &cobra.Command{
		Use:   "operators",
		Short: "Show supported operators for a signal/field",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sig := strings.ToLower(strings.TrimSpace(signal))
			ops, err := queryOperators(sig, strings.TrimSpace(field))
			if err != nil {
				return err
			}
			resp := map[string]any{
				"signal":    sig,
				"operators": ops,
			}
			if strings.TrimSpace(field) != "" {
				resp["field"] = strings.TrimSpace(field)
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&signal, "signal", "traces", "signal: traces|logs|metrics")
	cmd.Flags().StringVar(&field, "field", "", "field name")
	return cmd
}

func newQueryLintCommand(flags *globalFlags) *cobra.Command {
	var signal string
	var expr string
	var filePath string
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint query filter expressions with actionable suggestions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sig := strings.ToLower(strings.TrimSpace(signal))
			exprs := []string{}
			if strings.TrimSpace(expr) != "" {
				exprs = append(exprs, strings.TrimSpace(expr))
			}
			if strings.TrimSpace(filePath) != "" {
				payload, err := loadPayloadFile(filePath)
				if err != nil {
					return err
				}
				exprs = append(exprs, extractQueryExpressions(payload)...)
				if sig == "traces" && payloadContainsKey(payload, "filters") && !payloadContainsKey(payload, "filter") {
					return signozerrors.NewLocalError(
						signozerrors.ClassInputValidation,
						400,
						"invalid_filter_shape",
						"trace query payload uses unsupported key `filters`; expected `filter`",
						"use `filter.expression` in trace query specs (or run `signozctl query template --signal traces`)",
					)
				}
			}
			if len(exprs) == 0 {
				return signozerrors.NewInputValidationError("missing_required_flag", "provide --expr or --file")
			}
			for _, e := range exprs {
				if issue, hint := lintQueryExpression(sig, e); issue != "" {
					return signozerrors.NewLocalError(signozerrors.ClassInputValidation, 400, "invalid_filter_expression", issue, hint)
				}
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"valid":       true,
				"signal":      sig,
				"expressions": exprs,
			})
		},
	}
	cmd.Flags().StringVar(&signal, "signal", "traces", "signal: traces|logs|metrics")
	cmd.Flags().StringVar(&expr, "expr", "", "filter/search expression to lint")
	cmd.Flags().StringVar(&filePath, "file", "", "JSON payload file to inspect filter expressions")
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
		return nil, errInvalidOption("signal", signal, "traces|logs|metrics")
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
		return nil, errInvalidOption("signal", signal, "traces|logs|metrics")
	}
}

func buildQueryJSONSchema(signal string) (map[string]any, error) {
	base := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object",
		"required": []string{
			"schemaVersion", "requestType", "compositeQuery",
		},
		"properties": map[string]any{
			"schemaVersion": map[string]any{"type": "string", "const": "v1"},
			"start":         map[string]any{"type": "number"},
			"end":           map[string]any{"type": "number"},
			"requestType":   map[string]any{"type": "string"},
			"compositeQuery": map[string]any{
				"type":     "object",
				"required": []string{"queries"},
				"properties": map[string]any{
					"queries": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "object"},
					},
				},
			},
		},
	}
	switch signal {
	case "traces":
		base["title"] = "SigNoz Query Schema (traces)"
		base["properties"].(map[string]any)["requestType"] = map[string]any{"type": "string", "enum": []string{"trace"}}
	case "logs":
		base["title"] = "SigNoz Query Schema (logs)"
		base["properties"].(map[string]any)["requestType"] = map[string]any{"type": "string", "enum": []string{"raw"}}
	case "metrics":
		base["title"] = "SigNoz Query Schema (metrics)"
		base["properties"].(map[string]any)["requestType"] = map[string]any{"type": "string", "enum": []string{"time_series"}}
	default:
		return nil, errInvalidOption("signal", signal, "traces|logs|metrics")
	}
	return base, nil
}

func queryFieldCatalog(signal string) ([]map[string]any, error) {
	switch signal {
	case "traces":
		return []map[string]any{
			{"name": "service.name", "type": "string", "kind": "resource"},
			{"name": "name", "type": "string", "kind": "span"},
			{"name": "hasError", "type": "bool", "kind": "trace"},
			{"name": "response_status_code", "type": "string", "kind": "trace"},
			{"name": "trace_duration", "type": "duration", "kind": "trace"},
			{"name": "span_count", "type": "number", "kind": "trace"},
		}, nil
	case "logs":
		return []map[string]any{
			{"name": "service.name", "type": "string", "kind": "resource"},
			{"name": "severity_text", "type": "string", "kind": "attribute"},
			{"name": "body", "type": "string", "kind": "body"},
			{"name": "trace_id", "type": "string", "kind": "attribute"},
			{"name": "timestamp", "type": "time", "kind": "core"},
		}, nil
	case "metrics":
		return []map[string]any{
			{"name": "metricName", "type": "string", "kind": "metric"},
			{"name": "service.name", "type": "string", "kind": "resource"},
			{"name": "deployment.environment", "type": "string", "kind": "resource"},
			{"name": "host.name", "type": "string", "kind": "resource"},
		}, nil
	default:
		return nil, errInvalidOption("signal", signal, "traces|logs|metrics")
	}
}

func queryOperators(signal, field string) ([]string, error) {
	opsByType := map[string][]string{
		"string":   {"=", "!=", "IN", "NOT IN", "CONTAINS", "NOT CONTAINS", "REGEXP"},
		"bool":     {"=", "!="},
		"number":   {"=", "!=", ">", ">=", "<", "<=", "BETWEEN", "IN", "NOT IN"},
		"duration": {"=", "!=", ">", ">=", "<", "<=", "BETWEEN"},
		"time":     {"BETWEEN", ">", ">=", "<", "<="},
	}
	fields, err := queryFieldCatalog(signal)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(field) == "" {
		merged := []string{}
		seen := map[string]struct{}{}
		for _, f := range fields {
			t, _ := f["type"].(string)
			for _, op := range opsByType[t] {
				if _, ok := seen[op]; ok {
					continue
				}
				seen[op] = struct{}{}
				merged = append(merged, op)
			}
		}
		slices.Sort(merged)
		return merged, nil
	}
	for _, f := range fields {
		if strings.EqualFold(fmt.Sprint(f["name"]), field) {
			return opsByType[fmt.Sprint(f["type"])], nil
		}
	}
	return nil, signozerrors.NewInputValidationError("unknown_field", fmt.Sprintf("field %q is not in known %s catalog", field, signal))
}

func lintQueryExpression(signal, expr string) (string, string) {
	ex := strings.TrimSpace(expr)
	if ex == "" {
		return "expression is empty", "provide a non-empty filter expression"
	}
	switch signal {
	case "traces":
		re := regexp.MustCompile(`(?i)\bstatus\b`)
		if re.MatchString(ex) {
			return "unknown field 'status' in trace filter expression", "try `hasError = true` for error traces or `response_status_code != '200'` for failed HTTP status"
		}
	}
	return "", ""
}

func extractQueryExpressions(payload map[string]any) []string {
	out := []string{}
	composite, ok := payload["compositeQuery"].(map[string]any)
	if !ok {
		return out
	}
	queries, ok := composite["queries"].([]any)
	if !ok {
		return out
	}
	for _, q := range queries {
		qm, ok := q.(map[string]any)
		if !ok {
			continue
		}
		spec, ok := qm["spec"].(map[string]any)
		if !ok {
			continue
		}
		filter, ok := spec["filter"].(map[string]any)
		if !ok {
			continue
		}
		if expr, ok := filter["expression"].(string); ok && strings.TrimSpace(expr) != "" {
			out = append(out, strings.TrimSpace(expr))
		}
	}
	return out
}

func payloadContainsKey(v any, target string) bool {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			if k == target {
				return true
			}
			if payloadContainsKey(child, target) {
				return true
			}
		}
	case []any:
		for _, child := range t {
			if payloadContainsKey(child, target) {
				return true
			}
		}
	}
	return false
}

func decodePayload(raw []byte) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, errInvalidJSONPayload(err)
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
