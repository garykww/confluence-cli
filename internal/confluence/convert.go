package confluence

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"confluence-cli/internal/confluence/macros"
)

// ─── HTML Tokenizer ─────────────────────────────────────────

type tokenKind int

const (
	tkText      tokenKind = iota
	tkOpen                // <tag attrs>
	tkClose               // </tag>
	tkSelfClose           // <br/> or void elements
)

type token struct {
	kind  tokenKind
	tag   string
	attrs map[string]string
	data  string
}

// voidElements are HTML tags that never have children.
var voidElements = map[string]bool{
	"br": true, "hr": true, "img": true, "input": true,
	"col": true, "colgroup": true, "meta": true, "link": true,
}

func tokenize(s string) []token {
	var tokens []token
	i := 0
	for i < len(s) {
		if s[i] == '<' {
			// Check for comment <!-- ... -->
			if strings.HasPrefix(s[i:], "<!--") {
				end := strings.Index(s[i:], "-->")
				if end >= 0 {
					i += end + 3
				} else {
					i = len(s)
				}
				continue
			}
			// Check for CDATA <![CDATA[ ... ]]>
			if strings.HasPrefix(s[i:], "<![CDATA[") {
				end := strings.Index(s[i:], "]]>")
				if end >= 0 {
					text := s[i+9 : i+end]
					tokens = append(tokens, token{kind: tkText, data: text})
					i += end + 3
				} else {
					i = len(s)
				}
				continue
			}

			// Find closing >
			j := findTagEnd(s, i+1)
			if j < 0 {
				tokens = append(tokens, token{kind: tkText, data: s[i:]})
				break
			}
			raw := s[i+1 : j]
			i = j + 1

			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}

			// Close tag
			if raw[0] == '/' {
				tag := strings.TrimSpace(raw[1:])
				if idx := strings.IndexAny(tag, " \t\n"); idx >= 0 {
					tag = tag[:idx]
				}
				tokens = append(tokens, token{kind: tkClose, tag: strings.ToLower(tag)})
				continue
			}

			selfClose := strings.HasSuffix(raw, "/")
			if selfClose {
				raw = strings.TrimSpace(raw[:len(raw)-1])
			}

			tag, attrs := parseTagContent(raw)
			tag = strings.ToLower(tag)

			if selfClose || voidElements[tag] {
				tokens = append(tokens, token{kind: tkSelfClose, tag: tag, attrs: attrs})
			} else {
				tokens = append(tokens, token{kind: tkOpen, tag: tag, attrs: attrs})
			}
		} else {
			// Text node
			j := strings.IndexByte(s[i:], '<')
			if j < 0 {
				tokens = append(tokens, token{kind: tkText, data: s[i:]})
				break
			}
			text := s[i : i+j]
			if text != "" {
				tokens = append(tokens, token{kind: tkText, data: text})
			}
			i += j
		}
	}
	return tokens
}

// findTagEnd finds the index of '>' that closes the tag, respecting quoted attributes.
func findTagEnd(s string, start int) int {
	inQuote := byte(0)
	for i := start; i < len(s); i++ {
		if inQuote != 0 {
			if s[i] == inQuote {
				inQuote = 0
			}
			continue
		}
		switch s[i] {
		case '"', '\'':
			inQuote = s[i]
		case '>':
			return i
		}
	}
	return -1
}

func parseTagContent(raw string) (string, map[string]string) {
	// Split tag name from attributes
	idx := strings.IndexAny(raw, " \t\n")
	if idx < 0 {
		return raw, nil
	}
	tag := raw[:idx]
	rest := raw[idx:]
	return tag, parseAttrs(rest)
}

