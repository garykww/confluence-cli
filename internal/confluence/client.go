package confluence

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// Config holds Confluence connection settings loaded from environment variables.
type Config struct {
	BaseURL  string
	Email    string
	APIToken string
	Timeout  time.Duration // HTTP client timeout; defaults to 30s if zero
}

// ConfluencePage represents a single Confluence page from the REST API.
type ConfluencePage struct {
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Status    string        `json:"status"`
	Title     string        `json:"title"`
	Space     *SpaceRef     `json:"space,omitempty"`
	History   *History      `json:"history,omitempty"`
	Version   *Version      `json:"version,omitempty"`
	Body      *Body         `json:"body,omitempty"`
	Ancestors []AncestorRef `json:"ancestors,omitempty"`
}

type SpaceRef struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type History struct {
	CreatedDate string `json:"createdDate"`
	CreatedBy   *User  `json:"createdBy,omitempty"`
}

type User struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
}

type Version struct {
	Number int    `json:"number"`
	When   string `json:"when"`
	By     *User  `json:"by,omitempty"`
}

type Body struct {
	Storage *BodyContent `json:"storage,omitempty"`
	View    *BodyContent `json:"view,omitempty"`
}

type BodyContent struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}

type AncestorRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// SearchResult is the response from the content/search endpoint.
type SearchResult struct {
	Results []ConfluencePage `json:"results"`
	Size    int              `json:"size"`
	Limit   int              `json:"limit"`
	Start   int              `json:"start"`
}

// Space represents a Confluence space.
type Space struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Status      string       `json:"status"`
	Description *Description `json:"description,omitempty"`
	Homepage    *HomepageRef `json:"homepage,omitempty"`
}

type Description struct {
	Plain *PlainValue `json:"plain,omitempty"`
}

type PlainValue struct {
	Value string `json:"value"`
}

type HomepageRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// SpaceList is the response from the space listing endpoint.
type SpaceList struct {
	Results []Space `json:"results"`
	Size    int     `json:"size"`
	Limit   int     `json:"limit"`
	Start   int     `json:"start"`
}

// ChildPages is the response from the child page listing endpoint.
type ChildPages struct {
	Results []ConfluencePage `json:"results"`
	Size    int              `json:"size"`
	Limit   int              `json:"limit"`
	Start   int              `json:"start"`
}

// ConflictError is returned by write operations when the server responds with 409.
// This typically means the page version is stale; re-fetch the page and retry.
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("version conflict (re-fetch the page and retry): %s", e.Message)
}

// Client wraps an HTTP client with Confluence authentication.
type Client struct {
	httpClient *http.Client
	baseURL    string
	authHeader string
}

// NewClient creates a Client configured with Basic auth for the Confluence REST API.
func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	creds := cfg.Email + ":" + cfg.APIToken
	auth := base64.StdEncoding.EncodeToString([]byte(creds))
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    cfg.BaseURL + "/wiki/rest/api",
		authHeader: "Basic " + auth,
	}
}

// doGet performs an authenticated GET with automatic retry on transient failures.
// Retries up to 3 times on network errors, 429 Too Many Requests, and 5xx responses,
// with 500 ms delay between attempts.
func (c *Client) doGet(ctx context.Context, path string, params url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("GET %s cancelled: %w", path, ctx.Err())
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("building GET request for %s: %w", path, err)
		}
		req.Header.Set("Authorization", c.authHeader)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("GET %s: %w", path, err)
			continue // retry on network error
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response from GET %s: %w", path, err)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}

		apiErr := fmt.Errorf("GET %s: API returned %d: %s", path, resp.StatusCode, string(body))
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = apiErr
			continue // retry on rate limit or server error
		}
		return nil, apiErr // don't retry client errors (4xx)
	}
	return nil, lastErr
}

// doPut performs an authenticated PUT request with a JSON body.
func (c *Client) doPut(ctx context.Context, path string, payload any) ([]byte, error) {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding request body for PUT %s: %w", path, err)
	}

	u := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("building PUT request for %s: %w", path, err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PUT %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from PUT %s: %w", path, err)
	}

	if resp.StatusCode == http.StatusConflict {
		return nil, &ConflictError{Message: string(body)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("PUT %s: API returned %d: %s", path, resp.StatusCode, string(body))
	}
	return body, nil
}

