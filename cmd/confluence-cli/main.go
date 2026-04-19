package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"confluence-cli/internal/confluence"
)

// buildVersion is set at build time via -ldflags="-X main.buildVersion=x.y.z".
var buildVersion = "dev"

// maxInputBytes caps the size of Markdown input to prevent accidental OOM.
const maxInputBytes = 10 * 1024 * 1024 // 10 MB

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcmd := os.Args[1]

	// Handle help and offline commands before loading config.
	switch subcmd {
	case "version", "-version", "--version":
		fmt.Printf("confluence-cli %s\n", buildVersion)
		return
	case "help", "-h", "--help":
		printUsage()
		return
	case "to-storage":
		// Offline conversion — no Confluence config needed.
		runToStorage(os.Args[2:])
		return
	case "config-set":
		// Config file writer — no Confluence config needed.
		runConfigSet(os.Args[2:])
		return
	case "get-page", "search", "get-space", "list-spaces", "get-children", "update-page",
		"create-page", "list-attachments", "upload-attachment":
		// Valid subcommand — continue to config loading below.
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n", subcmd)
		printUsage()
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := loadConfig()
	client := confluence.NewClient(cfg)

	switch subcmd {
	case "get-page":
		runGetPage(ctx, client, os.Args[2:])
	case "search":
		runSearch(ctx, client, os.Args[2:])
	case "get-space":
		runGetSpace(ctx, client, os.Args[2:])
	case "list-spaces":
		runListSpaces(ctx, client, os.Args[2:])
	case "get-children":
		runGetChildren(ctx, client, os.Args[2:])
	case "update-page":
		runUpdatePage(ctx, client, os.Args[2:])
	case "create-page":
		runCreatePage(ctx, client, os.Args[2:])
	case "list-attachments":
		runListAttachments(ctx, client, os.Args[2:])
	case "upload-attachment":
		runUploadAttachment(ctx, client, os.Args[2:])
	}
}

func loadConfig() confluence.Config {
	fileCfg := loadConfigFile()

	getval := func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fileCfg[key]
	}

	baseURL := getval("CONFLUENCE_BASE_URL")
	email := getval("CONFLUENCE_EMAIL")
	token := getval("CONFLUENCE_API_TOKEN")

	var missing []string
	if baseURL == "" {
		missing = append(missing, "CONFLUENCE_BASE_URL")
	}
	if email == "" {
		missing = append(missing, "CONFLUENCE_EMAIL")
	}
	if token == "" {
		missing = append(missing, "CONFLUENCE_API_TOKEN")
	}
	if len(missing) > 0 {
		fatal("missing required config: set %s via environment variable or ~/.confluence-cli", strings.Join(missing, ", "))
	}

	baseURL = strings.TrimRight(baseURL, "/")

	timeout := 30 * time.Second
	if s := os.Getenv("CONFLUENCE_TIMEOUT"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			fatal("invalid CONFLUENCE_TIMEOUT %q: %v", s, err)
		}
		timeout = d
	}

	return confluence.Config{
		BaseURL:  baseURL,
		Email:    email,
		APIToken: token,
		Timeout:  timeout,
	}
}

// loadConfigFile reads KEY=VALUE pairs from .confluence-cli (current dir preferred,
// then home dir). Returns an empty map if no file is found.
func loadConfigFile() map[string]string {
	candidates := []string{".confluence-cli"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".confluence-cli"))
	}
	for _, path := range candidates {
		if cfg := loadConfigFileFrom(path); cfg != nil {
			return cfg
		}
	}
	return nil
}

