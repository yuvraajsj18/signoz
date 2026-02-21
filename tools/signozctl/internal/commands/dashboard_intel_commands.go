package commands

import (
	"fmt"
	"sort"
	"strings"

	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
)

func newDashboardCapabilitiesCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "Show dashboard/panel query capabilities for agents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return output.Render(cmd.OutOrStdout(), flags.Output, dashboardCapabilities())
		},
	}
	return cmd
}

func newDashboardWidgetTemplateCommand(flags *globalFlags) *cobra.Command {
	var panel string
	var signal string
	cmd := &cobra.Command{
		Use:   "widget-template",
		Short: "Generate runnable widget template for a panel/signal pair",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tpl, err := dashboardWidgetTemplate(strings.ToLower(strings.TrimSpace(panel)), strings.ToLower(strings.TrimSpace(signal)))
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, tpl)
		},
	}
	cmd.Flags().StringVar(&panel, "panel", "graph", "panel type: graph|table|value")
	cmd.Flags().StringVar(&signal, "signal", "traces", "signal: traces|logs|metrics")
	return cmd
}

func newDashboardLintCommand(flags *globalFlags) *cobra.Command {
	var filePath string
	var resource string
	var explain bool
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint dashboard payload and return agent-guiding validation hints",
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := loadPayloadFile(filePath)
			if err != nil {
				return err
			}
			if err := validateDashboardPayload(strings.ToLower(strings.TrimSpace(resource)), payload); err != nil {
				return err
			}
			issues := lintDashboardPayload(payload)
			if len(issues) > 0 {
				msg := issues[0]
				hint := "run `signozctl dashboard capabilities` and `signozctl dashboard widget-template --panel <type> --signal <signal>` to generate a valid shape"
				if explain {
					hint = "For aggregation queries, order by can only reference group by keys, aggregation aliases/expressions, or aggregation indices. Common valid keys: 0, count(), name, service.name"
				}
				return signozerrors.NewLocalError(signozerrors.ClassInputValidation, 400, "invalid_dashboard_query", msg, hint)
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"valid":    true,
				"resource": resource,
				"file":     filePath,
				"issues":   []any{},
			})
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "create", "resource: create|update")
	cmd.Flags().StringVar(&filePath, "file", "", "dashboard JSON payload file")
	cmd.Flags().BoolVar(&explain, "explain", false, "include detailed remediation hints for agents")
	return cmd
}

func dashboardCapabilities() map[string]any {
	return map[string]any{
		"panelTypes": []string{"graph", "table", "value"},
		"signals":    []string{"traces", "logs", "metrics"},
		"queryShapeRules": map[string]any{
			"traces": []string{
				"use `filter.expression` (not `filters`) in v5 trace query specs",
				"for aggregation-style widgets include `aggregations[].expression` (e.g. `count()` or `p95(duration_nano)`)",
				"for table sorting, `orderBy[].columnName` must reference aggregation expression/alias, group-by keys, or index `0`",
			},
			"logs": []string{
				"use `filter.expression` for search predicates",
				"use `aggregations[].expression` for aggregate widgets",
			},
			"metrics": []string{
				"use metric aggregation blocks (`metricName`, `timeAggregation`, `spaceAggregation`)",
				"group by dimensions with `groupBy[]` telemetry keys",
			},
		},
		"widgetTemplateCommand": "signozctl dashboard widget-template --panel <graph|table|value> --signal <traces|logs|metrics>",
		"lintCommand":           "signozctl dashboard lint --file <dashboard.json> --explain",
		"examplesPath":          "tools/signozctl/examples",
	}
}

