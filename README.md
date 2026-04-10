# confluence-cli

A lightweight Go CLI built for use with **Claude Code** to view and edit Confluence pages efficiently — no dependencies beyond the Go standard library.

Give Claude direct access to your Confluence content: fetch pages as Markdown, search with CQL, and push edits back — all from within a Claude Code session.

## Quick Start

```bash
# Clone and build
git clone git@github.com:garykww/confluence-cli.git
cd confluence-cli
make build

# Set credentials
export CONFLUENCE_BASE_URL=https://garykww.atlassian.net
export CONFLUENCE_EMAIL=your.name@example.com
export CONFLUENCE_API_TOKEN=your_atlassian_token

# Run
./confluence-cli list-spaces -limit 5
```

## Authentication Setup

confluence-cli uses **Atlassian API tokens** for Basic authentication — your Atlassian account password will not work.

### 1. Generate an API token

1. Log in to [https://id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens)
2. Click **Create API token**
3. Give it a label (e.g. `confluence-cli`) and click **Create**
4. Copy the token — it is only shown once

### 2. Set environment variables

The recommended approach is to add the variables to your shell profile so they persist across sessions:

```bash
# Add to ~/.zshrc or ~/.bashrc
export CONFLUENCE_BASE_URL=https://garykww.atlassian.net
export CONFLUENCE_EMAIL=your.name@example.com
export CONFLUENCE_API_TOKEN=your_token_here
```

Then reload your shell:

```bash
source ~/.zshrc   # or source ~/.bashrc
```

### 3. Verify the setup

```bash
./confluence-cli list-spaces -limit 3 -human
```

If you see a list of spaces, authentication is working. A `401` error means the token or email is wrong. A `missing required environment variables` error means one or more env vars are not set.

> **Security note:** Never commit your API token to source control. Use a password manager or your OS keychain to store it securely.

## Environment Variables

| Variable               | Description                                     | Required |
|------------------------|-------------------------------------------------|----------|
| `CONFLUENCE_BASE_URL`  | Base URL, e.g. `https://garykww.atlassian.net` | Yes      |
| `CONFLUENCE_EMAIL`     | Your Atlassian account email                    | Yes      |
| `CONFLUENCE_API_TOKEN` | Atlassian API token (not your password)         | Yes      |
| `CONFLUENCE_TIMEOUT`   | HTTP timeout, e.g. `60s` (default: `30s`)       | No       |

The `to-storage` subcommand is offline-only and requires none of them.

## Subcommands

| Subcommand     | Description                                                   |
|----------------|---------------------------------------------------------------|
| `get-page`     | Fetch a page by ID or full URL                                |
| `search`       | Search pages using a CQL query                                |
| `get-space`    | Get space details by key                                      |
| `list-spaces`  | List available spaces                                         |
| `get-children` | List child pages of a parent page                             |
| `update-page`  | Update a page from a Markdown file (or stdin)                 |
| `to-storage`   | Convert Markdown to Confluence storage format XHTML (offline) |
| `version`      | Print version information                                     |

Run `confluence-cli <subcommand> -h` for flag details on any subcommand.

---

### `get-page`

Fetch a single page. Default output is **Markdown with YAML frontmatter** (suitable for piping into `update-page`).

```
Flags:
  -id string     Page ID
  -url string    Full Confluence page URL (ID extracted automatically)
  -expand string Comma-separated expand fields (default: space,history,body.storage,body.view,version,ancestors)
  -human         Human-readable summary (title, space, version, body)
  -json          Full JSON response from the Confluence API
```

```bash
# Markdown output (default) — includes frontmatter with id, title, space, version
confluence-cli get-page -id 131166

# Human-readable summary
confluence-cli get-page -id 131166 -human

# Full API JSON
confluence-cli get-page -id 131166 -json

# Extract page ID from a URL automatically
confluence-cli get-page -url "https://garykww.atlassian.net/wiki/spaces/TEST/pages/131166"
```

---

### `search`

Search pages using [CQL (Confluence Query Language)](https://developer.atlassian.com/cloud/confluence/advanced-searching-using-cql/).

```
Flags:
  -cql string    CQL query string (required)
  -limit int     Max results (default: 10)
  -start int     Pagination offset (default: 0)
  -human         Human-readable table instead of JSON
```

```bash
# Search in a specific space
confluence-cli search -cql 'type=page AND space=TEST' -limit 5

# Search by title keyword
confluence-cli search -cql 'type=page AND title~"deployment"' -human

# Pages modified in the last 7 days
confluence-cli search -cql 'type=page AND lastModified >= now("-7d")' -limit 20

# Paginate through results
confluence-cli search -cql 'type=page AND space=TEST' -limit 10 -start 10
```

---

### `get-space`

```
Flags:
  -key string    Space key (required)
  -human         Human-readable output instead of JSON
```

```bash
confluence-cli get-space -key TEST
confluence-cli get-space -key TEST -human
```

---

### `list-spaces`

```
Flags:
  -limit int     Max spaces to return (default: 10)
  -start int     Pagination offset (default: 0)
  -human         Human-readable table instead of JSON
```

```bash
confluence-cli list-spaces -limit 20 -human
```

---

### `get-children`

```
Flags:
  -id string     Parent page ID (required)
  -limit int     Max child pages (default: 25)
  -human         Human-readable table instead of JSON
```

```bash
confluence-cli get-children -id 131166 -human
```

---

### `update-page`

Update an existing Confluence page from a Markdown file. The file must contain **YAML frontmatter** with `id` and `version` fields (both are emitted by `get-page` automatically).

```
Flags:
  -file string   Markdown file to read (reads stdin if omitted)
  -title string  Override the page title from frontmatter
```

The version is auto-incremented — pass the current version number, not `current + 1`.
If the version is stale (someone else edited the page), a conflict error is returned with a hint to re-fetch.

```bash
# Edit-in-place roundtrip
confluence-cli get-page -id 131166 > page.md
vim page.md
confluence-cli update-page -file page.md
```

**Frontmatter format** (produced by `get-page`):

```yaml
---
id: "131166"
title: "My Page Title"
space: "TEST"
version: 12
---

Your Markdown content here...
```

---

### `to-storage`

Offline conversion from Markdown to **Confluence storage format** XHTML. No network calls, no credentials required.

```
Flags:
  -file string   Read Markdown from file instead of stdin
```

```bash
# From stdin
echo "# Hello\n\nSome **bold** text." | confluence-cli to-storage

# From file
confluence-cli to-storage -file page.md

# Preview what update-page would send
confluence-cli get-page -id 131166 | confluence-cli to-storage
```

**Supported Markdown features:**

| Markdown | Confluence Storage Output |
|----------|--------------------------|
| `# H1` – `###### H6` | `<h1>` – `<h6>` |
| `**bold**` | `<strong>` |
| `*italic*` | `<em>` |
| `` `code` `` | `<code>` |
| `~~strike~~` | `<del>` |
| `[text](url)` | `<a href="...">` |
| `![alt](url)` | `<ac:image><ri:url .../>` |
| `![alt](attachment:file)` | `<ac:image><ri:attachment .../>` |
| ` ```lang ` code block | `ac:structured-macro` code macro |
| `- item` / `1. item` | `<ul>` / `<ol>` |
| `\| table \|` | `<table>` with `<thead>` / `<tbody>` |
| `---` | `<hr />` |
| `> quote` | `<blockquote>` |
| `> **Info:** text` | `ac:structured-macro` info macro |
| `> **Note:** text` | `ac:structured-macro` note macro |
| `> **Warning:** text` | `ac:structured-macro` warning macro |
| `> **Tip:** text` | `ac:structured-macro` tip macro |

---

## Edit Roundtrip Workflow

```bash
# 1. Fetch the page as Markdown (frontmatter carries id + version)
confluence-cli get-page -id 131166 > my-page.md

# 2. Edit in your favourite editor
vim my-page.md

# 3. Push the update back — version is auto-incremented
confluence-cli update-page -file my-page.md
# → Updated "My Page Title" (id:131166) to version 13
```

---

## Using with Claude Code (CLAUDE.md)

To let Claude read and edit Confluence pages during a session, add this to your `CLAUDE.md`. Claude will use the CLI via Bash to fetch, search, and update pages without any manual steps.

```markdown
- **Confluence**: Use the CLI (`<path-to>/confluence-cli`) via Bash to view and edit pages — fetch with `get-page`, search with `search`, push changes with `update-page`.
```

---

## Development

### Running Tests

```bash
make test

# Verbose
go test -v ./...

# Specific test
go test -run TestMarkdownToStorage ./internal/confluence/
```

### Building

```bash
# Current platform
make build

# All platforms (output in dist/)
make build-all
```

### Linting

```bash
make lint
```

---

## Project Structure

```
confluence-cli/
├── cmd/
│   └── confluence-cli/
│       ├── main.go          # Entry point, subcommand dispatch, config loading
│       ├── main_test.go     # Config loading tests
│       └── output.go        # Terminal formatters (JSON, human-readable, Markdown)
├── internal/
│   └── confluence/
│       ├── client.go        # HTTP client, auth, all Confluence API methods and types
│       ├── client_test.go   # Client and URL extraction tests
│       ├── convert.go       # HTML ↔ Markdown / Markdown → Storage converters
│       └── convert_test.go  # Conversion tests
├── .github/
│   └── workflows/
│       └── ci.yml           # CI: test, lint, cross-platform build, release
├── .gitignore
├── .golangci.yml            # Linter configuration
├── .goreleaser.yaml         # Release configuration
├── go.mod
└── Makefile
```
