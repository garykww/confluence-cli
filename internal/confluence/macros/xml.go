package macros

import (
	"sort"
	"strings"
)

// NodeToXML serializes a Noder tree back to XML/storage format.
// Used to render unsupported macros as a fenced code block so no content is lost.
func NodeToXML(n Noder) string {
	var sb strings.Builder
	writeXML(&sb, n)
	return sb.String()
}

func writeXML(sb *strings.Builder, n Noder) {
	if n.IsText() {
		sb.WriteString(n.Data())
		return
	}

	tag := n.Tag()
	sb.WriteString("<")
	sb.WriteString(tag)

	// Write attributes in sorted order for deterministic output.
	attrs := n.Attrs()
	if len(attrs) > 0 {
		keys := make([]string, 0, len(attrs))
		for k := range attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(" ")
			sb.WriteString(k)
			sb.WriteString(`="`)
			sb.WriteString(escapeAttr(attrs[k]))
			sb.WriteString(`"`)
		}
	}

	children := n.Children()
	if len(children) == 0 {
		sb.WriteString("/>")
		return
	}
	sb.WriteString(">")
	for _, child := range children {
		writeXML(sb, child)
	}
	sb.WriteString("</")
	sb.WriteString(tag)
	sb.WriteString(">")
}

func escapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	return s
}