func parseAttrs(s string) map[string]string {
	attrs := make(map[string]string)
	i := 0
	for i < len(s) {
		// Skip whitespace
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
			i++
		}
		if i >= len(s) {
			break
		}
		// Read attribute name
		j := i
		for j < len(s) && s[j] != '=' && s[j] != ' ' && s[j] != '\t' && s[j] != '\n' {
			j++
		}
		if j == i {
			i++
			continue
		}
		name := strings.ToLower(s[i:j])
		i = j

		// Skip whitespace
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
			i++
		}
		if i >= len(s) || s[i] != '=' {
			attrs[name] = ""
			continue
		}
		i++ // skip '='

		// Skip whitespace
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
			i++
		}
		if i >= len(s) {
			break
		}

		// Read value
		if s[i] == '"' || s[i] == '\'' {
			q := s[i]
			i++
			j = i
			for j < len(s) && s[j] != q {
				j++
			}
			attrs[name] = s[i:j]
			if j < len(s) {
				j++
			}
			i = j
		} else {
			j = i
			for j < len(s) && s[j] != ' ' && s[j] != '\t' && s[j] != '\n' {
				j++
			}
			attrs[name] = s[i:j]
			i = j
		}
	}
	return attrs
}

// ─── Tree Builder ───────────────────────────────────────────

type node struct {
	tag      string
	attrs    map[string]string
	children []*node
	data     string // text content for text nodes
}

// isText and attr are kept for internal use within this package.
func (n *node) isText() bool         { return n.tag == "" }
func (n *node) attr(k string) string { return n.Attr(k) }

// Tag, IsText, Attr, Data, Children implement macros.Noder so that *node can be
// passed directly to functions in the macros sub-package.
func (n *node) Tag() string  { return n.tag }
func (n *node) IsText() bool { return n.tag == "" }
func (n *node) Data() string { return n.data }
func (n *node) Attr(k string) string {
	if n.attrs == nil {
		return ""
	}
	return n.attrs[k]
}
func (n *node) Attrs() map[string]string { return n.attrs }
func (n *node) Children() []macros.Noder {
	result := make([]macros.Noder, len(n.children))
	for i, c := range n.children {
		result[i] = c
	}
	return result
}

func buildTree(tokens []token) []*node {
	root := &node{tag: "_root"}
	stack := []*node{root}

	for _, t := range tokens {
		parent := stack[len(stack)-1]
		switch t.kind {
		case tkText:
			parent.children = append(parent.children, &node{data: t.data})
		case tkOpen:
			child := &node{tag: t.tag, attrs: t.attrs}
			parent.children = append(parent.children, child)
			stack = append(stack, child)
		case tkClose:
			// Pop stack to find matching tag
			for i := len(stack) - 1; i > 0; i-- {
				if stack[i].tag == t.tag {
					stack = stack[:i]
					break
				}
			}
		case tkSelfClose:
			parent.children = append(parent.children, &node{tag: t.tag, attrs: t.attrs})
		}
	}
	return root.children
}

// ─── HTML → Markdown Renderer ───────────────────────────────

type renderCtx struct {
	listDepth int
	listType  []string // "ul" or "ol"
	listIndex []int    // counter for ol items
	inPre     bool
}

// InPre and SetInPre implement macros.Ctx.
func (c *renderCtx) InPre() bool     { return c.inPre }
func (c *renderCtx) SetInPre(v bool) { c.inPre = v }

// makeRenderFn wraps renderNodes as a macros.RenderFunc so that macro renderers
// can recursively render their child nodes without importing this package.
func makeRenderFn(ctx *renderCtx) macros.RenderFunc {
	return func(children []macros.Noder, _ macros.Ctx) string {
		nodes := make([]*node, len(children))
		for i, ch := range children {
			nodes[i] = ch.(*node) //nolint:forcetypeassert // always *node within this package
		}
		return renderNodes(nodes, ctx)
	}
}

// HTMLToMarkdown converts Confluence HTML (storage or view) to Markdown.
func HTMLToMarkdown(input string) string {
	tokens := tokenize(input)
	tree := buildTree(tokens)
	md := renderNodes(tree, &renderCtx{})
	return cleanMarkdown(md)
}

func renderNodes(nodes []*node, ctx *renderCtx) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(renderNode(n, ctx))
	}
	return b.String()
}

