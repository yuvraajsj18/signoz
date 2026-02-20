package commands

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/SigNoz/signoz/tools/signozctl/internal/client"
	"github.com/SigNoz/signoz/tools/signozctl/internal/config"
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
	cmd.AddCommand(newDashboardCommand(flags))
	cmd.AddCommand(newAlertsCommand(flags))
	cmd.AddCommand(newIAMCommand(flags))
	cmd.AddCommand(newSystemCommand(flags))
	cmd.AddCommand(newDocsCommand())

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
				return errors.New("missing required flags: --host, --email, --password")
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
				return fmt.Errorf("profile not found: %s", args[0])
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

	cmd.AddCommand(newQueryFileCommand(flags, "traces", "/api/v5/query_range", "Run a traces query payload against /api/v5/query_range"))
	cmd.AddCommand(newQueryFileCommand(flags, "logs", "/api/v5/query_range", "Run a logs query payload against /api/v5/query_range"))
	cmd.AddCommand(newQueryFileCommand(flags, "metrics", "/api/v5/query_range", "Run a metrics query payload against /api/v5/query_range"))
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
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filePath == "" {
				return errors.New("missing required flag: --file")
			}
			raw, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			cfg, prof, _, err := loadProfileFromFlags(flags, localProfile)
			_ = cfg
			if err != nil {
				return err
			}
			if prof.AccessToken == "" {
				return errors.New("profile is not authenticated; run `signozctl auth login` first")
			}

			c := client.New(prof.Host, prof.AccessToken)
			var resp map[string]any
			if err := c.PostRawJSON(cmd.Context(), path, raw, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "JSON payload file")
	cmd.Flags().StringVar(&localProfile, "profile", "", "profile name override")
	return cmd
}

func newDashboardCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Create, update, delete, and list dashboards",
	}

	var filePath string
	var localProfile string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a dashboard from JSON file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filePath == "" {
				return errors.New("missing required flag: --file")
			}
			raw, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			_, prof, _, err := loadProfileFromFlags(flags, localProfile)
			if err != nil {
				return err
			}
			c := client.New(prof.Host, prof.AccessToken)
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

	var listProfile string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List dashboards",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, prof, _, err := loadProfileFromFlags(flags, listProfile)
			if err != nil {
				return err
			}
			c := client.New(prof.Host, prof.AccessToken)
			var resp map[string]any
			if err := c.GetJSON(cmd.Context(), "/api/v1/dashboards", &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	listCmd.Flags().StringVar(&listProfile, "profile", "", "profile name override")
	cmd.AddCommand(listCmd)

	var updateFile string
	var updateProfile string
	updateCmd := &cobra.Command{
		Use:   "update <dashboard-id>",
		Short: "Update dashboard with JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if updateFile == "" {
				return errors.New("missing required flag: --file")
			}
			raw, err := os.ReadFile(updateFile)
			if err != nil {
				return err
			}
			_, prof, _, err := loadProfileFromFlags(flags, updateProfile)
			if err != nil {
				return err
			}
			c := client.New(prof.Host, prof.AccessToken)
			var resp map[string]any
			if err := c.PutRawJSON(cmd.Context(), "/api/v1/dashboards/"+args[0], raw, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	updateCmd.Flags().StringVar(&updateFile, "file", "", "dashboard JSON file")
	updateCmd.Flags().StringVar(&updateProfile, "profile", "", "profile name override")
	cmd.AddCommand(updateCmd)

	var deleteProfile string
	deleteCmd := &cobra.Command{
		Use:   "delete <dashboard-id>",
		Short: "Delete dashboard by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, prof, _, err := loadProfileFromFlags(flags, deleteProfile)
			if err != nil {
				return err
			}
			c := client.New(prof.Host, prof.AccessToken)
			var resp map[string]any
			if err := c.Delete(cmd.Context(), "/api/v1/dashboards/"+args[0], &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	deleteCmd.Flags().StringVar(&deleteProfile, "profile", "", "profile name override")
	cmd.AddCommand(deleteCmd)

	return cmd
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
		return "", errors.New("no organizations returned by /api/v2/sessions/context; pass --org-id explicitly")
	}
	return ctxResp.Data.Orgs[0].ID, nil
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
		return config.Profile{}, "", fmt.Errorf("profile not found: %s", name)
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
		return nil, config.Profile{}, "", fmt.Errorf("profile %s has no host; run auth login with --host", name)
	}
	return cfg, prof, name, nil
}

func hostOrProfileHost(flags *globalFlags, hostFlag string) (string, error) {
	if hostFlag != "" {
		return strings.TrimRight(hostFlag, "/"), nil
	}
	_, prof, _, err := loadProfileFromFlags(flags, "")
	if err != nil {
		return "", errors.New("missing host: use --host or authenticate with a profile")
	}
	return prof.Host, nil
}
