package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/SigNoz/signoz/tools/signozctl/internal/client"
	"github.com/SigNoz/signoz/tools/signozctl/internal/config"
	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
)

type globalFlags struct {
	ConfigPath string
	Profile    string
	Output     string
}

func NewRootCommand() *cobra.Command {
	flags := &globalFlags{}

	cmd := &cobra.Command{
		Use:   "signozctl",
		Short: "Agent-friendly CLI for SigNoz APIs",
	}

	defaultCfgPath, _ := config.DefaultPath()
	cmd.PersistentFlags().StringVar(&flags.ConfigPath, "config", defaultCfgPath, "path to signozctl config")
	cmd.PersistentFlags().StringVar(&flags.Profile, "profile", "", "profile name (defaults to active profile)")
	cmd.PersistentFlags().StringVar(&flags.Output, "output", "text", "output format: text|json")

	cmd.AddCommand(newAuthCommand(flags))
	cmd.AddCommand(newQueryCommand(flags))
	cmd.AddCommand(newViewCommand(flags))
	cmd.AddCommand(newDashboardCommand(flags))
	cmd.AddCommand(newAlertsCommand(flags))
	cmd.AddCommand(newIAMCommand(flags))
	cmd.AddCommand(newSystemCommand(flags))
	cmd.AddCommand(newDocsCommand(flags))

	return cmd
}

func newAuthCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication and profile management",
	}

	var host string
	var email string
	var password string
	var orgID string
	var loginProfile string
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Create a SigNoz session and store profile tokens",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if host == "" || email == "" || password == "" {
				return signozerrors.NewInputValidationError("missing_required_flags", "missing required flags: --host, --email, --password")
			}
			profileName := chooseProfile(flags.Profile, loginProfile)
			if profileName == "" {
				profileName = "default"
			}

			cfg, err := config.Load(flags.ConfigPath)
			if err != nil {
				return err
			}

			c := client.New(host, "")
			if orgID == "" {
				orgID, err = fetchOrgID(cmd.Context(), c, email, host)
				if err != nil {
					return err
				}
			}

			var loginResp struct {
				Data struct {
					AccessToken  string `json:"accessToken"`
					RefreshToken string `json:"refreshToken"`
				} `json:"data"`
			}
			if err := c.PostJSON(cmd.Context(), "/api/v2/sessions/email_password", map[string]string{
				"email":    email,
				"password": password,
				"orgId":    orgID,
			}, &loginResp); err != nil {
				return err
			}

			cfg.Profiles[profileName] = config.Profile{
				Host:         strings.TrimRight(host, "/"),
				Email:        email,
				OrgID:        orgID,
				AccessToken:  loginResp.Data.AccessToken,
				RefreshToken: loginResp.Data.RefreshToken,
			}
			cfg.ActiveProfile = profileName

			if err := config.Save(flags.ConfigPath, cfg); err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"profile":       profileName,
				"host":          strings.TrimRight(host, "/"),
				"email":         email,
				"orgId":         orgID,
				"authenticated": loginResp.Data.AccessToken != "",
			})
		},
	}
	loginCmd.Flags().StringVar(&host, "host", "", "SigNoz host, e.g. http://localhost:8080")
	loginCmd.Flags().StringVar(&email, "email", "", "account email")
	loginCmd.Flags().StringVar(&password, "password", "", "account password")
	loginCmd.Flags().StringVar(&orgID, "org-id", "", "organization id (optional; auto-resolved if omitted)")
	loginCmd.Flags().StringVar(&loginProfile, "profile", "", "profile name override")
	cmd.AddCommand(loginCmd)

	var statusProfile string
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show current profile auth status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(flags.ConfigPath)
			if err != nil {
				return err
			}
			profileName := chooseProfile(flags.Profile, statusProfile)
			prof, name, err := currentProfile(cfg, profileName)
			if err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"profile":       name,
				"host":          prof.Host,
				"email":         prof.Email,
				"orgId":         prof.OrgID,
				"authenticated": prof.AccessToken != "",
			})
		},
	}
	statusCmd.Flags().StringVar(&statusProfile, "profile", "", "profile name override")
	cmd.AddCommand(statusCmd)

	var logoutProfile string
	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear access and refresh token for a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(flags.ConfigPath)
			if err != nil {
				return err
			}
			profileName := chooseProfile(flags.Profile, logoutProfile)
			prof, name, err := currentProfile(cfg, profileName)
			if err != nil {
				return err
			}
			prof.AccessToken = ""
			prof.RefreshToken = ""
			cfg.Profiles[name] = prof
			if err := config.Save(flags.ConfigPath, cfg); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"profile":       name,
				"authenticated": false,
			})
		},
	}
	logoutCmd.Flags().StringVar(&logoutProfile, "profile", "", "profile name override")
	cmd.AddCommand(logoutCmd)

	var refreshProfile string
	refreshCmd := &cobra.Command{
		Use:   "refresh",
		Short: "Rotate access/refresh tokens for a profile session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(flags.ConfigPath)
			if err != nil {
				return err
			}
			profileName := chooseProfile(flags.Profile, refreshProfile)
			prof, name, err := currentProfile(cfg, profileName)
			if err != nil {
				return err
			}
			if prof.AccessToken == "" || prof.RefreshToken == "" {
				return signozerrors.NewAuthRequiredError("profile_not_authenticated", "profile is missing session tokens; run `signozctl auth login` first")
			}

			accessToken, refreshToken, err := rotateSession(cmd.Context(), prof.Host, prof.AccessToken, prof.RefreshToken)
			if err != nil {
				return err
			}
			prof.AccessToken = accessToken
			prof.RefreshToken = refreshToken
			cfg.Profiles[name] = prof
			if err := config.Save(flags.ConfigPath, cfg); err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"profile":       name,
				"host":          prof.Host,
				"authenticated": true,
				"rotated":       true,
			})
		},
	}
	refreshCmd.Flags().StringVar(&refreshProfile, "profile", "", "profile name override")
	cmd.AddCommand(refreshCmd)

	useCmd := &cobra.Command{
		Use:   "use <profile>",
		Short: "Set active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.ConfigPath)
			if err != nil {
				return err
			}
			if _, ok := cfg.Profiles[args[0]]; !ok {
				return errProfileNotFound(args[0])
			}
			cfg.ActiveProfile = args[0]
			if err := config.Save(flags.ConfigPath, cfg); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{"activeProfile": args[0]})
		},
	}
	cmd.AddCommand(useCmd)

	listCmd := &cobra.Command{
		Use:   "profiles",
		Short: "List configured profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(flags.ConfigPath)
			if err != nil {
				return err
			}
			list := make([]map[string]any, 0, len(cfg.Profiles))
			for name, prof := range cfg.Profiles {
				list = append(list, map[string]any{
					"name":          name,
					"host":          prof.Host,
					"email":         prof.Email,
					"orgId":         prof.OrgID,
					"authenticated": prof.AccessToken != "",
					"active":        name == cfg.ActiveProfile,
				})
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, list)
		},
	}
	cmd.AddCommand(listCmd)

	return cmd
}

func newQueryCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query Metrics, Logs, and Traces",
	}

	cmd.AddCommand(newQueryTemplateCommand(flags))
	cmd.AddCommand(newQuerySchemaCommand(flags))
	cmd.AddCommand(newQueryValidateCommand(flags))
	cmd.AddCommand(newQueryFieldsCommand(flags))
	cmd.AddCommand(newQueryOperatorsCommand(flags))
	cmd.AddCommand(newQueryLintCommand(flags))

	cmd.AddCommand(newQueryFileCommand(flags, "traces", "/api/v5/query_range", "Run a traces query payload against /api/v5/query_range"))
	cmd.AddCommand(newQueryFileCommand(flags, "logs", "/api/v5/query_range", "Run a logs query payload against /api/v5/query_range"))
	cmd.AddCommand(newQueryFileCommand(flags, "metrics", "/api/v5/query_range", "Run a metrics query payload against /api/v5/query_range"))
	cmd.AddCommand(newLogsTailCommand(flags))
	cmd.AddCommand(newProfileGetByIDCommand(flags, "trace <trace-id>", "Get trace summary by trace ID", "/api/v1/traces/%s"))
	cmd.AddCommand(newTraceRootCommand(flags))
	cmd.AddCommand(newTraceWaterfallCommand(flags))
	cmd.AddCommand(newTraceFlamegraphCommand(flags))
	cmd.AddCommand(newProfileGetCommand(flags, "trace-fields", "Get trace field config", "/api/v2/traces/fields"))
	cmd.AddCommand(newProfilePostFromFileCommand(flags, "trace-fields-update", "Update trace field config", "/api/v2/traces/fields"))
	cmd.AddCommand(newProfilePostFromFileCommand(flags, "services", "Query services", "/api/v2/services"))
	cmd.AddCommand(newProfilePostFromFileCommand(flags, "service-ops", "Query top service operations", "/api/v2/service/top_operations"))
	cmd.AddCommand(newProfilePostFromFileCommand(flags, "entrypoint-ops", "Query service entrypoint operations", "/api/v2/service/entry_point_operations"))
	cmd.AddCommand(newProfilePostFromFileCommand(flags, "dependency-graph", "Get dependency graph", "/api/v1/dependency_graph"))
	cmd.AddCommand(newProfilePostFromFileCommand(flags, "errors-list", "List grouped errors", "/api/v1/listErrors"))
	cmd.AddCommand(newProfilePostFromFileCommand(flags, "errors-count", "Count grouped errors", "/api/v1/countErrors"))

	var profile string
	errorDetailCmd := &cobra.Command{
		Use:   "error-detail <error-id>",
		Short: "Get error details by error ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := profileClient(flags, profile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.GetJSON(cmd.Context(), "/api/v1/errorFromErrorID?errorID="+url.QueryEscape(args[0]), &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	errorDetailCmd.Flags().StringVar(&profile, "profile", "", "profile name override")
	cmd.AddCommand(errorDetailCmd)

	return cmd
}

