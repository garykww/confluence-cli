package macros

import (
	"fmt"
	"html"
)

// RenderMention converts an ac:mention element to a Markdown @mention.
func RenderMention(n Noder) string {
	accountID := n.Attr("ac:account-id")
	if accountID == "" {
		return ""
	}
	return "@" + accountID
}

// StorageMention converts an account ID back to Confluence storage XML for a user mention.
func StorageMention(accountID string) string {
	return fmt.Sprintf(`<ac:link><ri:user ri:account-id="%s"/></ac:link>`, html.EscapeString(accountID))
}
