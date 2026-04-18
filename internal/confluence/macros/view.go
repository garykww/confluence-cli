package macros

import "strings"

// RenderView converts a Confluence view-format information-macro div to Markdown.
// The div typically carries class="confluence-information-macro confluence-information-macro-{type}".
func RenderView(n Noder, ctx Ctx, render RenderFunc) string {
	cls := n.Attr("class")
	macroType := "Info"
	switch {
	case strings.Contains(cls, "macro-warning"):
		macroType = "Warning"
	case strings.Contains(cls, "macro-note"):
		macroType = "Note"
	case strings.Contains(cls, "macro-tip"):
		macroType = "Tip"
	}

	// Prefer the dedicated body div; fall back to all children.
	body := ""
	for _, child := range n.Children() {
		if child.Tag() == "div" && strings.Contains(child.Attr("class"), "macro-body") {
			body = strings.TrimSpace(render(child.Children(), ctx))
		}
	}
	if body == "" {
		body = strings.TrimSpace(render(n.Children(), ctx))
	}

	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = "> " + l
	}
	return "\n\n> **" + macroType + ":** " + strings.TrimPrefix(strings.Join(lines, "\n"), "> ") + "\n\n"
}
