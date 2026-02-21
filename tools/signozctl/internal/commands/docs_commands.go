package commands

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
	"github.com/SigNoz/signoz/tools/signozctl/internal/output"
	"github.com/spf13/cobra"
)

type sitemapDoc struct {
	Loc string `xml:"loc"`
}

type sitemapURLSet struct {
	URLs []sitemapDoc `xml:"url"`
}

type docsIndexCacheFile struct {
	Format     string   `json:"format"`
	SitemapURL string   `json:"sitemapUrl"`
	UpdatedAt  int64    `json:"updatedAt"`
	URLs       []string `json:"urls"`
}

var (
	docsIndexMu  sync.RWMutex
	docsIndexMem = map[string][]string{}
)

func docsSitemapURL() string {
	if v := strings.TrimSpace(os.Getenv("SIGNOZCTL_DOCS_SITEMAP_URL")); v != "" {
		return v
	}
	return "https://signoz.io/sitemap.xml"
}

func docsAllowedHost() string {
	if v := strings.TrimSpace(os.Getenv("SIGNOZCTL_DOCS_HOST")); v != "" {
		return strings.ToLower(v)
	}
	return "signoz.io"
}

func docsCachePath() string {
	if v := strings.TrimSpace(os.Getenv("SIGNOZCTL_DOCS_CACHE_PATH")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "./docs_index.json"
	}
	return filepath.Join(home, ".cache", "signozctl", "docs_index.json")
}

func newDocsCommand(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Docs-intelligence commands for SigNoz docs",
	}

	var limit int
	var refreshIndex bool
	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search SigNoz docs using sitemap + fuzzy URL ranking",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return signozerrors.NewInputValidationError("invalid_query", "query cannot be empty")
			}
			results, err := searchDocs(cmd.Context(), docsSitemapURL(), docsCachePath(), query, limit, refreshIndex)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"query":   query,
				"results": results,
			})
		},
	}
	searchCmd.Flags().IntVar(&limit, "limit", 5, "max docs results to return")
	searchCmd.Flags().BoolVar(&refreshIndex, "refresh-index", false, "force sitemap refetch and rebuild index cache")
	cmd.AddCommand(searchCmd)

	fetchCmd := &cobra.Command{
		Use:   "fetch <url>",
		Short: "Fetch docs page content and return markdown-like text",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawURL := strings.TrimSpace(args[0])
			if err := validateDocsURL(rawURL, docsAllowedHost()); err != nil {
				return err
			}
			md, err := fetchDocsMarkdown(cmd.Context(), rawURL)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), flags.Output, map[string]any{
				"url":      rawURL,
				"markdown": md,
			})
		},
	}
	cmd.AddCommand(fetchCmd)

	return cmd
}

func validateDocsURL(rawURL, host string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return signozerrors.NewInputValidationError("invalid_url", fmt.Sprintf("invalid URL: %v", err))
	}
	if strings.ToLower(u.Hostname()) != strings.ToLower(host) {
		return signozerrors.NewInputValidationError("unsupported_host", fmt.Sprintf("only %s docs URLs are supported", host))
	}
	if !strings.HasPrefix(u.Path, "/docs/") {
		return signozerrors.NewInputValidationError("unsupported_path", "URL must be under /docs/")
	}
	return nil
}

