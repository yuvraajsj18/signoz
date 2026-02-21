package commands

import (
	"fmt"
	"os"

	"github.com/SigNoz/signoz/tools/signozctl/internal/client"
	"github.com/SigNoz/signoz/tools/signozctl/internal/config"
	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
)

func newAlertsCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "Alerting resources: alerts, rules, channels, route policies, downtime",
	}

	cmd.AddCommand(newAlertsTemplateCommand(flags))
	cmd.AddCommand(newAlertsSchemaCommand(flags))
	cmd.AddCommand(newAlertsValidateCommand(flags))

	cmd.AddCommand(newProfileGetCommand(flags, "list", "List active alerts", "/api/v1/alerts"))
	cmd.AddCommand(buildCRUDGroup(flags, "rules", "/api/v1/rules"))
	cmd.AddCommand(buildCRUDGroup(flags, "channels", "/api/v1/channels"))
	cmd.AddCommand(buildCRUDGroup(flags, "route-policies", "/api/v1/route_policies"))
	cmd.AddCommand(buildCRUDGroup(flags, "downtime", "/api/v1/downtime_schedules"))

	testRule := newProfilePostFromFileCommand(flags, "test-rule", "Test alert rule with request payload", "/api/v1/testRule")
	cmd.AddCommand(testRule)
	testChannel := newProfilePostFromFileCommand(flags, "test-channel", "Test alert channel with request payload", "/api/v1/testChannel")
	cmd.AddCommand(testChannel)

	return cmd
}

func newIAMCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "iam",
		Short: "Identity and access commands: invites, roles, API keys, users",
	}

	cmd.AddCommand(newIAMTemplateCommand(flags))
	cmd.AddCommand(newIAMSchemaCommand(flags))
	cmd.AddCommand(newIAMValidateCommand(flags))

	invite := &cobra.Command{
		Use:   "invite",
		Short: "Invite management",
	}
	invite.AddCommand(newProfileGetCommand(flags, "list", "List invites", "/api/v1/invite"))
	invite.AddCommand(newProfilePostFromFileCommand(flags, "create", "Create invite from JSON", "/api/v1/invite"))
	invite.AddCommand(newProfileDeleteByIDCommand(flags, "delete <id>", "Delete invite by ID", "/api/v1/invite/%s"))
	cmd.AddCommand(invite)

	roles := buildCRUDGroup(flags, "roles", "/api/v1/roles")
	cmd.AddCommand(roles)

	apiKeys := &cobra.Command{
		Use:   "api-keys",
		Short: "Personal API key lifecycle commands",
	}
	apiKeys.AddCommand(newProfileGetCommand(flags, "list", "List API keys", "/api/v1/pats"))
	apiKeys.AddCommand(newProfilePostFromFileCommand(flags, "create", "Create API key from JSON", "/api/v1/pats"))
	apiKeys.AddCommand(newProfilePutByIDFromFileCommand(flags, "update <id>", "Update API key by ID", "/api/v1/pats/%s"))
	apiKeys.AddCommand(newProfileDeleteByIDCommand(flags, "revoke <id>", "Revoke API key by ID", "/api/v1/pats/%s"))
	cmd.AddCommand(apiKeys)

	users := &cobra.Command{
		Use:   "users",
		Short: "User lookup commands",
	}
	users.AddCommand(newProfileGetCommand(flags, "list", "List users", "/api/v1/user"))
	users.AddCommand(newProfileGetCommand(flags, "me", "Get current user", "/api/v1/user/me"))
	cmd.AddCommand(users)

	return cmd
}