func newQueryFileCommand(flags *globalFlags, use, path, short string) *cobra.Command {
	var filePath string
	var localProfile string
	var last string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filePath == "" {
				return signozerrors.NewMissingRequiredFlagError("--file")
			}
			raw, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			if last != "" {
				duration, err := parseRelativeDuration(last)
				if err != nil {
					return signozerrors.NewInputValidationError("invalid_relative_duration", fmt.Sprintf("invalid --last value %q: %v", last, err))
				}
				end := time.Now().UnixMilli()
				start := end - duration.Milliseconds()
				if start < 0 {
					start = 0
				}
				raw, err = applyTimeRange(raw, start, end)
				if err != nil {
					return signozerrors.NewInputValidationError("invalid_payload", "failed to apply --last time range: "+err.Error())
				}
			}
			c, err := profileClient(flags, localProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.PostRawJSON(cmd.Context(), path, raw, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "JSON payload file")
	cmd.Flags().StringVar(&localProfile, "profile", "", "profile name override")
	cmd.Flags().StringVar(&last, "last", "", "relative time range ending now (e.g. 5m, 2h, 24h, 2d, 1w)")
	return cmd
}

func newDashboardCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Create, update, delete, and list dashboards",
	}

	cmd.AddCommand(newDashboardTemplateCommand(flags))
	cmd.AddCommand(newDashboardSchemaCommand(flags))
	cmd.AddCommand(newDashboardValidateCommand(flags))
	cmd.AddCommand(newDashboardLintCommand(flags))
	cmd.AddCommand(newDashboardCapabilitiesCommand(flags))
	cmd.AddCommand(newDashboardWidgetTemplateCommand(flags))
	cmd.AddCommand(newDashboardCookbookCommand(flags))
	cmd.AddCommand(newDashboardTemplatesCommand(flags))

	var filePath string
	var localProfile string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a dashboard from JSON file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filePath == "" {
				return signozerrors.NewMissingRequiredFlagError("--file")
			}
			raw, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			c, err := profileClient(flags, localProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.PostRawJSON(cmd.Context(), "/api/v1/dashboards", raw, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	createCmd.Flags().StringVar(&filePath, "file", "", "dashboard JSON file")
	createCmd.Flags().StringVar(&localProfile, "profile", "", "profile name override")
	cmd.AddCommand(createCmd)

	var viewTitle string
	var viewDescription string
	var viewProfile string
	viewCreateCmd := &cobra.Command{
		Use:   "view-create",
		Short: "Create an empty dashboard view with title/description flags",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(viewTitle) == "" {
				return signozerrors.NewMissingRequiredFlagError("--title")
			}
			c, err := profileClient(flags, viewProfile)
			if err != nil {
				return err
			}
			payload := map[string]any{
				"title":       viewTitle,
				"description": viewDescription,
				"widgets":     []any{},
				"layout":      []any{},
				"variables":   map[string]any{},
			}
			var resp map[string]any
			if err := c.PostJSON(cmd.Context(), "/api/v1/dashboards", payload, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	viewCreateCmd.Flags().StringVar(&viewTitle, "title", "", "dashboard title")
	viewCreateCmd.Flags().StringVar(&viewDescription, "description", "", "dashboard description")
	viewCreateCmd.Flags().StringVar(&viewProfile, "profile", "", "profile name override")
	cmd.AddCommand(viewCreateCmd)

	var listProfile string
	var listSummary bool
	var listFull bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List dashboards",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := profileClient(flags, listProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.GetJSON(cmd.Context(), "/api/v1/dashboards", &resp); err != nil {
				return err
			}
			if !listFull {
				return output.Render(cmd.OutOrStdout(), flags.Output, summarizeListResponse(resp))
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	listCmd.Flags().StringVar(&listProfile, "profile", "", "profile name override")
	listCmd.Flags().BoolVar(&listSummary, "summary", true, "return concise summary fields")
	listCmd.Flags().BoolVar(&listFull, "full", false, "return full raw payload")
	cmd.AddCommand(listCmd)

	var getProfile string
	getCmd := &cobra.Command{
		Use:   "get <dashboard-id>",
		Short: "Get dashboard by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := profileClient(flags, getProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.GetJSON(cmd.Context(), "/api/v1/dashboards/"+args[0], &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	getCmd.Flags().StringVar(&getProfile, "profile", "", "profile name override")
	cmd.AddCommand(getCmd)

	var updateFile string
	var updateProfile string
	var updateShowNormalizedDiff bool
	updateCmd := &cobra.Command{
		Use:   "update <dashboard-id>",
		Short: "Update dashboard with JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if updateFile == "" {
				return signozerrors.NewMissingRequiredFlagError("--file")
			}
			raw, err := os.ReadFile(updateFile)
			if err != nil {
				return err
			}
			c, err := profileClient(flags, updateProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.PutRawJSON(cmd.Context(), "/api/v1/dashboards/"+args[0], raw, &resp); err != nil {
				return err
			}
			if updateShowNormalizedDiff {
				var sent map[string]any
				_ = json.Unmarshal(raw, &sent)
				if normalized, ok := extractMapAtPath(resp, "data", "data"); ok {
					resp["normalizedDiff"] = normalizedTopLevelDiff(sent, normalized)
				}
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	updateCmd.Flags().StringVar(&updateFile, "file", "", "dashboard JSON file")
	updateCmd.Flags().StringVar(&updateProfile, "profile", "", "profile name override")
	updateCmd.Flags().BoolVar(&updateShowNormalizedDiff, "show-normalized-diff", false, "show top-level diff between sent payload and server-normalized payload")
	cmd.AddCommand(updateCmd)

	var applyProfile string
	var applyFile string
	var applyBy string
	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Create-or-update dashboard from JSON payload",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if applyFile == "" {
				return signozerrors.NewMissingRequiredFlagError("--file")
			}
			payload, raw, err := parsePayloadFile(applyFile)
			if err != nil {
				return err
			}
			matchBy := strings.ToLower(strings.TrimSpace(applyBy))
			if matchBy == "" {
				matchBy = "title"
			}
			if matchBy != "title" {
				return errInvalidOption("by", matchBy, "title")
			}
			targetTitle, _ := payload["title"].(string)
			if strings.TrimSpace(targetTitle) == "" {
				return signozerrors.NewInputValidationError("invalid_payload", "title is required for dashboard apply --by title")
			}

			c, err := profileClient(flags, applyProfile)
			if err != nil {
				return err
			}
			var listResp map[string]any
			if err := c.GetJSON(cmd.Context(), "/api/v1/dashboards", &listResp); err != nil {
				return err
			}
			items, _ := listResp["data"].([]any)
			for _, item := range items {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				id, _ := m["id"].(string)
				data, _ := m["data"].(map[string]any)
				title, _ := data["title"].(string)
				if id != "" && title == targetTitle {
					var updateResp map[string]any
					if err := c.PutRawJSON(cmd.Context(), "/api/v1/dashboards/"+id, raw, &updateResp); err != nil {
						return err
					}
					return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
						"mode": "updated", "id": id, "response": updateResp,
					})
				}
			}
			var createResp map[string]any
			if err := c.PostRawJSON(cmd.Context(), "/api/v1/dashboards", raw, &createResp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"mode": "created", "response": createResp,
			})
		},
	}
	applyCmd.Flags().StringVar(&applyProfile, "profile", "", "profile name override")
	applyCmd.Flags().StringVar(&applyFile, "file", "", "dashboard JSON file")
	applyCmd.Flags().StringVar(&applyBy, "by", "title", "idempotency key: title")
	cmd.AddCommand(applyCmd)

	var deleteProfile string
	deleteCmd := &cobra.Command{
		Use:   "delete <dashboard-id>",
		Short: "Delete dashboard by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := profileClient(flags, deleteProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.Delete(cmd.Context(), "/api/v1/dashboards/"+args[0], &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	deleteCmd.Flags().StringVar(&deleteProfile, "profile", "", "profile name override")
	cmd.AddCommand(deleteCmd)

	var panelGetProfile string
	panelGetCmd := &cobra.Command{
		Use:   "panel-get <dashboard-id> <panel-id>",
		Short: "Get single panel/widget JSON from a dashboard",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := profileClient(flags, panelGetProfile)
			if err != nil {
				return err
			}
			resp, data, err := fetchDashboardData(cmd.Context(), c, args[0])
			if err != nil {
				return err
			}
			_ = resp
			widgets, _ := data["widgets"].([]any)
			for _, w := range widgets {
				wm, ok := w.(map[string]any)
				if !ok {
					continue
				}
				if id, _ := wm["id"].(string); id == args[1] {
					return output.Render(cmd.OutOrStdout(), flags.Output, wm)
				}
			}
			return signozerrors.NewInputValidationError("panel_not_found", fmt.Sprintf("panel %q not found in dashboard %q", args[1], args[0]))
		},
	}
	panelGetCmd.Flags().StringVar(&panelGetProfile, "profile", "", "profile name override")
	cmd.AddCommand(panelGetCmd)

	var panelUpdateProfile string
	var panelUpdateFile string
	panelUpdateCmd := &cobra.Command{
		Use:   "panel-update <dashboard-id> <panel-id>",
		Short: "Update one panel/widget in a dashboard using panel JSON file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if panelUpdateFile == "" {
				return signozerrors.NewMissingRequiredFlagError("--file")
			}
			rawPanel, err := os.ReadFile(panelUpdateFile)
			if err != nil {
				return err
			}
			var panel map[string]any
			if err := json.Unmarshal(rawPanel, &panel); err != nil {
				return errInvalidJSONPayload(err)
			}
			panel["id"] = args[1]

			c, err := profileClient(flags, panelUpdateProfile)
			if err != nil {
				return err
			}
			_, data, err := fetchDashboardData(cmd.Context(), c, args[0])
			if err != nil {
				return err
			}

			widgets, _ := data["widgets"].([]any)
			found := false
			for i, w := range widgets {
				wm, ok := w.(map[string]any)
				if !ok {
					continue
				}
				if id, _ := wm["id"].(string); id == args[1] {
					widgets[i] = panel
					found = true
					break
				}
			}
			if !found {
				return signozerrors.NewInputValidationError("panel_not_found", fmt.Sprintf("panel %q not found in dashboard %q", args[1], args[0]))
			}
			data["widgets"] = widgets

			updatedRaw, err := json.Marshal(data)
			if err != nil {
				return err
			}
			var updateResp map[string]any
			if err := c.PutRawJSON(cmd.Context(), "/api/v1/dashboards/"+args[0], updatedRaw, &updateResp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, updateResp)
		},
	}
	panelUpdateCmd.Flags().StringVar(&panelUpdateProfile, "profile", "", "profile name override")
	panelUpdateCmd.Flags().StringVar(&panelUpdateFile, "file", "", "panel JSON file")
	cmd.AddCommand(panelUpdateCmd)

	var panelListProfile string
	panelListCmd := &cobra.Command{
		Use:   "panel-list <dashboard-id>",
		Short: "List panel/widget JSON for a dashboard",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := profileClient(flags, panelListProfile)
			if err != nil {
				return err
			}
			_, data, err := fetchDashboardData(cmd.Context(), c, args[0])
			if err != nil {
				return err
			}
			widgets, _ := data["widgets"].([]any)
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"dashboardId": args[0],
				"widgets":     widgets,
			})
		},
	}
	panelListCmd.Flags().StringVar(&panelListProfile, "profile", "", "profile name override")
	cmd.AddCommand(panelListCmd)

	var panelAddProfile string
	var panelAddFile string
	var panelX int
	var panelY int
	var panelW int
	var panelH int
	panelAddCmd := &cobra.Command{
		Use:   "panel-add <dashboard-id>",
		Short: "Add a panel/widget to dashboard from panel JSON file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if panelAddFile == "" {
				return signozerrors.NewMissingRequiredFlagError("--file")
			}
			rawPanel, err := os.ReadFile(panelAddFile)
			if err != nil {
				return err
			}
			var panel map[string]any
			if err := json.Unmarshal(rawPanel, &panel); err != nil {
				return errInvalidJSONPayload(err)
			}
			panelID, _ := panel["id"].(string)
			if strings.TrimSpace(panelID) == "" {
				return signozerrors.NewInputValidationError("invalid_panel_payload", "panel id is required in panel JSON (`id`)")
			}

			c, err := profileClient(flags, panelAddProfile)
			if err != nil {
				return err
			}
			_, data, err := fetchDashboardData(cmd.Context(), c, args[0])
			if err != nil {
				return err
			}

			widgets, _ := data["widgets"].([]any)
			for _, w := range widgets {
				wm, ok := w.(map[string]any)
				if !ok {
					continue
				}
				if id, _ := wm["id"].(string); id == panelID {
					return signozerrors.NewInputValidationError("panel_id_conflict", fmt.Sprintf("panel id %q already exists", panelID))
				}
			}
			widgets = append(widgets, panel)
			data["widgets"] = widgets

			layout, _ := data["layout"].([]any)
			layout = append(layout, map[string]any{
				"i": panelID,
				"x": panelX,
				"y": panelY,
				"w": panelW,
				"h": panelH,
			})
			data["layout"] = layout

			updatedRaw, err := json.Marshal(data)
			if err != nil {
				return err
			}
			var updateResp map[string]any
			if err := c.PutRawJSON(cmd.Context(), "/api/v1/dashboards/"+args[0], updatedRaw, &updateResp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, updateResp)
		},
	}
	panelAddCmd.Flags().StringVar(&panelAddProfile, "profile", "", "profile name override")
	panelAddCmd.Flags().StringVar(&panelAddFile, "file", "", "panel JSON file")
	panelAddCmd.Flags().IntVar(&panelX, "x", 0, "layout x position")
	panelAddCmd.Flags().IntVar(&panelY, "y", 0, "layout y position")
	panelAddCmd.Flags().IntVar(&panelW, "width", 6, "layout width")
	panelAddCmd.Flags().IntVar(&panelH, "height", 4, "layout height")
	panelAddCmd.Flags().IntVar(&panelW, "w", 6, "deprecated: use --width")
	panelAddCmd.Flags().IntVar(&panelH, "h", 4, "deprecated: use --height")
	_ = panelAddCmd.Flags().MarkHidden("w")
	_ = panelAddCmd.Flags().MarkHidden("h")
	cmd.AddCommand(panelAddCmd)

	var panelDeleteProfile string
	panelDeleteCmd := &cobra.Command{
		Use:   "panel-delete <dashboard-id> <panel-id>",
		Short: "Delete a panel/widget from dashboard",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := profileClient(flags, panelDeleteProfile)
			if err != nil {
				return err
			}
			_, data, err := fetchDashboardData(cmd.Context(), c, args[0])
			if err != nil {
				return err
			}

			targetID := args[1]
			widgets, _ := data["widgets"].([]any)
			newWidgets := make([]any, 0, len(widgets))
			removed := false
			for _, w := range widgets {
				wm, ok := w.(map[string]any)
				if !ok {
					newWidgets = append(newWidgets, w)
					continue
				}
				if id, _ := wm["id"].(string); id == targetID {
					removed = true
					continue
				}
				newWidgets = append(newWidgets, w)
			}
			if !removed {
				return signozerrors.NewInputValidationError("panel_not_found", fmt.Sprintf("panel %q not found in dashboard %q", targetID, args[0]))
			}
			data["widgets"] = newWidgets

			layout, _ := data["layout"].([]any)
			newLayout := make([]any, 0, len(layout))
			for _, li := range layout {
				lm, ok := li.(map[string]any)
				if !ok {
					newLayout = append(newLayout, li)
					continue
				}
				if i, _ := lm["i"].(string); i == targetID {
					continue
				}
				newLayout = append(newLayout, li)
			}
			data["layout"] = newLayout

			updatedRaw, err := json.Marshal(data)
			if err != nil {
				return err
			}
			var updateResp map[string]any
			if err := c.PutRawJSON(cmd.Context(), "/api/v1/dashboards/"+args[0], updatedRaw, &updateResp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, updateResp)
		},
	}
	panelDeleteCmd.Flags().StringVar(&panelDeleteProfile, "profile", "", "profile name override")
	cmd.AddCommand(panelDeleteCmd)

	cmd.AddCommand(newDashboardPublicCreateCommand(flags))
	cmd.AddCommand(newProfileGetByIDCommand(flags, "public-get <dashboard-id>", "Get public sharing config for a dashboard", "/api/v1/dashboards/%s/public"))
	cmd.AddCommand(newDashboardPublicUpdateCommand(flags))
	cmd.AddCommand(newProfileDeleteByIDCommand(flags, "public-delete <dashboard-id>", "Delete public sharing config for a dashboard", "/api/v1/dashboards/%s/public"))

	return cmd
}