// loadConfigFileFrom parses KEY=VALUE pairs from a single config file path.
// Returns nil if the file does not exist or cannot be read.
func loadConfigFileFrom(path string) map[string]string {
	f, err := os.Open(path) //nolint:gosec // intentional config file read
	if err != nil {
		return nil
	}
	defer f.Close()

	cfg := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		cfg[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return cfg
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `confluence-cli — Read and edit Confluence pages from Claude Code

Usage:
  confluence-cli <subcommand> [flags]

Subcommands:
  get-page           Fetch a page by ID or URL
  search             Search pages by CQL query
  get-space          Get space info by key
  list-spaces        List available spaces
  get-children       Get child pages of a parent page
  update-page        Update a page from Markdown (with frontmatter metadata)
  create-page        Create a new page from Markdown
  list-attachments   List attachments on a page
  upload-attachment  Upload a file as a page attachment
  to-storage         Convert Markdown (stdin or file) to Confluence storage format
  config-set         Write credentials to ~/.confluence-cli
  version            Print version information

Config (env vars or ~/.confluence-cli file):
  CONFLUENCE_BASE_URL   e.g. https://garykww.atlassian.net
  CONFLUENCE_EMAIL      Your Atlassian account email
  CONFLUENCE_API_TOKEN  Atlassian API token
  CONFLUENCE_TIMEOUT    HTTP timeout, e.g. 60s (default: 30s, env only)

Examples:
  confluence-cli config-set -base-url https://company.atlassian.net -email me@co.com -token TOKEN
  confluence-cli get-page -id 131166
  confluence-cli search -cql 'type=page AND space=TEST' -limit 5
  confluence-cli create-page -space TEST -title "New Page" -file page.md
  confluence-cli list-attachments -id 131166 -human
  confluence-cli upload-attachment -id 131166 -file diagram.png
  confluence-cli get-page -id 123 > page.md && vim page.md && confluence-cli update-page -file page.md

Run "confluence-cli <subcommand> -h" for subcommand-specific help.
`)
}

func runGetPage(ctx context.Context, client *confluence.Client, args []string) {
	fs := flag.NewFlagSet("get-page", flag.ExitOnError)
	id := fs.String("id", "", "Page ID")
	rawURL := fs.String("url", "", "Full Confluence page URL (extracts ID automatically)")
	expand := fs.String("expand", "", "Comma-separated expand fields (default: space,history,body.storage,body.view,version,ancestors)")
	human := fs.Bool("human", false, "Human-readable output (metadata + raw HTML body)")
	jsonOut := fs.Bool("json", false, "JSON output (full API response)")
	fs.Parse(args) //nolint:errcheck

	pageID := *id
	if pageID == "" && *rawURL != "" {
		var err error
		pageID, err = confluence.ExtractPageIDFromURL(*rawURL)
		if err != nil {
			fatal("%v", err)
		}
	}
	if pageID == "" {
		fatal("provide -id or -url")
	}

	page, err := client.GetPage(ctx, pageID, *expand)
	if err != nil {
		fatal("get-page: %v", err)
	}

	switch {
	case *jsonOut:
		printJSON(page)
	case *human:
		printPage(page)
	default:
		printPageMarkdown(page)
	}
}

func runSearch(ctx context.Context, client *confluence.Client, args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	cql := fs.String("cql", "", "CQL query string (required)")
	limit := fs.Int("limit", 10, "Maximum results to return")
	start := fs.Int("start", 0, "Starting index for pagination")
	human := fs.Bool("human", false, "Human-readable output instead of JSON")
	fs.Parse(args) //nolint:errcheck

	if *cql == "" {
		fatal("search requires -cql flag")
	}

	result, err := client.SearchContent(ctx, *cql, *limit, *start)
	if err != nil {
		fatal("search: %v", err)
	}

	if *human {
		printSearchResults(result)
	} else {
		printJSON(result)
	}
}

func runGetSpace(ctx context.Context, client *confluence.Client, args []string) {
	fs := flag.NewFlagSet("get-space", flag.ExitOnError)
	key := fs.String("key", "", "Space key (required)")
	human := fs.Bool("human", false, "Human-readable output instead of JSON")
	fs.Parse(args) //nolint:errcheck

	if *key == "" {
		fatal("get-space requires -key flag")
	}

	space, err := client.GetSpace(ctx, *key)
	if err != nil {
		fatal("get-space: %v", err)
	}

	if *human {
		printSpace(space)
	} else {
		printJSON(space)
	}
}

func runListSpaces(ctx context.Context, client *confluence.Client, args []string) {
	fs := flag.NewFlagSet("list-spaces", flag.ExitOnError)
	limit := fs.Int("limit", 10, "Maximum spaces to return")
	start := fs.Int("start", 0, "Starting index for pagination")
	human := fs.Bool("human", false, "Human-readable output instead of JSON")
	fs.Parse(args) //nolint:errcheck

	list, err := client.ListSpaces(ctx, *limit, *start)
	if err != nil {
		fatal("list-spaces: %v", err)
	}

	if *human {
		printSpaceList(list)
	} else {
		printJSON(list)
	}
}