func dashboardWidgetTemplate(panel, signal string) (map[string]any, error) {
	if panel != "graph" && panel != "table" && panel != "value" {
		return nil, errInvalidOption("panel", panel, "graph|table|value")
	}
	switch signal {
	case "traces":
		return map[string]any{
			"id":         "widget-1",
			"panelTypes": panel,
			"title":      "Trace widget",
			"query": map[string]any{
				"queryType": "builder",
				"builder": map[string]any{
					"queryData": []any{
						map[string]any{
							"queryName":    "A",
							"dataSource":   "traces",
							"expression":   "A",
							"stepInterval": 60,
							"aggregations": []any{
								map[string]any{"expression": "count()"},
							},
							"filter": map[string]any{
								"expression": "service.name = 'catalog-node'",
							},
						},
					},
					"queryFormulas": []any{},
				},
			},
		}, nil
	case "logs":
		return map[string]any{
			"id":         "widget-1",
			"panelTypes": panel,
			"title":      "Log widget",
			"query": map[string]any{
				"queryType": "builder",
				"builder": map[string]any{
					"queryData": []any{
						map[string]any{
							"queryName":    "A",
							"dataSource":   "logs",
							"expression":   "A",
							"stepInterval": 60,
							"aggregations": []any{
								map[string]any{"expression": "count()"},
							},
							"filter": map[string]any{
								"expression": "service.name = 'catalog-node' AND severity_text = 'error'",
							},
						},
					},
					"queryFormulas": []any{},
				},
			},
		}, nil
	case "metrics":
		return map[string]any{
			"id":         "widget-1",
			"panelTypes": panel,
			"title":      "Metric widget",
			"query": map[string]any{
				"queryType": "builder",
				"builder": map[string]any{
					"queryData": []any{
						map[string]any{
							"queryName":    "A",
							"dataSource":   "metrics",
							"expression":   "A",
							"stepInterval": 60,
							"aggregations": []any{
								map[string]any{
									"metricName":       "signoz_calls_total",
									"timeAggregation":  "sum",
									"spaceAggregation": "sum",
									"temporality":      "",
								},
							},
							"filter": map[string]any{
								"expression": "service.name = 'catalog-node'",
							},
						},
					},
					"queryFormulas": []any{},
				},
			},
		}, nil
	default:
		return nil, errInvalidOption("signal", signal, "traces|logs|metrics")
	}
}

func lintDashboardPayload(payload map[string]any) []string {
	issues := make([]string, 0)
	widgets, _ := payload["widgets"].([]any)
	for wi, w := range widgets {
		wm, ok := w.(map[string]any)
		if !ok {
			continue
		}
		query, ok := wm["query"].(map[string]any)
		if !ok {
			continue
		}
		builder, ok := query["builder"].(map[string]any)
		if !ok {
			continue
		}
		queryData, ok := builder["queryData"].([]any)
		if !ok {
			continue
		}
		for qi, q := range queryData {
			qm, ok := q.(map[string]any)
			if !ok {
				continue
			}
			validOrderKeys := dashboardValidOrderByKeys(qm)
			orderBy, _ := qm["orderBy"].([]any)
			for _, ord := range orderBy {
				om, ok := ord.(map[string]any)
				if !ok {
					continue
				}
				col, _ := om["columnName"].(string)
				if strings.TrimSpace(col) == "" {
					continue
				}
				if _, ok := validOrderKeys[col]; !ok {
					keys := make([]string, 0, len(validOrderKeys))
					for k := range validOrderKeys {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					issues = append(issues, fmt.Sprintf("invalid order by key '%s' for widget[%d] queryData[%d]. Valid keys are: %s", col, wi, qi, strings.Join(keys, ", ")))
				}
			}
		}
	}
	return issues
}

func dashboardValidOrderByKeys(queryData map[string]any) map[string]struct{} {
	out := map[string]struct{}{
		"0": {},
	}
	if aggs, ok := queryData["aggregations"].([]any); ok {
		for _, a := range aggs {
			am, ok := a.(map[string]any)
			if !ok {
				continue
			}
			if expr, _ := am["expression"].(string); strings.TrimSpace(expr) != "" {
				out[expr] = struct{}{}
			}
			if alias, _ := am["alias"].(string); strings.TrimSpace(alias) != "" {
				out[alias] = struct{}{}
			}
		}
	}
	if groupBy, ok := queryData["groupBy"].([]any); ok {
		for _, g := range groupBy {
			gm, ok := g.(map[string]any)
			if !ok {
				continue
			}
			switch key := gm["key"].(type) {
			case string:
				if strings.TrimSpace(key) != "" {
					out[key] = struct{}{}
				}
			case map[string]any:
				if v, _ := key["key"].(string); strings.TrimSpace(v) != "" {
					out[v] = struct{}{}
				}
			}
		}
	}
	return out
}
