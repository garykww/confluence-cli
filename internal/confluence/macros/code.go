package macros

import (
	"fmt"
	"html"
	"strings"
)

// RenderCode converts a Confluence code/noformat macro to a Markdown fenced code block.
func RenderCode(n Noder, ctx Ctx, render RenderFunc) string {
	lang := ""
	body := ""
	for _, child := range n.Children() {
		switch child.Tag() {
		case "ac:parameter":
			if child.Attr("ac:name") == "language" {
				lang = strings.TrimSpace(render(child.Children(), ctx))
			}
		case "ac:plain-text-body":
			body = render(child.Children(), ctx)
		}
	}
	fence := "```"
	if lang != "" {
		fence += lang
	}
	return "\n\n" + fence + "\n" + strings.TrimRight(body, "\n") + "\n```\n\n"
}

// StorageCode converts a fenced code block to Confluence storage XML.
// Pass an empty lang string for a language-less block.
func StorageCode(lang, body string) string {
	if lang != "" {
		return fmt.Sprintf(
			`<ac:structured-macro ac:name="code" ac:schema-version="1">`+
				`<ac:parameter ac:name="language">%s</ac:parameter>`+
				`<ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body>`+
				`</ac:structured-macro>`,
			html.EscapeString(lang), body,
		)
	}
	return fmt.Sprintf(
		`<ac:structured-macro ac:name="code" ac:schema-version="1">`+
			`<ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body>`+
			`</ac:structured-macro>`,
		body,
	)
}
