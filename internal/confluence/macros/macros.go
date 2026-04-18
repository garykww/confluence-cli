// Package macros implements rendering for individual Confluence macros in both
// directions: HTML/storage → Markdown (Render*) and Markdown → storage XML (Storage*).
package macros

// Noder is the minimal read interface macros need from an HTML tree node.
// confluence.node satisfies this interface implicitly.
type Noder interface {
	Tag() string
	Attr(key string) string
	Attrs() map[string]string
	Children() []Noder
	IsText() bool
	Data() string
}

// Ctx is the rendering-context interface passed between renderers.
// confluence.renderCtx satisfies this interface implicitly.
type Ctx interface {
	InPre() bool
	SetInPre(v bool)
}

// RenderFunc renders a slice of child nodes to a Markdown string.
type RenderFunc func(children []Noder, ctx Ctx) string

// RenderUnknown serializes any unrecognised Confluence element (ac:* or ri:*)
// to a ```confluence-macro code block so content is never silently dropped.
func RenderUnknown(n Noder) string {
	return "\n\n```confluence-macro\n" + NodeToXML(n) + "\n```\n\n"
}

// Dispatch routes an ac:structured-macro node to the correct Render* function.
func Dispatch(n Noder, ctx Ctx, render RenderFunc) string {
	switch n.Attr("ac:name") {
	case "code", "noformat":
		return RenderCode(n, ctx, render)
	case "info", "note", "warning", "tip":
		return RenderCallout(n, ctx, render)
	case "toc":
		return "" // auto-generated; no Markdown equivalent
	case "expand":
		return RenderExpand(n, ctx, render)
	case "panel":
		return RenderPanel(n, ctx, render)
	case "profile":
		return "" // user-profile; no Markdown equivalent
	default:
		return RenderUnknown(n)
	}
}