func buildCRUDGroup(flags *globalFlags, name, basePath string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("%s CRUD operations", name),
	}
	cmd.AddCommand(newProfileGetCommand(flags, "list", fmt.Sprintf("List %s", name), basePath))
	cmd.AddCommand(newProfileGetByIDCommand(flags, "get <id>", fmt.Sprintf("Get %s by ID", name), basePath+"/%s"))
	cmd.AddCommand(newProfilePostFromFileCommand(flags, "create", fmt.Sprintf("Create %s from JSON", name), basePath))
	cmd.AddCommand(newProfilePutByIDFromFileCommand(flags, "update <id>", fmt.Sprintf("Update %s by ID", name), basePath+"/%s"))
	cmd.AddCommand(newProfileDeleteByIDCommand(flags, "delete <id>", fmt.Sprintf("Delete %s by ID", name), basePath+"/%s"))
	return cmd
}

func newProfileGetCommand(flags *globalFlags, use, short, path string) *cobra.Command {
	var localProfile string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := profileClient(flags, localProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.GetJSON(cmd.Context(), path, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&localProfile, "profile", "", "profile name override")
	return cmd
}

func newProfileGetByIDCommand(flags *globalFlags, use, short, pathFmt string) *cobra.Command {
	var localProfile string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := profileClient(flags, localProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.GetJSON(cmd.Context(), fmt.Sprintf(pathFmt, args[0]), &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&localProfile, "profile", "", "profile name override")
	return cmd
}

func newProfilePostFromFileCommand(flags *globalFlags, use, short, path string) *cobra.Command {
	var localProfile string
	var filePath string
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
	return cmd
}

func newProfilePutByIDFromFileCommand(flags *globalFlags, use, short, pathFmt string) *cobra.Command {
	var localProfile string
	var filePath string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if err := c.PutRawJSON(cmd.Context(), fmt.Sprintf(pathFmt, args[0]), raw, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "JSON payload file")
	cmd.Flags().StringVar(&localProfile, "profile", "", "profile name override")
	return cmd
}

func newProfileDeleteByIDCommand(flags *globalFlags, use, short, pathFmt string) *cobra.Command {
	var localProfile string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := profileClient(flags, localProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.Delete(cmd.Context(), fmt.Sprintf(pathFmt, args[0]), &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&localProfile, "profile", "", "profile name override")
	return cmd
}

func newProfilePostByIDFromFileCommand(flags *globalFlags, use, short, pathFmt string) *cobra.Command {
	var localProfile string
	var filePath string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if err := c.PostRawJSON(cmd.Context(), fmt.Sprintf(pathFmt, args[0]), raw, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "JSON payload file")
	cmd.Flags().StringVar(&localProfile, "profile", "", "profile name override")
	return cmd
}

func newProfilePostByIDWithOptionalFileCommand(flags *globalFlags, use, short, pathFmt string) *cobra.Command {
	var localProfile string
	var filePath string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw := []byte(`{}`)
			if filePath != "" {
				var err error
				raw, err = os.ReadFile(filePath)
				if err != nil {
					return err
				}
			}
			c, err := profileClient(flags, localProfile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.PostRawJSON(cmd.Context(), fmt.Sprintf(pathFmt, args[0]), raw, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "optional JSON payload file")
	cmd.Flags().StringVar(&localProfile, "profile", "", "profile name override")
	return cmd
}

func profileClient(flags *globalFlags, localProfile string) (*client.Client, error) {
	cfg, prof, profileName, err := loadProfileFromFlags(flags, localProfile)
	if err != nil {
		return nil, err
	}
	if prof.AccessToken == "" {
		return nil, signozerrors.NewAuthRequiredError("profile_not_authenticated", "profile is not authenticated; run `signozctl auth login` first")
	}
	c := client.New(prof.Host, prof.AccessToken)
	c.Refresh = prof.RefreshToken
	c.OnRotated = func(accessToken, refreshToken string) error {
		latest, err := config.Load(flags.ConfigPath)
		if err != nil {
			return err
		}
		p, ok := latest.Profiles[profileName]
		if !ok {
			p = cfg.Profiles[profileName]
		}
		p.AccessToken = accessToken
		p.RefreshToken = refreshToken
		latest.Profiles[profileName] = p
		return config.Save(flags.ConfigPath, latest)
	}
	return c, nil
}
