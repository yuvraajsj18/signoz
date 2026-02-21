package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type dashboardTemplateIndex struct {
	Templates []dashboardTemplateMeta `json:"templates"`
}

type dashboardTemplateMeta struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Source      string   `json:"source"`
	Tags        []string `json:"tags,omitempty"`
}

func templatesIndexURL() string {
	if v := strings.TrimSpace(os.Getenv("SIGNOZCTL_DASHBOARD_TEMPLATES_INDEX_URL")); v != "" {
		return v
	}
	return "https://api.github.com/repos/SigNoz/dashboards/git/trees/main?recursive=1"
}

func templatesBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("SIGNOZCTL_DASHBOARD_TEMPLATES_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://raw.githubusercontent.com/SigNoz/dashboards/main"
}

func newDashboardTemplatesCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Browse and apply dashboard templates",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available dashboard templates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			templates, err := loadDashboardTemplates(cmd.Context())
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{"templates": templates})
		},
	}
	cmd.AddCommand(listCmd)

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search dashboard templates by id/name/description/tags",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templates, err := loadDashboardTemplates(cmd.Context())
			if err != nil {
				return err
			}
			query := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
			filtered := make([]dashboardTemplateMeta, 0)
			for _, t := range templates {
				if matchesTemplateQuery(t, query) {
					filtered = append(filtered, t)
				}
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{"query": query, "templates": filtered})
		},
	}
	cmd.AddCommand(searchCmd)

	showCmd := &cobra.Command{
		Use:   "show <template-id>",
		Short: "Show dashboard template JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tpl, err := findDashboardTemplate(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			payload, err := fetchTemplatePayload(cmd.Context(), tpl.Source)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, payload)
		},
	}
	cmd.AddCommand(showCmd)

	var profile string
	var interactive bool
	applyCmd := &cobra.Command{
		Use:   "apply [template-id]",
		Short: "Create dashboard from a template (or choose interactively)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var templateID string
			if interactive {
				templates, err := loadDashboardTemplates(cmd.Context())
				if err != nil {
					return err
				}
				templateID, err = selectTemplateInteractive(templates)
				if err != nil {
					return err
				}
			} else {
				if len(args) != 1 {
					return signozerrors.NewInputValidationError("missing_template_id", "template-id is required unless --interactive is used")
				}
				templateID = args[0]
			}

			tpl, err := findDashboardTemplate(cmd.Context(), templateID)
			if err != nil {
				return err
			}
			payload, err := fetchTemplatePayload(cmd.Context(), tpl.Source)
			if err != nil {
				return err
			}
			c, err := profileClient(flags, profile)
			if err != nil {
				return err
			}
			var resp map[string]any
			if err := c.PostJSON(cmd.Context(), "/api/v1/dashboards", payload, &resp); err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, resp)
		},
	}
	applyCmd.Flags().StringVar(&profile, "profile", "", "profile name override")
	applyCmd.Flags().BoolVar(&interactive, "interactive", false, "choose template using keyboard picker")
	cmd.AddCommand(applyCmd)

	return cmd
}

func selectTemplateInteractive(templates []dashboardTemplateMeta) (string, error) {
	if len(templates) == 0 {
		return "", signozerrors.NewInputValidationError("no_templates_available", "no dashboard templates available")
	}
	if forced := strings.TrimSpace(os.Getenv("SIGNOZCTL_INTERACTIVE_TEMPLATE_ID")); forced != "" {
		return forced, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", signozerrors.NewInputValidationError("interactive_requires_tty", "--interactive requires a TTY terminal")
	}

	options := make([]string, 0, len(templates))
	idByOption := make(map[string]string, len(templates))
	for _, tpl := range templates {
		label := fmt.Sprintf("%s  (%s)", tpl.ID, tpl.Name)
		options = append(options, label)
		idByOption[label] = tpl.ID
	}

	selected := []string{}
	prompt := &survey.MultiSelect{
		Message:  "Select template (arrow keys, space to select, enter to apply):",
		Options:  options,
		PageSize: 12,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return "", signozerrors.NewInputValidationError("interactive_selection_failed", err.Error())
	}
	if len(selected) == 0 {
		return "", signozerrors.NewInputValidationError("interactive_selection_empty", "no template selected")
	}
	if len(selected) > 1 {
		return "", signozerrors.NewInputValidationError("interactive_selection_multiple", "select exactly one template")
	}
	templateID, ok := idByOption[selected[0]]
	if !ok || templateID == "" {
		return "", signozerrors.NewInputValidationError("interactive_selection_invalid", "selected template is invalid")
	}
	return templateID, nil
}

