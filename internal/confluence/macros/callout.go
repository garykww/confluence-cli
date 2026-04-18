package macros

import (
	"fmt"
	"strings"
)

// RenderCallout converts an info/note/warning/tip macro to a Markdown blockquote.
func RenderCallout(n Noder, ctx Ctx, render RenderFunc) string {
	name := n.Attr("ac:name")
	label := strings.ToUpper(name[:1]) + name[1:]
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
	return "\n\n> **" + label + ":** " + strings.TrimPrefix(strings.Join(lines, "\n"), "> ") + "\n\n"
}

// StorageCallout converts a macro-style blockquote (e.g. "> **Info:** ...") to
// Confluence storage XML. name must be one of: info, note, warning, tip.
// inlineConverter is called to process inline Markdown within body.
func StorageCallout(name, body string, inlineConverter func(string) string) string {
	return fmt.Sprintf(
		`<ac:structured-macro ac:name="%s" ac:schema-version="1">`+
			`<ac:rich-text-body><p>%s</p></ac:rich-text-body>`+
			`</ac:structured-macro>`,
		name, inlineConverter(body),
	)
}
