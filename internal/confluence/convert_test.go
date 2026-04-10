package confluence

import (
	"strings"
	"testing"
)

// ─── ParseFrontmatter ───────────────────────────────────────

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMeta PageMeta
		wantBody string
	}{
		{
			name:     "all fields present",
			input:    "---\nid: \"123\"\ntitle: \"My Page\"\nspace: \"TEST\"\nversion: 5\n---\n\nBody here.",
			wantMeta: PageMeta{ID: "123", Title: "My Page", Space: "TEST", Version: 5},
			wantBody: "Body here.",
		},
		{
			name:     "no frontmatter",
			input:    "Just plain content",
			wantMeta: PageMeta{},
			wantBody: "Just plain content",
		},
		{
			name:     "missing closing delimiter",
			input:    "---\nid: \"123\"\nno closing fence",
			wantMeta: PageMeta{},
			wantBody: "---\nid: \"123\"\nno closing fence",
		},
		{
			// ParseFrontmatter searches for "\n---" in md[4:]. With input "---\n---\n\nBody",
			// md[4:] is "---\n\nBody" which contains no "\n---", so it falls through
			// and returns the full string as the body unchanged.
			name:     "empty frontmatter block (no inner newline-fence match)",
			input:    "---\n---\n\nBody",
			wantMeta: PageMeta{},
			wantBody: "---\n---\n\nBody",
		},
		{
			name:     "version parsed as int",
			input:    "---\nversion: 42\n---\n",
			wantMeta: PageMeta{Version: 42},
			wantBody: "",
		},
		{
			name:  "id without quotes",
			input: "---\nid: 456\ntitle: plain\n---\n",
			// values are trimmed of surrounding quotes, so bare "456" stays "456"
			wantMeta: PageMeta{ID: "456", Title: "plain"},
			wantBody: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, body := ParseFrontmatter(tt.input)
			if meta != tt.wantMeta {
				t.Errorf("meta = %+v, want %+v", meta, tt.wantMeta)
			}
			if strings.TrimSpace(body) != strings.TrimSpace(tt.wantBody) {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

// ─── HTMLToMarkdown ─────────────────────────────────────────

func TestHTMLToMarkdown_Block(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "paragraph",
			input: "<p>Hello world</p>",
			want:  "Hello world",
		},
		{
			name:  "h1",
			input: "<h1>Main Title</h1>",
			want:  "# Main Title",
		},
		{
			name:  "h3",
			input: "<h3>Sub-section</h3>",
			want:  "### Sub-section",
		},
		{
			name:  "horizontal rule",
			input: "<hr/>",
			want:  "---",
		},
		{
			name:  "blockquote",
			input: "<blockquote><p>some quote</p></blockquote>",
			want:  "> some quote",
		},
		{
			name:  "pre code block",
			input: "<pre><code>fmt.Println(\"hi\")</code></pre>",
			want:  "```\nfmt.Println(\"hi\")\n```",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "html comment stripped",
			input: "<!-- comment --><p>visible</p>",
			want:  "visible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.TrimSpace(HTMLToMarkdown(tt.input))
			if got != tt.want {
				t.Errorf("HTMLToMarkdown(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHTMLToMarkdown_Inline(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bold strong",
			input: "<p><strong>bold</strong></p>",
			want:  "**bold**",
		},
		{
			name:  "italic em",
			input: "<p><em>italic</em></p>",
			want:  "*italic*",
		},
		{
			name:  "inline code",
			input: "<p><code>myFunc()</code></p>",
			want:  "`myFunc()`",
		},
		{
			name:  "strikethrough del",
			input: "<p><del>removed</del></p>",
			want:  "~~removed~~",
		},
		{
			name:  "anchor link",
			input: `<p><a href="https://example.com">click here</a></p>`,
			want:  "[click here](https://example.com)",
		},
		{
			name:  "link with empty href renders text only",
			input: `<p><a href="">label</a></p>`,
			want:  "label",
		},
		{
			name:  "image with alt and src",
			input: `<img alt="logo" src="https://example.com/img.png"/>`,
			want:  "![logo](https://example.com/img.png)",
		},
		{
			name:  "image without src renders empty",
			input: `<img alt="logo"/>`,
			want:  "",
		},
		{
			name:  "underline passthrough (no markdown equivalent)",
			input: "<p><u>underlined</u></p>",
			want:  "underlined",
		},
		{
			// <br/> emits "  \n" but cleanMarkdown strips trailing whitespace,
			// so the two spaces are removed and the result is just "\n".
			name:  "line break in paragraph",
			input: "<p>line1<br/>line2</p>",
			want:  "line1\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.TrimSpace(HTMLToMarkdown(tt.input))
			if got != tt.want {
				t.Errorf("HTMLToMarkdown(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHTMLToMarkdown_Lists(t *testing.T) {
	t.Run("unordered list", func(t *testing.T) {
		input := "<ul><li>Item A</li><li>Item B</li></ul>"
		got := strings.TrimSpace(HTMLToMarkdown(input))
		if !strings.Contains(got, "- Item A") {
			t.Errorf("expected '- Item A', got: %q", got)
		}
		if !strings.Contains(got, "- Item B") {
			t.Errorf("expected '- Item B', got: %q", got)
		}
	})

	t.Run("ordered list", func(t *testing.T) {
		input := "<ol><li>First</li><li>Second</li></ol>"
		got := strings.TrimSpace(HTMLToMarkdown(input))
		if !strings.Contains(got, "1. First") {
			t.Errorf("expected '1. First', got: %q", got)
		}
		if !strings.Contains(got, "2. Second") {
			t.Errorf("expected '2. Second', got: %q", got)
		}
	})
}

func TestHTMLToMarkdown_Table(t *testing.T) {
	input := `<table>
		<thead><tr><th>Name</th><th>Value</th></tr></thead>
		<tbody><tr><td>foo</td><td>bar</td></tr></tbody>
	</table>`
	got := HTMLToMarkdown(input)
	if !strings.Contains(got, "| Name |") {
		t.Errorf("expected header row with 'Name', got: %q", got)
	}
	if !strings.Contains(got, "---|") {
		t.Errorf("expected separator row, got: %q", got)
	}
	if !strings.Contains(got, "| foo |") {
		t.Errorf("expected data row with 'foo', got: %q", got)
	}
}

func TestHTMLToMarkdown_ConfluenceMacros(t *testing.T) {
	t.Run("code macro with language", func(t *testing.T) {
		input := `<ac:structured-macro ac:name="code">` +
			`<ac:parameter ac:name="language">go</ac:parameter>` +
			`<ac:plain-text-body><![CDATA[fmt.Println()]]></ac:plain-text-body>` +
			`</ac:structured-macro>`
		got := strings.TrimSpace(HTMLToMarkdown(input))
		if !strings.Contains(got, "```go") {
			t.Errorf("expected ```go fence, got: %q", got)
		}
		if !strings.Contains(got, "fmt.Println()") {
			t.Errorf("expected code content, got: %q", got)
		}
	})

	t.Run("info macro", func(t *testing.T) {
		input := `<ac:structured-macro ac:name="info">` +
			`<ac:rich-text-body><p>important note</p></ac:rich-text-body>` +
			`</ac:structured-macro>`
		got := strings.TrimSpace(HTMLToMarkdown(input))
		if !strings.Contains(got, "**Info:**") {
			t.Errorf("expected **Info:** label, got: %q", got)
		}
		if !strings.Contains(got, "important note") {
			t.Errorf("expected macro body, got: %q", got)
		}
	})

	t.Run("toc macro renders empty", func(t *testing.T) {
		input := `<ac:structured-macro ac:name="toc"></ac:structured-macro>`
		got := strings.TrimSpace(HTMLToMarkdown(input))
		if got != "" {
			t.Errorf("expected empty output for toc macro, got: %q", got)
		}
	})

	t.Run("CDATA text node extracted", func(t *testing.T) {
		input := "<div><![CDATA[raw text]]></div>"
		got := HTMLToMarkdown(input)
		if !strings.Contains(got, "raw text") {
			t.Errorf("expected CDATA content, got: %q", got)
		}
	})
}

// ─── MarkdownToStorage ──────────────────────────────────────

func TestMarkdownToStorage_Blocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain paragraph",
			input: "Hello world",
			want:  "<p>Hello world</p>",
		},
		{
			name:  "h1",
			input: "# Title",
			want:  "<h1>Title</h1>",
		},
		{
			name:  "h2",
			input: "## Section",
			want:  "<h2>Section</h2>",
		},
		{
			name:  "h6",
			input: "###### Deep",
			want:  "<h6>Deep</h6>",
		},
		{
			name:  "horizontal rule ---",
			input: "---",
			want:  "<hr />",
		},
		{
			name:  "horizontal rule ***",
			input: "***",
			want:  "<hr />",
		},
		{
			name:  "blockquote",
			input: "> some quote",
			want:  "<blockquote><p>some quote</p></blockquote>",
		},
		{
			name:  "info macro blockquote",
			input: "> **Info:** This is info",
			want:  `<ac:structured-macro ac:name="info" ac:schema-version="1"><ac:rich-text-body><p>This is info</p></ac:rich-text-body></ac:structured-macro>`,
		},
		{
			name:  "warning macro blockquote",
			input: "> **Warning:** Be careful",
			want:  `<ac:structured-macro ac:name="warning" ac:schema-version="1"><ac:rich-text-body><p>Be careful</p></ac:rich-text-body></ac:structured-macro>`,
		},
		{
			name:  "code block no language",
			input: "```\nfmt.Println()\n```",
			want:  `<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:plain-text-body><![CDATA[fmt.Println()]]></ac:plain-text-body></ac:structured-macro>`,
		},
		{
			name:  "code block with language",
			input: "```go\nfmt.Println()\n```",
			want:  `<ac:structured-macro ac:name="code" ac:schema-version="1"><ac:parameter ac:name="language">go</ac:parameter><ac:plain-text-body><![CDATA[fmt.Println()]]></ac:plain-text-body></ac:structured-macro>`,
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "strips frontmatter before converting",
			input: "---\nid: \"1\"\n---\n\nBody text",
			want:  "<p>Body text</p>",
		},
		{
			name:  "blank lines ignored",
			input: "\n\n\nHello\n\n\n",
			want:  "<p>Hello</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MarkdownToStorage(tt.input)
			if got != tt.want {
				t.Errorf("MarkdownToStorage(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMarkdownToStorage_InlineFormatting(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bold",
			input: "This is **bold** text",
			want:  "<p>This is <strong>bold</strong> text</p>",
		},
		{
			name:  "inline code",
			input: "Call `myFunc()`",
			want:  "<p>Call <code>myFunc()</code></p>",
		},
		{
			name:  "link",
			input: "[Click](https://example.com)",
			want:  `<p><a href="https://example.com">Click</a></p>`,
		},
		{
			name:  "strikethrough",
			input: "~~deleted~~",
			want:  "<p><del>deleted</del></p>",
		},
		{
			name:  "external image",
			input: "![alt](https://example.com/img.png)",
			want:  `<p><ac:image><ri:url ri:value="https://example.com/img.png"/></ac:image></p>`,
		},
		{
			name:  "attachment image",
			input: "![doc](attachment:diagram.png)",
			want:  `<p><ac:image><ri:attachment ri:filename="diagram.png"/></ac:image></p>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MarkdownToStorage(tt.input)
			if got != tt.want {
				t.Errorf("MarkdownToStorage(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMarkdownToStorage_Lists(t *testing.T) {
	t.Run("unordered list", func(t *testing.T) {
		got := MarkdownToStorage("- Alpha\n- Beta")
		if !strings.Contains(got, "<ul>") {
			t.Errorf("expected <ul>, got: %q", got)
		}
		if !strings.Contains(got, "<li>Alpha</li>") {
			t.Errorf("expected <li>Alpha</li>, got: %q", got)
		}
		if !strings.Contains(got, "<li>Beta</li>") {
			t.Errorf("expected <li>Beta</li>, got: %q", got)
		}
	})

	t.Run("ordered list", func(t *testing.T) {
		got := MarkdownToStorage("1. First\n2. Second")
		if !strings.Contains(got, "<ol>") {
			t.Errorf("expected <ol>, got: %q", got)
		}
		if !strings.Contains(got, "<li>First</li>") {
			t.Errorf("expected <li>First</li>, got: %q", got)
		}
	})
}

func TestMarkdownToStorage_Table(t *testing.T) {
	input := "| Name | Value |\n|------|-------|\n| foo | bar |"
	got := MarkdownToStorage(input)
	if !strings.Contains(got, "<table>") {
		t.Errorf("expected <table>, got: %q", got)
	}
	if !strings.Contains(got, "<th>Name</th>") {
		t.Errorf("expected <th>Name</th>, got: %q", got)
	}
	if !strings.Contains(got, "<td>foo</td>") {
		t.Errorf("expected <td>foo</td>, got: %q", got)
	}
	if !strings.Contains(got, "<td>bar</td>") {
		t.Errorf("expected <td>bar</td>, got: %q", got)
	}
}

// ─── Roundtrip: Markdown → Storage → Markdown ───────────────

func TestRoundtrip_HeadingAndParagraph(t *testing.T) {
	original := "# Hello\n\nSome paragraph text."
	storage := MarkdownToStorage(original)
	recovered := strings.TrimSpace(HTMLToMarkdown(storage))

	if !strings.Contains(recovered, "# Hello") {
		t.Errorf("heading lost in roundtrip, got: %q", recovered)
	}
	if !strings.Contains(recovered, "Some paragraph text.") {
		t.Errorf("paragraph lost in roundtrip, got: %q", recovered)
	}
}

// ─── collapseWS / cleanMarkdown (helpers) ──────────────────

func TestCollapseWS(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello  world", "hello world"},
		{"a\t\tb", "a b"},
		{"no change", "no change"},
		{"  leading", " leading"},
		{"trailing  ", "trailing "},
		{"a\n\nb", "a b"},
	}
	for _, tt := range tests {
		got := collapseWS(tt.input)
		if got != tt.want {
			t.Errorf("collapseWS(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCleanMarkdown(t *testing.T) {
	t.Run("trims and ends with newline", func(t *testing.T) {
		got := cleanMarkdown("  hello  ")
		if got != "hello\n" {
			t.Errorf("cleanMarkdown = %q, want %q", got, "hello\n")
		}
	})
	t.Run("collapses 3+ newlines to 2", func(t *testing.T) {
		got := cleanMarkdown("a\n\n\n\nb")
		if strings.Contains(got, "\n\n\n") {
			t.Errorf("cleanMarkdown should collapse 3+ newlines, got: %q", got)
		}
	})
}