func loadDashboardTemplates(ctx context.Context) ([]dashboardTemplateMeta, error) {
	indexURL := templatesIndexURL()
	if strings.Contains(indexURL, "api.github.com/repos/SigNoz/dashboards/git/trees/") {
		return loadTemplatesFromGitHubTree(ctx, indexURL, templatesBaseURL())
	}
	return loadTemplatesFromIndexJSON(ctx, indexURL, templatesBaseURL())
}

func loadTemplatesFromIndexJSON(ctx context.Context, indexURL, baseURL string) ([]dashboardTemplateMeta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, signozerrors.NewLocalError(signozerrors.ClassAPIUnavailable, resp.StatusCode, "templates_index_fetch_failed", fmt.Sprintf("failed to fetch templates index: status=%d", resp.StatusCode), "retry later or set SIGNOZCTL_DASHBOARD_TEMPLATES_INDEX_URL")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var idx dashboardTemplateIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, errInvalidJSONPayload(err)
	}
	for i := range idx.Templates {
		if idx.Templates[i].ID == "" {
			idx.Templates[i].ID = sanitizeTemplateID(idx.Templates[i].Name)
		}
		if idx.Templates[i].Source != "" && !strings.HasPrefix(idx.Templates[i].Source, "http://") && !strings.HasPrefix(idx.Templates[i].Source, "https://") {
			idx.Templates[i].Source = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(idx.Templates[i].Source, "/")
		}
	}
	sort.Slice(idx.Templates, func(i, j int) bool { return idx.Templates[i].ID < idx.Templates[j].ID })
	return idx.Templates, nil
}

func loadTemplatesFromGitHubTree(ctx context.Context, treeURL, baseURL string) ([]dashboardTemplateMeta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, treeURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, signozerrors.NewLocalError(signozerrors.ClassAPIUnavailable, resp.StatusCode, "templates_tree_fetch_failed", fmt.Sprintf("failed to fetch GitHub templates tree: status=%d", resp.StatusCode), "retry later or set SIGNOZCTL_DASHBOARD_TEMPLATES_INDEX_URL")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, errInvalidJSONPayload(err)
	}
	templates := make([]dashboardTemplateMeta, 0)
	for _, item := range tree.Tree {
		if item.Type != "blob" || !strings.HasSuffix(item.Path, ".json") {
			continue
		}
		id := sanitizeTemplateID(strings.TrimSuffix(pathBase(item.Path), ".json"))
		if id == "" {
			continue
		}
		templates = append(templates, dashboardTemplateMeta{
			ID:          id,
			Name:        strings.ReplaceAll(strings.TrimSuffix(pathBase(item.Path), ".json"), "-", " "),
			Description: item.Path,
			Source:      strings.TrimRight(baseURL, "/") + "/" + item.Path,
		})
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].ID < templates[j].ID })
	return templates, nil
}

func findDashboardTemplate(ctx context.Context, id string) (*dashboardTemplateMeta, error) {
	templates, err := loadDashboardTemplates(ctx)
	if err != nil {
		return nil, err
	}
	needle := sanitizeTemplateID(id)
	for _, t := range templates {
		if sanitizeTemplateID(t.ID) == needle {
			c := t
			return &c, nil
		}
	}
	return nil, errInvalidOption("template-id", id, "run `signozctl dashboard templates list`")
}

func fetchTemplatePayload(ctx context.Context, source string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, signozerrors.NewLocalError(signozerrors.ClassAPIUnavailable, resp.StatusCode, "template_payload_fetch_failed", fmt.Sprintf("failed to fetch template payload: status=%d", resp.StatusCode), "verify template source URL and retry")
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, errInvalidJSONPayload(err)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}

func matchesTemplateQuery(t dashboardTemplateMeta, q string) bool {
	hay := strings.ToLower(strings.Join(append([]string{t.ID, t.Name, t.Description}, t.Tags...), " "))
	for _, token := range strings.Fields(q) {
		if !strings.Contains(hay, token) {
			return false
		}
	}
	return true
}

func sanitizeTemplateID(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func pathBase(p string) string {
	parts := strings.Split(strings.TrimSpace(p), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