func renderNode(n *node, ctx *renderCtx) string {
	if n.isText() {
		text := html.UnescapeString(n.data)
		if ctx.inPre {
			return text
		}
		return collapseWS(text)
	}

	switch n.tag {
	// ── Headings ──
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(n.tag[1] - '0')
		prefix := strings.Repeat("#", level)
		content := strings.TrimSpace(renderNodes(n.children, ctx))
		if content == "" {
			return ""
		}
		return "\n\n" + prefix + " " + content + "\n\n"

	// ── Paragraphs ──
	case "p":
		content := strings.TrimSpace(renderNodes(n.children, ctx))
		if content == "" {
			return ""
		}
		return content + "\n\n"

	// ── Inline formatting ──
	case "strong", "b":
		c := renderNodes(n.children, ctx)
		if strings.TrimSpace(c) == "" {
			return c
		}
		return "**" + c + "**"
	case "em", "i":
		c := renderNodes(n.children, ctx)
		if strings.TrimSpace(c) == "" {
			return c
		}
		return "*" + c + "*"
	case "del", "s":
		c := renderNodes(n.children, ctx)
		return "~~" + c + "~~"
	case "code":
		if ctx.inPre {
			return renderNodes(n.children, ctx)
		}
		c := renderNodes(n.children, ctx)
		return "`" + c + "`"
	case "u":
		return renderNodes(n.children, ctx) // markdown has no underline

	// ── Code blocks ──
	case "pre":
		ctx.inPre = true
		c := renderNodes(n.children, ctx)
		ctx.inPre = false
		return "\n\n```\n" + strings.TrimRight(c, "\n") + "\n```\n\n"

	// ── Links ──
	case "a":
		href := n.attr("href")
		c := strings.TrimSpace(renderNodes(n.children, ctx))
		if href == "" || c == "" {
			return c
		}
		return "[" + c + "](" + href + ")"

	// ── Images ──
	case "img":
		alt := n.attr("alt")
		src := n.attr("src")
		if src != "" {
			return "![" + alt + "](" + src + ")"
		}
		return ""

	// ── Line breaks / rules ──
	case "br":
		if ctx.inPre {
			return "\n"
		}
		return "  \n"
	case "hr":
		return "\n\n---\n\n"

	// ── Lists ──
	case "ul":
		ctx.listDepth++
		ctx.listType = append(ctx.listType, "ul")
		ctx.listIndex = append(ctx.listIndex, 0)
		c := renderNodes(n.children, ctx)
		ctx.listDepth--
		ctx.listType = ctx.listType[:len(ctx.listType)-1]
		ctx.listIndex = ctx.listIndex[:len(ctx.listIndex)-1]
		return "\n" + c + "\n"
	case "ol":
		ctx.listDepth++
		ctx.listType = append(ctx.listType, "ol")
		ctx.listIndex = append(ctx.listIndex, 0)
		c := renderNodes(n.children, ctx)
		ctx.listDepth--
		ctx.listType = ctx.listType[:len(ctx.listType)-1]
		ctx.listIndex = ctx.listIndex[:len(ctx.listIndex)-1]
		return "\n" + c + "\n"
	case "li":
		indent := strings.Repeat("  ", ctx.listDepth-1)
		c := strings.TrimSpace(renderNodes(n.children, ctx))
		if len(ctx.listType) > 0 && ctx.listType[len(ctx.listType)-1] == "ol" {
			ctx.listIndex[len(ctx.listIndex)-1]++
			idx := ctx.listIndex[len(ctx.listIndex)-1]
			return indent + fmt.Sprintf("%d. ", idx) + c + "\n"
		}
		return indent + "- " + c + "\n"

	// ── Tables ──
	case "table":
		return "\n\n" + renderTable(n, ctx) + "\n"

	// ── Blockquotes ──
	case "blockquote":
		c := strings.TrimSpace(renderNodes(n.children, ctx))
		lines := strings.Split(c, "\n")
		for i, l := range lines {
			lines[i] = "> " + l
		}
		return "\n\n" + strings.Join(lines, "\n") + "\n\n"

	// ── Confluence macros (storage format) ──
	case "ac:structured-macro":
		return macros.Dispatch(n, ctx, makeRenderFn(ctx))
	case "ac:rich-text-body":
		return renderNodes(n.children, ctx)
	case "ac:plain-text-body":
		return renderNodes(n.children, ctx)
	case "ac:parameter":
		return "" // skip macro params
	case "ac:placeholder":
		return "" // template hint text; no content value
	case "ac:layout", "ac:layout-section", "ac:layout-cell":
		return renderNodes(n.children, ctx)
	case "ac:emoticon":
		fb := n.attr("ac:emoji-fallback")
		if fb != "" {
			return fb
		}
		return n.attr("ac:emoji-shortname")
	case "ac:image":
		return macros.RenderImage(n)
	case "ac:link":
		return macros.RenderLink(n, ctx, makeRenderFn(ctx))
	case "ri:url":
		return ""
	case "ri:attachment":
		return ""
	case "ri:page":
		return ""
	case "ri:user":
		return ""

	// ── Confluence view-format wrappers ──
	case "div":
		cls := n.attr("class")
		if strings.Contains(cls, "confluence-information-macro") {
			return macros.RenderView(n, ctx, makeRenderFn(ctx))
		}
		if strings.Contains(cls, "code-block") || strings.Contains(cls, "codeblock") {
			ctx.inPre = true
			c := renderNodes(n.children, ctx)
			ctx.inPre = false
			return "\n\n```\n" + strings.TrimRight(c, "\n") + "\n```\n\n"
		}
		return renderNodes(n.children, ctx)

	// ── Pass-through containers ──
	case "span", "thead", "tbody", "tfoot", "colgroup", "col",
		"section", "article", "main", "nav", "header", "footer",
		"figure", "figcaption", "details", "summary", "time":
		return renderNodes(n.children, ctx)

	default:
		// Unknown Confluence-specific elements are preserved as raw XML.
		// Generic HTML elements render their children as usual.
		if strings.HasPrefix(n.tag, "ac:") || strings.HasPrefix(n.tag, "ri:") {
			return macros.RenderUnknown(n)
		}
		return renderNodes(n.children, ctx)
	}
}