func runGetChildren(ctx context.Context, client *confluence.Client, args []string) {
	fs := flag.NewFlagSet("get-children", flag.ExitOnError)
	id := fs.String("id", "", "Parent page ID (required)")
	limit := fs.Int("limit", 25, "Maximum child pages to return")
	human := fs.Bool("human", false, "Human-readable output instead of JSON")
	fs.Parse(args) //nolint:errcheck

	if *id == "" {
		fatal("get-children requires -id flag")
	}

	children, err := client.GetChildPages(ctx, *id, *limit)
	if err != nil {
		fatal("get-children: %v", err)
	}

	if *human {
		printChildPages(children)
	} else {
		printJSON(children)
	}
}

func runUpdatePage(ctx context.Context, client *confluence.Client, args []string) {
	fs := flag.NewFlagSet("update-page", flag.ExitOnError)
	file := fs.String("file", "", "Markdown file with frontmatter (reads stdin if omitted)")
	title := fs.String("title", "", "Override page title from frontmatter")
	fs.Parse(args) //nolint:errcheck

	input, err := readInput(*file)
	if err != nil {
		fatal("reading input: %v", err)
	}
	if len(input) == 0 {
		fatal("no input provided; pass -file or pipe Markdown to stdin")
	}

	md := string(input)
	meta, body := confluence.ParseFrontmatter(md)

	if meta.ID == "" {
		fatal("frontmatter missing 'id' field — cannot determine which page to update")
	}
	if meta.Version == 0 {
		fatal("frontmatter missing 'version' field — required to avoid conflicts")
	}

	pageTitle := meta.Title
	if *title != "" {
		pageTitle = *title
	}
	if pageTitle == "" {
		fatal("no title found in frontmatter or -title flag")
	}

	storage := confluence.MarkdownToStorage(body)

	page, err := client.UpdatePage(ctx, meta.ID, pageTitle, meta.Version, storage)
	if err != nil {
		var conflictErr *confluence.ConflictError
		if errors.As(err, &conflictErr) {
			fatal("update-page: %v\nHint: run 'confluence-cli get-page -id %s' to refresh the page version", err, meta.ID)
		}
		fatal("update-page: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Updated page %q (id:%s) to version %d\n", page.Title, page.ID, page.Version.Number)
	printPageMarkdown(page)
}

func runCreatePage(ctx context.Context, client *confluence.Client, args []string) {
	fs := flag.NewFlagSet("create-page", flag.ExitOnError)
	file := fs.String("file", "", "Markdown file with optional frontmatter (reads stdin if omitted)")
	title := fs.String("title", "", "Page title (overrides frontmatter)")
	space := fs.String("space", "", "Space key (overrides frontmatter)")
	parent := fs.String("parent", "", "Parent page ID (creates as child; overrides frontmatter parent_id)")
	fs.Parse(args) //nolint:errcheck

	input, err := readInput(*file)
	if err != nil {
		fatal("reading input: %v", err)
	}

	md := string(input)
	meta, body := confluence.ParseFrontmatter(md)

	pageTitle := firstNonEmpty(*title, meta.Title)
	spaceKey := firstNonEmpty(*space, meta.Space)
	parentID := firstNonEmpty(*parent, meta.ParentID)

	if pageTitle == "" {
		fatal("provide -title or set 'title' in frontmatter")
	}
	if spaceKey == "" {
		fatal("provide -space or set 'space' in frontmatter")
	}

	storage := confluence.MarkdownToStorage(body)

	page, err := client.CreatePage(ctx, spaceKey, pageTitle, parentID, storage)
	if err != nil {
		fatal("create-page: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Created page %q (id:%s) in space %s\n", page.Title, page.ID, spaceKey)
	printPageMarkdown(page)
}

func runListAttachments(ctx context.Context, client *confluence.Client, args []string) {
	fs := flag.NewFlagSet("list-attachments", flag.ExitOnError)
	id := fs.String("id", "", "Page ID (required)")
	rawURL := fs.String("url", "", "Full Confluence page URL (extracts ID automatically)")
	limit := fs.Int("limit", 25, "Maximum attachments to return")
	human := fs.Bool("human", false, "Human-readable output instead of JSON")
	fs.Parse(args) //nolint:errcheck

	pageID := *id
	if pageID == "" && *rawURL != "" {
		var err error
		pageID, err = confluence.ExtractPageIDFromURL(*rawURL)
		if err != nil {
			fatal("%v", err)
		}
	}
	if pageID == "" {
		fatal("provide -id or -url")
	}

	list, err := client.ListAttachments(ctx, pageID, *limit)
	if err != nil {
		fatal("list-attachments: %v", err)
	}

	if *human {
		printAttachmentList(list)
	} else {
		printJSON(list)
	}
}

func runUploadAttachment(ctx context.Context, client *confluence.Client, args []string) {
	fs := flag.NewFlagSet("upload-attachment", flag.ExitOnError)
	id := fs.String("id", "", "Target page ID (required)")
	rawURL := fs.String("url", "", "Full Confluence page URL (extracts ID automatically)")
	file := fs.String("file", "", "Local file to upload (required)")
	human := fs.Bool("human", false, "Human-readable output instead of JSON")
	fs.Parse(args) //nolint:errcheck

	pageID := *id
	if pageID == "" && *rawURL != "" {
		var err error
		pageID, err = confluence.ExtractPageIDFromURL(*rawURL)
		if err != nil {
			fatal("%v", err)
		}
	}
	if pageID == "" {
		fatal("provide -id or -url")
	}
	if *file == "" {
		fatal("upload-attachment requires -file")
	}

	attachment, err := client.UploadAttachment(ctx, pageID, *file)
	if err != nil {
		fatal("upload-attachment: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Uploaded %q to page id:%s\n", attachment.Title, pageID)
	if *human {
		printAttachment(attachment)
	} else {
		printJSON(attachment)
	}
}

func runConfigSet(args []string) {
	fs := flag.NewFlagSet("config-set", flag.ExitOnError)
	baseURL := fs.String("base-url", "", "Confluence base URL (required)")
	email := fs.String("email", "", "Atlassian account email (required)")
	token := fs.String("token", "", "Atlassian API token (required)")
	fs.Parse(args) //nolint:errcheck

	if *baseURL == "" || *email == "" || *token == "" {
		fatal("config-set requires -base-url, -email, and -token")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fatal("cannot determine home directory: %v", err)
	}
	cfgPath := filepath.Join(home, ".confluence-cli")

	content := fmt.Sprintf("CONFLUENCE_BASE_URL=%s\nCONFLUENCE_EMAIL=%s\nCONFLUENCE_API_TOKEN=%s\n",
		strings.TrimRight(*baseURL, "/"), *email, *token)

	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		fatal("writing config file: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Config written to %s\n", cfgPath)
}

// firstNonEmpty returns the first non-empty string argument.
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func runToStorage(args []string) {
	fs := flag.NewFlagSet("to-storage", flag.ExitOnError)
	file := fs.String("file", "", "Read Markdown from file instead of stdin")
	fs.Parse(args) //nolint:errcheck

	input, err := readInput(*file)
	if err != nil {
		fatal("reading input: %v", err)
	}
	if len(input) == 0 {
		fatal("no input provided; pass -file or pipe Markdown to stdin")
	}

	fmt.Println(confluence.MarkdownToStorage(string(input)))
}

// readInput reads from a file path or stdin, enforcing maxInputBytes.
func readInput(filePath string) ([]byte, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		if len(data) > maxInputBytes {
			return nil, fmt.Errorf("file exceeds maximum input size of %d MB", maxInputBytes/1024/1024)
		}
		return data, nil
	}

	r := io.LimitReader(os.Stdin, int64(maxInputBytes)+1)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) > maxInputBytes {
		return nil, fmt.Errorf("stdin input exceeds maximum size of %d MB", maxInputBytes/1024/1024)
	}
	return data, nil
}

// fatal prints an error message to stderr and exits with code 1.
func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