// GetPage fetches a single Confluence page by ID with optional expand fields.
func (c *Client) GetPage(ctx context.Context, id string, expand string) (*ConfluencePage, error) {
	if id == "" {
		return nil, fmt.Errorf("page ID is required")
	}
	if expand == "" {
		expand = "space,history,body.storage,body.view,version,ancestors"
	}
	params := url.Values{"expand": {expand}}

	data, err := c.doGet(ctx, "/content/"+id, params)
	if err != nil {
		return nil, err
	}

	var page ConfluencePage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parsing page response: %w", err)
	}
	return &page, nil
}

// SearchContent searches Confluence pages using CQL.
func (c *Client) SearchContent(ctx context.Context, cql string, limit, start int) (*SearchResult, error) {
	if cql == "" {
		return nil, fmt.Errorf("CQL query is required")
	}
	if limit <= 0 {
		limit = 10
	}
	if start < 0 {
		start = 0
	}
	params := url.Values{
		"cql":    {cql},
		"limit":  {fmt.Sprintf("%d", limit)},
		"start":  {fmt.Sprintf("%d", start)},
		"expand": {"space,history,body.view,version"},
	}

	data, err := c.doGet(ctx, "/content/search", params)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}
	return &result, nil
}

// GetSpace fetches a Confluence space by key.
func (c *Client) GetSpace(ctx context.Context, key string) (*Space, error) {
	if key == "" {
		return nil, fmt.Errorf("space key is required")
	}
	params := url.Values{"expand": {"description.plain,homepage"}}

	data, err := c.doGet(ctx, "/space/"+key, params)
	if err != nil {
		return nil, err
	}

	var space Space
	if err := json.Unmarshal(data, &space); err != nil {
		return nil, fmt.Errorf("parsing space response: %w", err)
	}
	return &space, nil
}

// ListSpaces returns a paginated list of Confluence spaces.
func (c *Client) ListSpaces(ctx context.Context, limit, start int) (*SpaceList, error) {
	if limit <= 0 {
		limit = 10
	}
	if start < 0 {
		start = 0
	}
	params := url.Values{
		"limit":  {fmt.Sprintf("%d", limit)},
		"start":  {fmt.Sprintf("%d", start)},
		"expand": {"description.plain,homepage"},
	}

	data, err := c.doGet(ctx, "/space", params)
	if err != nil {
		return nil, err
	}

	var list SpaceList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing space list response: %w", err)
	}
	return &list, nil
}

// GetChildPages returns the child pages of a given parent page.
func (c *Client) GetChildPages(ctx context.Context, id string, limit int) (*ChildPages, error) {
	if id == "" {
		return nil, fmt.Errorf("parent page ID is required")
	}
	if limit <= 0 {
		limit = 25
	}
	params := url.Values{
		"limit":  {fmt.Sprintf("%d", limit)},
		"expand": {"space,history,version"},
	}

	data, err := c.doGet(ctx, "/content/"+id+"/child/page", params)
	if err != nil {
		return nil, err
	}

	var children ChildPages
	if err := json.Unmarshal(data, &children); err != nil {
		return nil, fmt.Errorf("parsing child pages response: %w", err)
	}
	return &children, nil
}

// UpdatePage updates a Confluence page's title and body content.
// The version is auto-incremented (pass the current version, not current+1).
func (c *Client) UpdatePage(ctx context.Context, id, title string, version int, storageBody string) (*ConfluencePage, error) {
	if id == "" {
		return nil, fmt.Errorf("page ID is required")
	}
	if title == "" {
		return nil, fmt.Errorf("page title is required")
	}
	if version <= 0 {
		return nil, fmt.Errorf("version must be a positive integer")
	}

	payload := map[string]any{
		"type":    "page",
		"title":   title,
		"version": map[string]any{"number": version + 1},
		"body": map[string]any{
			"storage": map[string]any{
				"value":          storageBody,
				"representation": "storage",
			},
		},
	}

	data, err := c.doPut(ctx, "/content/"+id, payload)
	if err != nil {
		return nil, err
	}

	var page ConfluencePage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parsing updated page response: %w", err)
	}
	return &page, nil
}

var pageIDRegex = regexp.MustCompile(`/pages/(\d+)`)

// ExtractPageIDFromURL pulls the numeric page ID from a Confluence page URL.
func ExtractPageIDFromURL(rawURL string) (string, error) {
	matches := pageIDRegex.FindStringSubmatch(rawURL)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find page ID in URL: %s", rawURL)
	}
	return matches[1], nil
}
