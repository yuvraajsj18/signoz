package commands

import (
	"fmt"
	"strings"

	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
)

type dashboardRecipe struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Panels      []string `json:"panels"`
}

func newDashboardCookbookCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cookbook",
		Short: "Built-in dashboard recipes for common observability panels",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available dashboard recipes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"recipes": dashboardRecipes(),
			})
		},
	}
	cmd.AddCommand(listCmd)

	var services []string
	showCmd := &cobra.Command{
		Use:   "show <recipe-id>",
		Short: "Show runnable widget payload for a dashboard recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			widget, err := dashboardRecipeWidget(args[0], services)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"recipe":  args[0],
				"services": normalizeServices(services),
				"widget":  widget,
			})
		},
	}
	showCmd.Flags().StringArrayVar(&services, "service", nil, "service.name filter (repeatable)")
	cmd.AddCommand(showCmd)

	return cmd
}

func dashboardRecipes() []dashboardRecipe {
	return []dashboardRecipe{
		{
			ID:          "p95-latency-by-service",
			Name:        "P95 Latency By Service",
			Description: "Trace p95 latency grouped by service.name",
			Panels:      []string{"graph"},
		},
		{
			ID:          "top-failing-endpoints",
			Name:        "Top Failing Endpoints",
			Description: "Top endpoints by non-200 response status from traces",
			Panels:      []string{"table"},
		},
		{
			ID:          "error-log-rate",
			Name:        "Error Log Rate",
			Description: "Count of error logs grouped by service.name",
			Panels:      []string{"value", "graph"},
		},
	}
}

func dashboardRecipeWidget(recipeID string, services []string) (map[string]any, error) {
	svcs := normalizeServices(services)
	switch recipeID {
	case "p95-latency-by-service":
		return map[string]any{
			"id":         "service-latency-p95",
			"panelTypes": "graph",
			"title":      "p95 Latency by Service (ms)",
			"description": "p95 trace latency grouped by service.name",
			"query": map[string]any{
				"queryType": "builder",
				"builder": map[string]any{
					"queryData": []any{
						map[string]any{
							"queryName":   "A",
							"dataSource":  "traces",
							"expression":  "A",
							"stepInterval": 60,
							"aggregations": []any{
								map[string]any{"expression": "p95(duration_nano)", "alias": "p95_duration_ns"},
							},
							"groupBy": []any{
								map[string]any{
									"key":      "service.name",
									"type":     "resource",
									"dataType": "string",
									"isColumn": false,
									"isJSON":   false,
								},
							},
							"filter": map[string]any{
								"expression": serviceExpression(svcs),
							},
						},
					},
					"queryFormulas": []any{},
				},
			},
		}, nil
	case "top-failing-endpoints":
		return map[string]any{
			"id":         "top-failing-endpoints",
			"panelTypes": "table",
			"title":      "Top Failing Endpoints (trace count)",
			"description": "Top endpoint names by non-200 response status",
			"query": map[string]any{
				"queryType": "builder",
				"builder": map[string]any{
					"queryData": []any{
						map[string]any{
							"queryName":   "A",
							"dataSource":  "traces",
							"expression":  "A",
							"stepInterval": 60,
							"aggregations": []any{
								map[string]any{"expression": "count()"},
							},
							"groupBy": []any{
								map[string]any{"key": "name", "type": "tag", "dataType": "string", "isColumn": true, "isJSON": false},
								map[string]any{"key": "service.name", "type": "resource", "dataType": "string", "isColumn": false, "isJSON": false},
							},
							"filter": map[string]any{
								"expression": fmt.Sprintf("%s AND response_status_code != '200'", serviceExpression(svcs)),
							},
							"orderBy": []any{
								map[string]any{"columnName": "count()", "order": "desc"},
							},
							"limit": 10,
						},
					},
					"queryFormulas": []any{},
				},
			},
		}, nil
	case "error-log-rate":
		return map[string]any{
			"id":         "error-log-rate",
			"panelTypes": "value",
			"title":      "Error Log Rate",
			"description": "Error logs count grouped by service.name",
			"query": map[string]any{
				"queryType": "builder",
				"builder": map[string]any{
					"queryData": []any{
						map[string]any{
							"queryName":   "A",
							"dataSource":  "logs",
							"expression":  "A",
							"stepInterval": 60,
							"aggregations": []any{
								map[string]any{"expression": "count()"},
							},
							"filter": map[string]any{
								"expression": fmt.Sprintf("%s AND severity_text = 'error'", serviceExpression(svcs)),
							},
						},
					},
					"queryFormulas": []any{},
				},
			},
		}, nil
	default:
		return nil, signozerrors.NewInputValidationError("invalid_recipe_id", "unknown recipe-id: "+recipeID)
	}
}

func normalizeServices(services []string) []string {
	out := make([]string, 0, len(services))
	seen := map[string]struct{}{}
	for _, s := range services {
		v := strings.TrimSpace(s)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return []string{"catalog-node", "pricing-fastapi"}
	}
	return out
}

func serviceExpression(services []string) string {
	if len(services) == 1 {
		return fmt.Sprintf("service.name = '%s'", services[0])
	}
	quoted := make([]string, 0, len(services))
	for _, s := range services {
		quoted = append(quoted, fmt.Sprintf("'%s'", s))
	}
	return "service.name IN (" + strings.Join(quoted, ", ") + ")"
}
