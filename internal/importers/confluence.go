package importers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ConfluenceConfig holds configuration for a Confluence connection.
type ConfluenceConfig struct {
	BaseURL  string `json:"base_url"`
	Username string `json:"username"`
	APIToken string `json:"api_token"`
	SpaceKey string `json:"space_key"`
}

// ConfluenceClient provides access to the Confluence REST API.
type ConfluenceClient struct {
	config     ConfluenceConfig
	httpClient *http.Client
}

// NewConfluenceClient creates a new Confluence API client.
func NewConfluenceClient(config ConfluenceConfig) *ConfluenceClient {
	return &ConfluenceClient{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// confluenceSearchResult represents the Confluence search API response.
type confluenceSearchResult struct {
	Results []confluencePage `json:"results"`
	Size    int              `json:"size"`
}

type confluencePage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	Version struct {
		When string `json:"when"`
	} `json:"version"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

// FetchPages fetches all pages from the configured Confluence space.
func (c *ConfluenceClient) FetchPages() ([]ImportedItem, error) {
	endpoint := fmt.Sprintf("%s/rest/api/content?spaceKey=%s&expand=body.storage,version&limit=50",
		strings.TrimRight(c.config.BaseURL, "/"),
		url.QueryEscape(c.config.SpaceKey),
	)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(c.config.Username, c.config.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching pages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("confluence API error (%d): %s", resp.StatusCode, string(body))
	}

	var result confluenceSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	var items []ImportedItem
	sixMonthsAgo := time.Now().AddDate(0, -6, 0)

	for _, page := range result.Results {
		content := htmlToPlainText(page.Body.Storage.Value)
		modified, _ := time.Parse(time.RFC3339, page.Version.When)

		item := ImportedItem{
			Title:        page.Title,
			Content:      content,
			SourceURL:    c.config.BaseURL + page.Links.WebUI,
			LastModified: modified,
		}

		if !modified.IsZero() && modified.Before(sixMonthsAgo) {
			item.Stale = true
			item.StaleReason = fmt.Sprintf("Last updated %s (over 6 months ago)", modified.Format("Jan 2006"))
		}

		items = append(items, item)
	}

	return items, nil
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// htmlToPlainText does a basic HTML-to-text conversion.
func htmlToPlainText(html string) string {
	// Replace common block elements with newlines.
	text := strings.ReplaceAll(html, "<br>", "\n")
	text = strings.ReplaceAll(html, "<br/>", "\n")
	text = strings.ReplaceAll(text, "</p>", "\n\n")
	text = strings.ReplaceAll(text, "</div>", "\n")
	text = strings.ReplaceAll(text, "</li>", "\n")

	// Strip all remaining tags.
	text = htmlTagRegex.ReplaceAllString(text, "")

	// Clean up whitespace.
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return strings.Join(cleaned, "\n")
}