// ── Table rendering ──

func renderTable(n *node, ctx *renderCtx) string {
	var headerRows, bodyRows [][]string
	collectTableRows(n, &headerRows, &bodyRows, ctx)

	if len(headerRows) == 0 && len(bodyRows) == 0 {
		return ""
	}

	// If no explicit headers, promote first body row
	if len(headerRows) == 0 && len(bodyRows) > 0 {
		headerRows = bodyRows[:1]
		bodyRows = bodyRows[1:]
	}

	maxCols := 0
	for _, r := range headerRows {
		if len(r) > maxCols {
			maxCols = len(r)
		}
	}
	for _, r := range bodyRows {
		if len(r) > maxCols {
			maxCols = len(r)
		}
	}
	if maxCols == 0 {
		return ""
	}

	var b strings.Builder
	// Header
	for _, row := range headerRows {
		b.WriteString("|")
		for j := 0; j < maxCols; j++ {
			cell := ""
			if j < len(row) {
				cell = row[j]
			}
			b.WriteString(" " + cell + " |")
		}
		b.WriteString("\n")
	}
	// Separator
	b.WriteString("|")
	for j := 0; j < maxCols; j++ {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	// Body
	for _, row := range bodyRows {
		b.WriteString("|")
		for j := 0; j < maxCols; j++ {
			cell := ""
			if j < len(row) {
				cell = row[j]
			}
			b.WriteString(" " + cell + " |")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func collectTableRows(n *node, headers, body *[][]string, ctx *renderCtx) {
	for _, child := range n.children {
		switch child.tag {
		case "thead":
			for _, tr := range child.children {
				if tr.tag == "tr" {
					*headers = append(*headers, collectCells(tr, ctx))
				}
			}
		case "tbody":
			for _, tr := range child.children {
				if tr.tag == "tr" {
					*body = append(*body, collectCells(tr, ctx))
				}
			}
		case "tr":
			// Direct <tr> children (no thead/tbody wrapper)
			cells := collectCells(child, ctx)
			hasHeader := false
			for _, c := range child.children {
				if c.tag == "th" {
					hasHeader = true
					break
				}
			}
			if hasHeader {
				*headers = append(*headers, cells)
			} else {
				*body = append(*body, cells)
			}
		default:
			collectTableRows(child, headers, body, ctx)
		}
	}
}

func collectCells(tr *node, ctx *renderCtx) []string {
	var cells []string
	for _, td := range tr.children {
		if td.tag == "td" || td.tag == "th" {
			text := strings.TrimSpace(renderNodes(td.children, ctx))
			// Flatten newlines within cells
			text = strings.ReplaceAll(text, "\n", " ")
			text = reMultiSpace.ReplaceAllString(text, " ")
			cells = append(cells, text)
		}
	}
	return cells
}

// ── Cleanup ──

var (
	reMultiNewline = regexp.MustCompile(`\n{3,}`)
	reMultiSpace   = regexp.MustCompile(`[ \t]{2,}`)
	reTrailingWS   = regexp.MustCompile(`(?m)[ \t]+$`)
)

func cleanMarkdown(s string) string {
	s = strings.TrimSpace(s)
	s = reMultiNewline.ReplaceAllString(s, "\n\n")
	s = reTrailingWS.ReplaceAllString(s, "")
	return s + "\n"
}

func collapseWS(s string) string {
	// Collapse runs of whitespace into a single space
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				b.WriteByte(' ')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return b.String()
}

// ═══════════════════════════════════════════════════════════
// Markdown → Confluence Storage Format
// ═══════════════════════════════════════════════════════════

var (
	reHeading       = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	reOrderedItem   = regexp.MustCompile(`^(\s*)\d+\.\s+(.+)$`)
	reUnorderedItem = regexp.MustCompile(`^(\s*)[-*]\s+(.+)$`)
	reHR            = regexp.MustCompile(`^(---+|\*\*\*+|___+)$`)
	reBqLine        = regexp.MustCompile(`^>\s?(.*)$`)

	// Inline patterns (applied in order)
	reInlineCode   = regexp.MustCompile("`([^`]+)`")
	reInlineImage  = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	reInlineLink   = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reInlineBold   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	_              = regexp.MustCompile(`(?:^|[^*])\*([^*]+?)\*(?:[^*]|$)`) // reInlineItalic reserved for future use
	reInlineStrike = regexp.MustCompile(`~~(.+?)~~`)
)

// MarkdownToStorage converts Markdown text to Confluence storage format XHTML.
func MarkdownToStorage(md string) string {
	md = stripFrontmatter(md)
	lines := strings.Split(md, "\n")
	var buf strings.Builder
	i := 0

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Blank line
		if trimmed == "" {
			i++
			continue
		}

		// Heading
		if m := reHeading.FindStringSubmatch(trimmed); m != nil {
			level := len(m[1])
			text := inlineToStorage(m[2])
			fmt.Fprintf(&buf, "<h%d>%s</h%d>", level, text, level)
			i++
			continue
		}

		// Code block
		if strings.HasPrefix(trimmed, "```") {
			lang := strings.TrimPrefix(trimmed, "```")
			lang = strings.TrimSpace(lang)
			i++
			var code strings.Builder
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				code.WriteString(lines[i])
				code.WriteByte('\n')
				i++
			}
			if i < len(lines) {
				i++ // skip closing ```
			}
			codeStr := strings.TrimRight(code.String(), "\n")
			// confluence-macro blocks contain raw storage XML — pass through unchanged.
			if lang == "confluence-macro" {
				buf.WriteString(codeStr)
				continue
			}
			buf.WriteString(macros.StorageCode(lang, codeStr))
			continue
		}

		// HR
		if reHR.MatchString(trimmed) {
			buf.WriteString("<hr />")
			i++
			continue
		}

		// Table
		if strings.HasPrefix(trimmed, "|") {
			i = parseTableBlock(lines, i, &buf)
			continue
		}

		// Blockquote (including info/note/warning/tip macros)
		if reBqLine.MatchString(trimmed) {
			i = parseBlockquoteBlock(lines, i, &buf)
			continue
		}

		// Unordered list
		if reUnorderedItem.MatchString(trimmed) {
			i = parseListBlock(lines, i, &buf, false)
			continue
		}

		// Ordered list
		if reOrderedItem.MatchString(trimmed) {
			i = parseListBlock(lines, i, &buf, true)
			continue
		}

		// Details/summary (HTML passthrough)
		if trimmed == "<details>" {
			for i < len(lines) && strings.TrimSpace(lines[i]) != "</details>" {
				buf.WriteString(lines[i])
				i++
			}
			if i < len(lines) {
				buf.WriteString(lines[i])
				i++
			}
			continue
		}

		// Paragraph (default)
		var para strings.Builder
		for i < len(lines) {
			l := strings.TrimSpace(lines[i])
			if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "```") ||
				strings.HasPrefix(l, "|") || strings.HasPrefix(l, ">") ||
				reHR.MatchString(l) || reUnorderedItem.MatchString(l) || reOrderedItem.MatchString(l) {
				break
			}
			if para.Len() > 0 {
				para.WriteByte(' ')
			}
			para.WriteString(l)
			i++
		}
		fmt.Fprintf(&buf, "<p>%s</p>", inlineToStorage(para.String()))
	}

	return buf.String()
}

// ── Block parsers for MD → Storage ──

func parseTableBlock(lines []string, i int, buf *strings.Builder) int {
	var rows [][]string
	sepIdx := -1

	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		cells := parseTableLine(trimmed)
		// Check if this is the separator row (|---|---|)
		isSep := true
		for _, c := range cells {
			cleaned := strings.Trim(c, "- :")
			if cleaned != "" {
				isSep = false
				break
			}
		}
		if isSep && len(rows) > 0 {
			sepIdx = len(rows)
			i++
			continue
		}
		rows = append(rows, cells)
		i++
	}

	if len(rows) == 0 {
		return i
	}

	buf.WriteString("<table>")

	headerEnd := 1
	if sepIdx > 0 {
		headerEnd = sepIdx
	}

	// Header
	buf.WriteString("<thead>")
	for r := 0; r < headerEnd && r < len(rows); r++ {
		buf.WriteString("<tr>")
		for _, cell := range rows[r] {
			fmt.Fprintf(buf, "<th>%s</th>", inlineToStorage(cell))
		}
		buf.WriteString("</tr>")
	}
	buf.WriteString("</thead>")

	// Body
	if headerEnd < len(rows) {
		buf.WriteString("<tbody>")
		for r := headerEnd; r < len(rows); r++ {
			buf.WriteString("<tr>")
			for _, cell := range rows[r] {
				fmt.Fprintf(buf, "<td>%s</td>", inlineToStorage(cell))
			}
			buf.WriteString("</tr>")
		}
		buf.WriteString("</tbody>")
	}

	buf.WriteString("</table>")
	return i
}

