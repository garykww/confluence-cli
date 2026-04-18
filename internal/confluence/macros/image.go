package macros

import (
	"fmt"
	"html"
	"strings"
)

// RenderImage converts an ac:image element to a Markdown image.
func RenderImage(n Noder) string {
	for _, child := range n.Children() {
		switch child.Tag() {
		case "ri:url":
			return "![image](" + child.Attr("ri:value") + ")"
		case "ri:attachment":
			name := child.Attr("ri:filename")
			return "![" + name + "](attachment:" + name + ")"
		}
	}
	return ""
}

// StorageImage converts a Markdown image to Confluence storage XML.
// src may be a plain URL or "attachment:<filename>" for attached files.
func StorageImage(alt, src string) string {
	if strings.HasPrefix(src, "attachment:") {
		fname := strings.TrimPrefix(src, "attachment:")
		return fmt.Sprintf(`<ac:image><ri:attachment ri:filename="%s"/></ac:image>`, html.EscapeString(fname))
	}
	return fmt.Sprintf(`<ac:image><ri:url ri:value="%s"/></ac:image>`, html.EscapeString(src))
}
