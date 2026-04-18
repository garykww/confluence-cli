package macros

import "strings"

// RenderExpand converts a Confluence expand macro to a Markdown details/summary block.
func RenderExpand(n Noder, ctx Ctx, render RenderFunc) string {
	title := ""
	body := ""
	for _, child := range n.Children() {
		switch child.Tag() {
		case "ac:parameter":
			if child.Attr("ac:name") == "title" {
				title = strings.TrimSpace(render(child.Children(), ctx))
			}
		case "ac:rich-text-body":
			body = strings.TrimSpace(render(child.Children(), ctx))
		}
	}
	if title != "" {
		return "\n\n<details>\n<summary>" + title + "</summary>\n\n" + body + "\n\n</details>\n\n"
	}
	return "\n\n" + body + "\n\n"
}

// StorageExpand converts a Markdown details/summary block to Confluence expand macro XML.
func StorageExpand(title, body string) string {
	var sb strings.Builder
	sb.WriteString(`<ac:structured-macro ac:name="expand" ac:schema-version="1">`)
	if title != "" {
		sb.WriteString(`<ac:parameter ac:name="title">`)
		sb.WriteString(title)
		sb.WriteString(`</ac:parameter>`)
	}
	sb.WriteString(`<ac:rich-text-body>`)
	sb.WriteString(body)
	sb.WriteString(`</ac:rich-text-body>`)
	sb.WriteString(`</ac:structured-macro>`)
	return sb.String()
}