func parseTableLine(s string) []string {
	s = strings.Trim(s, "|")
	parts := strings.Split(s, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

func parseBlockquoteBlock(lines []string, i int, buf *strings.Builder) int {
	var content strings.Builder
	for i < len(lines) {
		m := reBqLine.FindStringSubmatch(lines[i])
		if m == nil {
			break
		}
		content.WriteString(m[1])
		content.WriteByte('\n')
		i++
	}
	text := strings.TrimSpace(content.String())

	// Check if this is a macro-style blockquote: > **Info:** ...
	reMacro := regexp.MustCompile(`^\*\*(Info|Note|Warning|Tip):\*\*\s*(.*)`)
	if m := reMacro.FindStringSubmatch(text); m != nil {
		macroName := strings.ToLower(m[1])
		body := m[2]
		// Remaining lines after the first
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			body = m[2] + text[idx:]
		}
		body = strings.TrimSpace(body)
		buf.WriteString(macros.StorageCallout(macroName, body, inlineToStorage))
		return i
	}

	// Regular blockquote
	fmt.Fprintf(buf, "<blockquote><p>%s</p></blockquote>", inlineToStorage(text))
	return i
}

func parseListBlock(lines []string, i int, buf *strings.Builder, ordered bool) int {
	tag := "ul"
	itemRe := reUnorderedItem
	if ordered {
		tag = "ol"
		itemRe = reOrderedItem
	}

	buf.WriteString("<" + tag + ">")
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			continue
		}
		m := itemRe.FindStringSubmatch(lines[i])
		if m == nil {
			break
		}
		content := m[2]
		i++

		// Check for nested list (indented items on subsequent lines)
		var nested strings.Builder
		for i < len(lines) {
			next := lines[i]
			if strings.TrimSpace(next) == "" {
				break
			}
			// Check if the next line is more indented (continuation or nested)
			indent := len(m[1])
			nextIndent := len(next) - len(strings.TrimLeft(next, " \t"))
			if nextIndent <= indent {
				break
			}
			nested.WriteString(next)
			nested.WriteByte('\n')
			i++
		}

		if nested.Len() > 0 {
			// Recursively convert nested content
			nestedStorage := MarkdownToStorage(nested.String())
			fmt.Fprintf(buf, "<li>%s%s</li>", inlineToStorage(content), nestedStorage)
		} else {
			fmt.Fprintf(buf, "<li>%s</li>", inlineToStorage(content))
		}
	}
	buf.WriteString("</" + tag + ">")
	return i
}

