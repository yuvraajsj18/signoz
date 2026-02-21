package commands

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
)

func newLogsTailCommand(flags *globalFlags) *cobra.Command {
	var filePath string
	var profile string
	var intervalRaw string
	var iterations int
	var last string

	cmd := &cobra.Command{
		Use:   "logs-tail",
		Short: "Live tail logs by polling query API",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filePath == "" {
				return signozerrors.NewMissingRequiredFlagError("--file")
			}
			raw, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			interval, err := time.ParseDuration(intervalRaw)
			if err != nil || interval <= 0 {
				return signozerrors.NewInputValidationError("invalid_interval", "interval must be a positive duration, e.g. 2s")
			}

			c, err := profileClient(flags, profile)
			if err != nil {
				return err
			}

			seen := map[string]struct{}{}
			emitted := make([]any, 0)
			i := 0
			for {
				if iterations > 0 && i >= iterations {
					break
				}
				i++

				payloadRaw := raw
				if last != "" {
					d, err := parseRelativeDuration(last)
					if err != nil {
						return signozerrors.NewInputValidationError("invalid_relative_duration", "invalid --last value: "+err.Error())
					}
					end := time.Now().UnixMilli()
					start := end - d.Milliseconds()
					payloadRaw, err = applyTimeRange(payloadRaw, start, end)
					if err != nil {
						return signozerrors.NewInputValidationError("invalid_payload", "failed to apply --last time range: "+err.Error())
					}
				}

				var resp map[string]any
				if err := c.PostRawJSON(cmd.Context(), "/api/v5/query_range", payloadRaw, &resp); err != nil {
					return err
				}
				newRows := extractRowsFromQueryResponse(resp)
				for _, row := range newRows {
					key := rowDedupKey(row)
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					emitted = append(emitted, row)
				}

				if iterations == 0 {
					select {
					case <-cmd.Context().Done():
						return output.Render(cmd.OutOrStdout(), flags.Output, emitted)
					case <-time.After(interval):
					}
				} else if i < iterations {
					time.Sleep(interval)
				}
			}

			return output.Render(cmd.OutOrStdout(), flags.Output, emitted)
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "JSON payload file (logs query request)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name override")
	cmd.Flags().StringVar(&intervalRaw, "interval", "2s", "poll interval (e.g. 2s)")
	cmd.Flags().IntVar(&iterations, "iterations", 0, "number of poll iterations (0 = run until interrupted)")
	cmd.Flags().StringVar(&last, "last", "5m", "relative time range ending now (e.g. 5m, 1h)")
	return cmd
}

func extractRowsFromQueryResponse(resp map[string]any) []any {
	data, _ := resp["data"].(map[string]any)
	innerData, _ := data["data"].(map[string]any)
	results, _ := innerData["results"].([]any)
	rows := make([]any, 0)
	for _, result := range results {
		rmap, _ := result.(map[string]any)
		list, _ := rmap["rows"].([]any)
		rows = append(rows, list...)
	}
	return rows
}

func rowDedupKey(row any) string {
	b, err := json.Marshal(row)
	if err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return strings.TrimSpace(string(b))
}