func searchDocs(ctx context.Context, sitemapURL, cachePath, query string, limit int, refreshIndex bool) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 5
	}
	urls, err := loadDocsURLs(ctx, sitemapURL, cachePath, refreshIndex)
	if err != nil {
		return nil, err
	}

	tokens := queryTokens(query)
	scored := make([]map[string]any, 0, len(urls))
	for _, item := range urls {
		score := scoreDocURL(item, tokens)
		if score <= 0 {
			continue
		}
		scored = append(scored, map[string]any{
			"url":   item,
			"title": docsTitleFromURL(item),
			"score": score,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		si, _ := scored[i]["score"].(int)
		sj, _ := scored[j]["score"].(int)
		if si == sj {
			ui, _ := scored[i]["url"].(string)
			uj, _ := scored[j]["url"].(string)
			return ui < uj
		}
		return si > sj
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

func loadDocsURLs(ctx context.Context, sitemapURL, cachePath string, refresh bool) ([]string, error) {
	cacheKey := sitemapURL + "|" + cachePath
	if !refresh {
		docsIndexMu.RLock()
		if urls, ok := docsIndexMem[cacheKey]; ok && len(urls) > 0 {
			docsIndexMu.RUnlock()
			return append([]string(nil), urls...), nil
		}
		docsIndexMu.RUnlock()

		if urls, ok := readDocsIndexDisk(cachePath, sitemapURL); ok && len(urls) > 0 {
			docsIndexMu.Lock()
			docsIndexMem[cacheKey] = append([]string(nil), urls...)
			docsIndexMu.Unlock()
			return urls, nil
		}
	}

	urls, err := fetchDocsURLs(ctx, sitemapURL)
	if err != nil {
		return nil, err
	}
	docsIndexMu.Lock()
	docsIndexMem[cacheKey] = append([]string(nil), urls...)
	docsIndexMu.Unlock()
	_ = writeDocsIndexDisk(cachePath, sitemapURL, urls)
	return urls, nil
}

func fetchDocsURLs(ctx context.Context, sitemapURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch sitemap: status=%d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var urlSet sitemapURLSet
	if err := xml.Unmarshal(body, &urlSet); err != nil {
		return nil, fmt.Errorf("invalid sitemap XML: %w", err)
	}
	out := make([]string, 0, len(urlSet.URLs))
	for _, u := range urlSet.URLs {
		if strings.Contains(u.Loc, "/docs/") {
			out = append(out, strings.TrimSpace(u.Loc))
		}
	}
	return out, nil
}

func readDocsIndexDisk(cachePath, sitemapURL string) ([]string, bool) {
	raw, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false
	}
	var cached docsIndexCacheFile
	if err := json.Unmarshal(raw, &cached); err != nil {
		return nil, false
	}
	if cached.Format != "signozctl.docs.index.v1" || cached.SitemapURL != sitemapURL || len(cached.URLs) == 0 {
		return nil, false
	}
	return cached.URLs, true
}

func writeDocsIndexDisk(cachePath, sitemapURL string, urls []string) error {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	payload := docsIndexCacheFile{
		Format:     "signozctl.docs.index.v1",
		SitemapURL: sitemapURL,
		UpdatedAt:  time.Now().Unix(),
		URLs:       urls,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, raw, 0o600)
}

func resetDocsSearchCache() {
	docsIndexMu.Lock()
	defer docsIndexMu.Unlock()
	docsIndexMem = map[string][]string{}
}

func queryTokens(query string) []string {
	raw := strings.Fields(strings.ToLower(query))
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.Trim(t, ".,:;/\\-_")
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func docsTitleFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	base := path.Base(strings.TrimSuffix(u.Path, "/"))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.TrimSpace(base)
	if base == "" || base == "docs" {
		return "SigNoz Docs"
	}
	return strings.Title(base)
}

func scoreDocURL(raw string, tokens []string) int {
	if len(tokens) == 0 {
		return 0
	}
	lower := strings.ToLower(raw)
	score := 0
	matched := 0
	for _, t := range tokens {
		if strings.Contains(lower, t) {
			score += 3
			matched++
		}
	}
	if matched == len(tokens) {
		score += 5
	}
	if strings.Contains(lower, "/docs/") {
		score += 1
	}
	return score
}

func fetchDocsMarkdown(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to fetch docs URL: status=%d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	content := string(body)
	return htmlToMarkdown(content), nil
}

func htmlToMarkdown(s string) string {
	reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	s = reScript.ReplaceAllString(s, "")
	reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	s = reStyle.ReplaceAllString(s, "")
	reTagBreak := regexp.MustCompile(`(?i)</(h1|h2|h3|h4|h5|h6|p|div|section|article|li|ul|ol|main|pre|code|tr)>`)
	s = reTagBreak.ReplaceAllString(s, "\n")
	reLi := regexp.MustCompile(`(?i)<li[^>]*>`)
	s = reLi.ReplaceAllString(s, "- ")
	reTags := regexp.MustCompile(`(?is)<[^>]+>`)
	s = reTags.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	reBlank := regexp.MustCompile(`\n{3,}`)
	s = reBlank.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