// ── Inline Markdown → Storage ──

func inlineToStorage(text string) string {
	// Protect code spans first
	var codeSpans []string
	text = reInlineCode.ReplaceAllStringFunc(text, func(match string) string {
		inner := reInlineCode.FindStringSubmatch(match)[1]
		idx := len(codeSpans)
		codeSpans = append(codeSpans, inner)
		return fmt.Sprintf("\x00CODE%d\x00", idx)
	})

	// Escape raw angle brackets so literal <...> in Markdown text doesn't become
	// an HTML tag in storage. This must happen before the patterns below because
	// those patterns generate their own <tag> strings which must NOT be escaped.
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")

	// Images before links (![...](...) vs [...](...))
	text = reInlineImage.ReplaceAllStringFunc(text, func(match string) string {
		m := reInlineImage.FindStringSubmatch(match)
		return macros.StorageImage(m[1], m[2])
	})

	// Links
	text = reInlineLink.ReplaceAllStringFunc(text, func(match string) string {
		m := reInlineLink.FindStringSubmatch(match)
		return macros.StorageLink(m[1], m[2])
	})

	// Bold (before italic)
	text = reInlineBold.ReplaceAllString(text, "<strong>$1</strong>")

	// Italic — simple approach: match single * not preceded/followed by *
	text = replaceItalic(text)

	// Strikethrough
	text = reInlineStrike.ReplaceAllString(text, "<del>$1</del>")

	// Restore code spans
	for idx, code := range codeSpans {
		text = strings.Replace(text, fmt.Sprintf("\x00CODE%d\x00", idx), "<code>"+html.EscapeString(code)+"</code>", 1)
	}

	return text
}

