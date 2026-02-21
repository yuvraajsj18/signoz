package commands

import (
	"fmt"

	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
)

func newDashboardPublicCreateCommand(flags *globalFlags) *cobra.Command {
	return newDashboardPublicUpsertCommand(flags, "public-create <dashboard-id>", "Create public sharing config for a dashboard", "post")
}

func newDashboardPublicUpdateCommand(flags *globalFlags) *cobra.Command {
	return newDashboardPublicUpsertCommand(flags, "public-update <dashboard-id>", "Update public sharing config for a dashboard", "put")
}

func newDashboardPublicUpsertCommand(flags *globalFlags, use, short, method string) *cobra.Command {
	var profile string
	var filePath string
	var enabled bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := loadOptionalJSONPayload(filePath)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("enabled") {
				payload["timeRangeEnabled"] = enabled
				if enabled {
					if _, ok := payload["defaultTimeRange"]; !ok {
						payload["defaultTimeRange"] = "5m"
					}
				}
			}
			c, err := profileClient(flags, profile)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/api/v1/dashboards/%s/public", args[0])
			var resp map[string]any
			switch method {
			case "post":
				if err := c.PostJSON(cmd.Context(), path, payload, &resp); err != nil {
					return err
				}
			case "put":
				if err := c.PutRawJSON(cmd.Context(), path, mustMarshalJSON(payload), &resp); err != nil {
					return err
				}
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name override")
	cmd.Flags().StringVar(&filePath, "file", "", "optional JSON payload file")
	cmd.Flags().BoolVar(&enabled, "enabled", false, "set public sharing enable state")
	return cmd
}
