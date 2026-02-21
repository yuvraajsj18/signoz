package commands

import (
	"fmt"
	"net/url"
	"strings"

	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
)

func newViewCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Saved views for traces/logs/metrics explorer",
	}

	var listProfile string
	var sourcePage string
	var category string
	var name string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List saved views",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := profileClient(flags, listProfile)
			if err != nil {
				return err
			}
			values := url.Values{}
			if strings.TrimSpace(sourcePage) != "" {
				values.Set("sourcePage", strings.TrimSpace(sourcePage))
			}
			if strings.TrimSpace(category) != "" {
				values.Set("category", strings.TrimSpace(category))
			}
			if strings.TrimSpace(name) != "" {
				values.Set("name", strings.TrimSpace(name))
			}
			path := "/api/v1/explorer/views"
			if values.Encode() != "" {
				path += "?" + values.Encode()
			}
			var resp map[string]any
			if err := c.GetJSON(cmd.Context(), path, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	listCmd.Flags().StringVar(&listProfile, "profile", "", "profile name override")
	listCmd.Flags().StringVar(&sourcePage, "source-page", "", "filter by source page: traces|logs|metrics")
	listCmd.Flags().StringVar(&category, "category", "", "filter by category")
	listCmd.Flags().StringVar(&name, "name", "", "filter by view name")
	cmd.AddCommand(listCmd)

	var createProfile string
	var createFile string
	var createName string
	var createSourcePage string
	var createServiceName string
	var extraData string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create saved view",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var payload map[string]any
			if strings.TrimSpace(createFile) != "" {
				p, err := loadPayloadFile(createFile)
				if err != nil {
					return err
				}
				payload = p
			} else {
				if strings.TrimSpace(createName) == "" {
					return signozerrors.NewMissingRequiredFlagError("--name")
				}
				if strings.TrimSpace(createSourcePage) == "" {
					return signozerrors.NewMissingRequiredFlagError("--source-page")
				}
				p, err := buildSavedViewPayloadFromFlags(createName, createSourcePage, createServiceName, extraData)
				if err != nil {
					return err
				}
				payload = p
			}
			c, err := profileClient(flags, createProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.PostJSON(cmd.Context(), "/api/v1/explorer/views", payload, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	createCmd.Flags().StringVar(&createProfile, "profile", "", "profile name override")
	createCmd.Flags().StringVar(&createFile, "file", "", "saved-view JSON payload file")
	createCmd.Flags().StringVar(&createName, "name", "", "saved view name (used when --file is not provided)")
	createCmd.Flags().StringVar(&createSourcePage, "source-page", "traces", "source page: traces|logs|metrics")
	createCmd.Flags().StringVar(&createServiceName, "service-name", "", "service.name filter convenience (traces/logs)")
	createCmd.Flags().StringVar(&extraData, "extra-data", "{}", "extraData JSON string")
	cmd.AddCommand(createCmd)

	cmd.AddCommand(newProfileGetByIDCommand(flags, "get <view-id>", "Get saved view by ID", "/api/v1/explorer/views/%s"))
	cmd.AddCommand(newProfilePutByIDFromFileCommand(flags, "update <view-id>", "Update saved view by ID", "/api/v1/explorer/views/%s"))
	cmd.AddCommand(newProfileDeleteByIDCommand(flags, "delete <view-id>", "Delete saved view by ID", "/api/v1/explorer/views/%s"))

	return cmd
}

func buildSavedViewPayloadFromFlags(name, sourcePage, serviceName, extraData string) (map[string]any, error) {
	sourcePage = strings.ToLower(strings.TrimSpace(sourcePage))
	if sourcePage != "traces" && sourcePage != "logs" && sourcePage != "metrics" {
		return nil, errInvalidOption("source-page", sourcePage, "traces|logs|metrics")
	}
	if strings.TrimSpace(extraData) == "" {
		extraData = "{}"
	}
	// Validate extraData is a JSON object/string payload.
	if !jsonLike(extraData) {
		return nil, signozerrors.NewInputValidationError("invalid_extra_data", "extra-data must be valid JSON text (for example: '{}')")
	}

	filterExpr := ""
	var filters map[string]any
	if strings.TrimSpace(serviceName) != "" {
		filterExpr = fmt.Sprintf("service.name = '%s'", strings.TrimSpace(serviceName))
		filters = map[string]any{
			"op": "AND",
			"items": []any{
				map[string]any{
					"key": map[string]any{
						"key":      "service.name",
						"dataType": "string",
						"type":     "resource",
						"isColumn": false,
						"isJSON":   false,
					},
					"op":    "=",
					"value": strings.TrimSpace(serviceName),
				},
			},
		}
	}

	queryData := map[string]any{
		"queryName":          "A",
		"dataSource":         sourcePage,
		"aggregateOperator":  "count",
		"aggregateAttribute": map[string]any{"key": "", "type": "", "dataType": ""},
		"timeAggregation":    "rate",
		"spaceAggregation":   "sum",
		"stepInterval":       60,
		"filter":             map[string]any{"expression": filterExpr},
		"filters":            filters,
		"groupBy":            []any{},
		"expression":         "A",
		"disabled":           false,
		"having":             []any{},
		"limit":              20,
		"orderBy":            []any{},
		"legend":             "",
		"functions":          []any{},
	}

	return map[string]any{
		"name":       strings.TrimSpace(name),
		"sourcePage": sourcePage,
		"category":   "",
		"tags":       []any{},
		"extraData":  extraData,
		"compositeQuery": map[string]any{
			"queryType": "builder",
			"panelType": "list",
			"unit":      "none",
			"builderQueries": map[string]any{
				"A": queryData,
			},
		},
	}, nil
}

func jsonLike(raw string) bool {
	trim := strings.TrimSpace(raw)
	return strings.HasPrefix(trim, "{") && strings.HasSuffix(trim, "}")
}