func replaceItalic(s string) string {
	// Match *text* where * is not part of ** (bold already replaced to <strong>)
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '*' && (i+1 < len(s) && s[i+1] != '*') {
			// Find closing *
			end := strings.IndexByte(s[i+1:], '*')
			if end > 0 {
				inner := s[i+1 : i+1+end]
				result.WriteString("<em>")
				result.WriteString(inner)
				result.WriteString("</em>")
				i = i + 1 + end + 1
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

// PageMeta holds metadata parsed from YAML frontmatter.
type PageMeta struct {
	ID       string
	Title    string
	Space    string
	Version  int
	ParentID string
}

// ParseFrontmatter extracts YAML frontmatter metadata and the remaining body
// from a markdown string. Returns zero-value PageMeta if no frontmatter found.
func ParseFrontmatter(md string) (PageMeta, string) {
	var meta PageMeta
	if !strings.HasPrefix(md, "---\n") {
		return meta, md
	}
	end := strings.Index(md[4:], "\n---")
	if end < 0 {
		return meta, md
	}

	header := md[4 : 4+end]
	body := strings.TrimLeft(md[4+end+4:], "\n")

	for _, line := range strings.Split(header, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, "\"")
		switch key {
		case "id":
			meta.ID = val
		case "title":
			meta.Title = val
		case "space":
			meta.Space = val
		case "version":
			//nolint:errcheck // best-effort integer parse; zero value is handled by caller
			fmt.Sscanf(val, "%d", &meta.Version)
		case "parent_id":
			meta.ParentID = val
		}
	}

	return meta, body
}

func stripFrontmatter(md string) string {
	_, body := ParseFrontmatter(md)
	return body
}