func fetchDashboardData(ctx context.Context, c *client.Client, dashboardID string) (map[string]any, map[string]any, error) {
	var resp map[string]any
	if err := c.GetJSON(ctx, "/api/v1/dashboards/"+dashboardID, &resp); err != nil {
		return nil, nil, err
	}
	dataAny, ok := resp["data"]
	if !ok {
		return nil, nil, signozerrors.NewInputValidationError("invalid_dashboard_response", "dashboard response missing data")
	}
	dataWrap, ok := dataAny.(map[string]any)
	if !ok {
		return nil, nil, signozerrors.NewInputValidationError("invalid_dashboard_response", "dashboard response data is not an object")
	}
	payloadAny, ok := dataWrap["data"]
	if !ok {
		return nil, nil, signozerrors.NewInputValidationError("invalid_dashboard_response", "dashboard payload missing data.data")
	}
	payload, ok := payloadAny.(map[string]any)
	if !ok {
		return nil, nil, signozerrors.NewInputValidationError("invalid_dashboard_response", "dashboard payload data.data is not an object")
	}
	return resp, payload, nil
}

func newSystemCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System endpoints such as health and version",
	}

	var host string
	healthCmd := &cobra.Command{
		Use:   "health",
		Short: "Get server health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			targetHost, err := hostOrProfileHost(flags, host)
			if err != nil {
				return err
			}
			c := client.New(targetHost, "")
			var resp map[string]any
			if err := c.GetJSON(cmd.Context(), "/api/v1/health", &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	healthCmd.Flags().StringVar(&host, "host", "", "SigNoz host override")
	cmd.AddCommand(healthCmd)

	var versionHost string
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Get SigNoz version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			targetHost, err := hostOrProfileHost(flags, versionHost)
			if err != nil {
				return err
			}
			c := client.New(targetHost, "")
			var resp map[string]any
			if err := c.GetJSON(cmd.Context(), "/api/v1/version", &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	versionCmd.Flags().StringVar(&versionHost, "host", "", "SigNoz host override")
	cmd.AddCommand(versionCmd)

	cmd.AddCommand(newProfileGetCommand(flags, "usage", "Get usage info", "/api/v1/usage"))
	cmd.AddCommand(newProfileGetCommand(flags, "disks", "Get disk info", "/api/v1/disks"))
	cmd.AddCommand(newProfileGetCommand(flags, "raw-export", "Export raw data", "/api/v1/export_raw_data"))
	cmd.AddCommand(newProfileGetCommand(flags, "ttl", "Get TTL settings", "/api/v1/settings/ttl"))
	cmd.AddCommand(newProfilePostFromFileCommand(flags, "set-ttl", "Set TTL settings", "/api/v1/settings/ttl"))
	cmd.AddCommand(newProfileGetCommand(flags, "apdex", "Get Apdex settings", "/api/v1/settings/apdex"))
	cmd.AddCommand(newProfilePostFromFileCommand(flags, "set-apdex", "Set Apdex settings", "/api/v1/settings/apdex"))

	return cmd
}

func fetchOrgID(ctx context.Context, c *client.Client, email, host string) (string, error) {
	ref := host
	if _, err := url.Parse(host); err != nil {
		ref = "http://localhost:8080"
	}
	path := "/api/v2/sessions/context?email=" + url.QueryEscape(email) + "&ref=" + url.QueryEscape(ref)
	var ctxResp struct {
		Data struct {
			Orgs []struct {
				ID string `json:"id"`
			} `json:"orgs"`
		} `json:"data"`
	}
	if err := c.GetJSON(ctx, path, &ctxResp); err != nil {
		return "", err
	}
	if len(ctxResp.Data.Orgs) == 0 {
		return "", signozerrors.NewInputValidationError("missing_org_context", "no organizations returned by /api/v2/sessions/context; pass --org-id explicitly")
	}
	return ctxResp.Data.Orgs[0].ID, nil
}

func rotateSession(ctx context.Context, host, accessToken, refreshToken string) (string, string, error) {
	c := client.New(host, accessToken)
	var rotateResp struct {
		Data struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		} `json:"data"`
	}
	if err := c.PostJSON(ctx, "/api/v2/sessions/rotate", map[string]string{"refreshToken": refreshToken}, &rotateResp); err != nil {
		return "", "", err
	}
	if rotateResp.Data.AccessToken == "" || rotateResp.Data.RefreshToken == "" {
		return "", "", signozerrors.NewAuthRequiredError("invalid_rotate_response", "rotate session response missing tokens")
	}
	return rotateResp.Data.AccessToken, rotateResp.Data.RefreshToken, nil
}

func chooseProfile(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func currentProfile(cfg *config.File, explicit string) (config.Profile, string, error) {
	name := explicit
	if name == "" {
		name = cfg.ActiveProfile
	}
	if name == "" {
		name = "default"
	}
	prof, ok := cfg.Profiles[name]
	if !ok {
		return config.Profile{}, "", errProfileNotFound(name)
	}
	return prof, name, nil
}

func loadProfileFromFlags(flags *globalFlags, localProfile string) (*config.File, config.Profile, string, error) {
	cfg, err := config.Load(flags.ConfigPath)
	if err != nil {
		return nil, config.Profile{}, "", err
	}
	prof, name, err := currentProfile(cfg, chooseProfile(flags.Profile, localProfile))
	if err != nil {
		return nil, config.Profile{}, "", err
	}
	if prof.Host == "" {
		return nil, config.Profile{}, "", signozerrors.NewInputValidationError("profile_missing_host", fmt.Sprintf("profile %s has no host; run auth login with --host", name))
	}
	return cfg, prof, name, nil
}

func hostOrProfileHost(flags *globalFlags, hostFlag string) (string, error) {
	if hostFlag != "" {
		return strings.TrimRight(hostFlag, "/"), nil
	}
	_, prof, _, err := loadProfileFromFlags(flags, "")
	if err != nil {
		return "", errMissingHost()
	}
	return prof.Host, nil
}

func applyTimeRange(raw []byte, startMillis, endMillis int64) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	payload["start"] = startMillis
	payload["end"] = endMillis
	return json.Marshal(payload)
}

func parseRelativeDuration(raw string) (time.Duration, error) {
	v := strings.TrimSpace(strings.ToLower(raw))
	if v == "" {
		return 0, signozerrors.NewInputValidationError("invalid_relative_duration", "duration is empty")
	}

	if strings.HasSuffix(v, "d") || strings.HasSuffix(v, "w") {
		unit := v[len(v)-1]
		n, err := strconv.Atoi(v[:len(v)-1])
		if err != nil || n <= 0 {
			return 0, signozerrors.NewInputValidationError("invalid_relative_duration", "expected positive integer before d/w")
		}
		switch unit {
		case 'd':
			return time.Duration(n) * 24 * time.Hour, nil
		case 'w':
			return time.Duration(n) * 7 * 24 * time.Hour, nil
		}
	}

	duration, err := time.ParseDuration(v)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, signozerrors.NewInputValidationError("invalid_relative_duration", "duration must be > 0")
	}
	return duration, nil
}
