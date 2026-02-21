package commands

import (
	"context"
	"sort"

	"github.com/SigNoz/signoz/tools/signozctl/internal/client"
	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
)

func newTraceRootCommand(flags *globalFlags) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "trace-root <trace-id>",
		Short: "Resolve root span id for a trace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := profileClient(flags, profile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.PostJSON(cmd.Context(), "/api/v2/traces/waterfall/"+args[0], map[string]any{}, &resp); err != nil {
				return err
			}
			spanID := firstSpanID(resp)
			if spanID == "" {
				return signozerrors.NewInputValidationError("trace_root_not_found", "root span id not found for trace "+args[0])
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"traceId":    args[0],
				"rootSpanId": spanID,
			})
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name override")
	return cmd
}

func newTraceWaterfallCommand(flags *globalFlags) *cobra.Command {
	var profile string
	var filePath string
	var selectedSpanID string
	var expandSelected bool
	var uncollapseSpans []string
	var expandAll bool

	cmd := &cobra.Command{
		Use:   "trace-waterfall <trace-id>",
		Short: "Get span-level waterfall for trace ID (supports expansion flags; --file optional)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := loadOptionalJSONPayload(filePath)
			if err != nil {
				return err
			}

			if selectedSpanID != "" {
				payload["selectedSpanId"] = selectedSpanID
			}
			if cmd.Flags().Changed("expand-selected") {
				payload["isSelectedSpanIDUnCollapsed"] = expandSelected
			}
			if len(uncollapseSpans) > 0 {
				items := make([]any, 0, len(uncollapseSpans))
				for _, s := range uncollapseSpans {
					items = append(items, s)
				}
				payload["uncollapsedSpans"] = items
			}

			c, err := profileClient(flags, profile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if expandAll {
				resp, err = expandAllWaterfall(cmd.Context(), c, args[0], payload)
				if err != nil {
					return err
				}
			} else {
				if err := c.PostJSON(cmd.Context(), "/api/v2/traces/waterfall/"+args[0], payload, &resp); err != nil {
					return err
				}
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "optional JSON payload file")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name override")
	cmd.Flags().StringVar(&selectedSpanID, "selected-span-id", "", "selected span ID to focus/center waterfall")
	cmd.Flags().BoolVar(&expandSelected, "expand-selected", false, "expand selected span in waterfall")
	cmd.Flags().StringArrayVar(&uncollapseSpans, "uncollapse-span", nil, "parent span ID to keep uncollapsed (repeatable)")
	cmd.Flags().BoolVar(&expandAll, "expand-all", false, "attempt to expand all available spans for the trace")
	return cmd
}

func newTraceFlamegraphCommand(flags *globalFlags) *cobra.Command {
	var profile string
	var filePath string
	var selectedSpanID string

	cmd := &cobra.Command{
		Use:   "trace-flamegraph <trace-id>",
		Short: "Get flamegraph spans for trace ID (--file optional)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := loadOptionalJSONPayload(filePath)
			if err != nil {
				return err
			}
			if selectedSpanID != "" {
				payload["selectedSpanId"] = selectedSpanID
			}

			c, err := profileClient(flags, profile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.PostJSON(cmd.Context(), "/api/v2/traces/flamegraph/"+args[0], payload, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "optional JSON payload file")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name override")
	cmd.Flags().StringVar(&selectedSpanID, "selected-span-id", "", "selected span ID to focus/center flamegraph")
	return cmd
}

func expandAllWaterfall(ctx context.Context, c *client.Client, traceID string, payload map[string]any) (map[string]any, error) {
	const maxIter = 20
	known := map[string]struct{}{}
	for _, spanID := range stringSliceFromPayload(payload["uncollapsedSpans"]) {
		known[spanID] = struct{}{}
	}

	var lastResp map[string]any
	for i := 0; i < maxIter; i++ {
		resp := map[string]any{}
		if err := c.PostJSON(ctx, "/api/v2/traces/waterfall/"+traceID, payload, &resp); err != nil {
			return nil, err
		}
		lastResp = resp

		spans := extractSpans(resp)
		if _, ok := payload["selectedSpanId"]; !ok {
			if root := firstSpanID(resp); root != "" {
				payload["selectedSpanId"] = root
				payload["isSelectedSpanIDUnCollapsed"] = true
			}
		}

		added := false
		for _, span := range spans {
			spanID, _ := span["spanId"].(string)
			hasChildren, _ := span["hasChildren"].(bool)
			if hasChildren && spanID != "" {
				if _, exists := known[spanID]; !exists {
					known[spanID] = struct{}{}
					added = true
				}
			}
		}
		uncollapsed := make([]string, 0, len(known))
		for spanID := range known {
			uncollapsed = append(uncollapsed, spanID)
		}
		sort.Strings(uncollapsed)
		payload["uncollapsedSpans"] = uncollapsed

		if !added {
			return resp, nil
		}
	}
	return lastResp, nil
}

func firstSpanID(resp map[string]any) string {
	spans := extractSpans(resp)
	if len(spans) == 0 {
		return ""
	}
	id, _ := spans[0]["spanId"].(string)
	return id
}

func extractSpans(resp map[string]any) []map[string]any {
	raw, ok := resp["spans"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func stringSliceFromPayload(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
