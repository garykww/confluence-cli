package macros

import (
	"fmt"
	"html"
	"strings"
)

// RenderLink converts an ac:link element to a Markdown link or @mention.
func RenderLink(n Noder, ctx Ctx, render RenderFunc) string {
	href := ""
	text := ""
	userAccountID := ""
	for _, child := range n.Children() {
		switch child.Tag() {
		case "ri:page":
			href = child.Attr("ri:content-title")
			if text == "" {
				text = href
			}
		case "ri:url":
			href = child.Attr("ri:value")
		case "ri:user":
			userAccountID = child.Attr("ri:account-id")
		case "ac:plain-text-link-body", "ac:link-body":
			text = strings.TrimSpace(render(child.Children(), ctx))
		}
	}
	// User mention: prefer explicit link-body text, fall back to account-id.
	if userAccountID != "" {
		if text != "" {
			return "@" + text
		}
		return "@" + userAccountID
	}
	if href != "" && text != "" {
		return "[" + text + "](" + href + ")"
	}
	if text != "" {
		return text
	}
	return href
}

// StorageLink converts a Markdown link to an HTML anchor in Confluence storage format.
func StorageLink(text, href string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(href), text)
}
