package commands

import (
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

	var templateSourcePage string
	var templateServiceName string
	var templateName string
	var templateExtraData string
	templateCmd := &cobra.Command{
		Use:   "template",
		Short: "Generate saved-view payload template",
		RunE: func(cmd *cobra.Command, _ []string) error {
			name := strings.TrimSpace(templateName)
			if name == "" {
				name = "signozctl saved view"
			}
			payload, err := buildSavedViewPayloadFromFlags(name, templateSourcePage, templateServiceName, templateExtraData)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, payload)
		},
	}
	templateCmd.Flags().StringVar(&templateSourcePage, "source-page", "traces", "source page: traces|logs|metrics")
	templateCmd.Flags().StringVar(&templateServiceName, "service-name", "", "service.name filter convenience (traces/logs)")
	templateCmd.Flags().StringVar(&templateName, "name", "signozctl saved view", "saved view name")
	templateCmd.Flags().StringVar(&templateExtraData, "extra-data", "{}", "extraData JSON string")
	cmd.AddCommand(templateCmd)

	var schemaSourcePage string
	schemaCmd := &cobra.Command{
		Use:   "schema",
		Short: "Show saved-view payload schema idea",
		RunE: func(cmd *cobra.Command, _ []string) error {
			schema, err := buildSavedViewSchema(schemaSourcePage)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, schema)
		},
	}
	schemaCmd.Flags().StringVar(&schemaSourcePage, "source-page", "traces", "source page: traces|logs|metrics")
	cmd.AddCommand(schemaCmd)

	var validateFile string
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate saved-view payload file locally",
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := loadPayloadFile(validateFile)
			if err != nil {
				return err
			}
			if err := validateSavedViewPayload(payload); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"valid": true,
				"file":  validateFile,
			})
		},
	}
	validateCmd.Flags().StringVar(&validateFile, "file", "", "JSON payload file")
	cmd.AddCommand(validateCmd)

	var listProfile string
	var sourcePage string
	var category string
	var name string
	var summary bool
	var full bool
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
			if summary && !full {
				return output.Render(cmd.OutOrStdout(), flags.Output, summarizeListResponse(resp))
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	listCmd.Flags().StringVar(&listProfile, "profile", "", "profile name override")
	listCmd.Flags().StringVar(&sourcePage, "source-page", "", "filter by source page: traces|logs|metrics")
	listCmd.Flags().StringVar(&category, "category", "", "filter by category")
	listCmd.Flags().StringVar(&name, "name", "", "filter by view name")
	listCmd.Flags().BoolVar(&summary, "summary", false, "return concise summary fields")
	listCmd.Flags().BoolVar(&full, "full", false, "return full raw payload")
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
	var updateProfile string
	var updateFile string
	var showNormalizedDiff bool
	updateCmd := &cobra.Command{
		Use:   "update <view-id>",
		Short: "Update saved view by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(updateFile) == "" {
				return signozerrors.NewMissingRequiredFlagError("--file")
			}
			payload, raw, err := parsePayloadFile(updateFile)
			if err != nil {
				return err
			}
			c, err := profileClient(flags, updateProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.PutRawJSON(cmd.Context(), "/api/v1/explorer/views/"+args[0], raw, &resp); err != nil {
				return err
			}
			if showNormalizedDiff {
				if normalized, ok := resp["data"].(map[string]any); ok {
					resp["normalizedDiff"] = normalizedTopLevelDiff(payload, normalized)
				}
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	updateCmd.Flags().StringVar(&updateProfile, "profile", "", "profile name override")
	updateCmd.Flags().StringVar(&updateFile, "file", "", "saved-view JSON payload file")
	updateCmd.Flags().BoolVar(&showNormalizedDiff, "show-normalized-diff", false, "show top-level diff between sent payload and server-normalized payload")
	cmd.AddCommand(updateCmd)

	var applyProfile string
	var applyFile string
	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Create-or-update saved view by name+sourcePage",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(applyFile) == "" {
				return signozerrors.NewMissingRequiredFlagError("--file")
			}
			payload, raw, err := parsePayloadFile(applyFile)
			if err != nil {
				return err
			}
			targetName, _ := payload["name"].(string)
			targetSource, _ := payload["sourcePage"].(string)
			if strings.TrimSpace(targetName) == "" || strings.TrimSpace(targetSource) == "" {
				return signozerrors.NewInputValidationError("invalid_payload", "name and sourcePage are required for view apply")
			}

			c, err := profileClient(flags, applyProfile)
			if err != nil {
				return err
			}
			var listResp map[string]any
			if err := c.GetJSON(cmd.Context(), "/api/v1/explorer/views", &listResp); err != nil {
				return err
			}
			items, _ := listResp["data"].([]any)
			for _, item := range items {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				id, _ := m["id"].(string)
				name, _ := m["name"].(string)
				source, _ := m["sourcePage"].(string)
				if id != "" && name == targetName && source == targetSource {
					var updateResp map[string]any
					if err := c.PutRawJSON(cmd.Context(), "/api/v1/explorer/views/"+id, raw, &updateResp); err != nil {
						return err
					}
					return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
						"mode": "updated", "id": id, "response": updateResp,
					})
				}
			}
			var createResp map[string]any
			if err := c.PostRawJSON(cmd.Context(), "/api/v1/explorer/views", raw, &createResp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"mode": "created", "response": createResp,
			})
		},
	}
	applyCmd.Flags().StringVar(&applyProfile, "profile", "", "profile name override")
	applyCmd.Flags().StringVar(&applyFile, "file", "", "saved-view JSON payload file")
	cmd.AddCommand(applyCmd)

	cmd.AddCommand(newProfileDeleteByIDCommand(flags, "delete <view-id>", "Delete saved view by ID", "/api/v1/explorer/views/%s"))

	return cmd
}

