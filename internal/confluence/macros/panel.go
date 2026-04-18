package macros

import "strings"

// RenderPanel converts a Confluence panel macro to a Markdown blockquote.
func RenderPanel(n Noder, ctx Ctx, render RenderFunc) string {
	body := ""
	for _, child := range n.Children() {
		if child.Tag() == "ac:rich-text-body" {
			body = strings.TrimSpace(render(child.Children(), ctx))
		}
	}
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = "> " + l
	}
	return "\n\n" + strings.Join(lines, "\n") + "\n\n"
}

// StoragePanel converts a plain Markdown blockquote to Confluence panel macro XML.
func StoragePanel(body string) string {
	return `<ac:structured-macro ac:name="panel" ac:schema-version="1">` +
		`<ac:rich-text-body><p>` + body + `</p></ac:rich-text-body>` +
		`</ac:structured-macro>`
}