func buildSavedViewSchema(sourcePage string) (map[string]any, error) {
	sourcePage = strings.ToLower(strings.TrimSpace(sourcePage))
	if sourcePage != "traces" && sourcePage != "logs" && sourcePage != "metrics" {
		return nil, errInvalidOption("source-page", sourcePage, "traces|logs|metrics")
	}
	return map[string]any{
		"resource":   "saved-view",
		"sourcePage": sourcePage,
		"required":   []string{"name", "sourcePage", "compositeQuery"},
		"optional":   []string{"category", "tags", "extraData"},
		"nestedHints": []string{
			"compositeQuery.queryType",
			"compositeQuery.panelType",
			"compositeQuery.unit",
			"compositeQuery.queries[]",
			"compositeQuery.queries[].type",
			"compositeQuery.queries[].spec.signal",
			"compositeQuery.queries[].spec.filter.expression",
			"compositeQuery.builderQueries.<name>.filters.items[] (legacy)",
		},
	}, nil
}

func validateSavedViewPayload(payload map[string]any) error {
	if _, ok := payload["name"].(string); !ok {
		return signozerrors.NewInputValidationError("invalid_payload", "name is required and must be a string")
	}
	sp, ok := payload["sourcePage"].(string)
	if !ok {
		return signozerrors.NewInputValidationError("invalid_payload", "sourcePage is required and must be a string")
	}
	if _, err := buildSavedViewSchema(sp); err != nil {
		return err
	}
	composite, ok := payload["compositeQuery"].(map[string]any)
	if !ok {
		return signozerrors.NewInputValidationError("invalid_payload", "compositeQuery is required and must be an object")
	}
	if _, ok := composite["queryType"].(string); !ok {
		return signozerrors.NewInputValidationError("invalid_payload", "compositeQuery.queryType is required and must be a string")
	}
	if _, ok := composite["builderQueries"].(map[string]any); ok {
		return nil
	}
	if _, ok := composite["queries"].([]any); ok {
		return nil
	}
	return signozerrors.NewInputValidationError("invalid_payload", "compositeQuery must contain either builderQueries (legacy) or queries[]")
}

func buildSavedViewPayloadFromFlags(name, sourcePage, serviceName, extraData string) (map[string]any, error) {
	sourcePage = strings.ToLower(strings.TrimSpace(sourcePage))
	if sourcePage != "traces" && sourcePage != "logs" && sourcePage != "metrics" {
		return nil, errInvalidOption("source-page", sourcePage, "traces|logs|metrics")
	}
	if strings.TrimSpace(extraData) == "" {
		extraData = "{}"
	}
	trimmedExtra := strings.TrimSpace(extraData)
	if trimmedExtra == "{}" {
		extraData = defaultExtraDataForSourcePage(sourcePage)
	}
	// Validate extraData is a JSON object/string payload.
	if !jsonLike(extraData) {
		return nil, signozerrors.NewInputValidationError("invalid_extra_data", "extra-data must be valid JSON text (for example: '{}')")
	}

	filterExpr := ""
	if strings.TrimSpace(serviceName) != "" {
		filterExpr = "service.name = '" + strings.TrimSpace(serviceName) + "'"
	}

	spec := map[string]any{
		"name":         "A",
		"signal":       sourcePage,
		"source":       "",
		"stepInterval": 0,
		"filter":       map[string]any{"expression": filterExpr},
		"having":       map[string]any{"expression": ""},
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
			"queries": []any{
				map[string]any{
					"type": "builder_query",
					"spec": spec,
				},
			},
		},
	}, nil
}

func jsonLike(raw string) bool {
	trim := strings.TrimSpace(raw)
	return strings.HasPrefix(trim, "{") && strings.HasSuffix(trim, "}")
}

func defaultExtraDataForSourcePage(sourcePage string) string {
	switch sourcePage {
	case "traces":
		return `{"version":1,"selectColumns":[{"name":"service.name","signal":"traces","fieldContext":"resource","fieldDataType":"string"},{"name":"name","signal":"traces","fieldContext":"span","fieldDataType":"string"},{"name":"duration_nano","signal":"traces","fieldContext":"span","fieldDataType":""},{"name":"http_method","signal":"traces","fieldContext":"span","fieldDataType":""},{"name":"response_status_code","signal":"traces","fieldContext":"span","fieldDataType":""}]}`
	case "logs":
		return `{"version":1,"selectColumns":[{"name":"timestamp","signal":"logs","fieldContext":"span","fieldDataType":"timestamp"},{"name":"severity_text","signal":"logs","fieldContext":"span","fieldDataType":"string"},{"name":"body","signal":"logs","fieldContext":"span","fieldDataType":"string"},{"name":"service.name","signal":"logs","fieldContext":"resource","fieldDataType":"string"}]}`
	default:
		return "{}"
	}
}
