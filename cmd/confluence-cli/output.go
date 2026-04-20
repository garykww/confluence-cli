package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/garykww/confluence-cli/internal/confluence"
)

// printJSON marshals any value as indented JSON to stdout.
func printJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: JSON encoding failed: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

// printPage renders a ConfluencePage in human-readable format.
func printPage(p *confluence.ConfluencePage) {
	fmt.Printf("Title:   %s\n", p.Title)
	fmt.Printf("ID:      %s\n", p.ID)
	fmt.Printf("Status:  %s\n", p.Status)

	if p.Space != nil {
		fmt.Printf("Space:   %s (%s)\n", p.Space.Name, p.Space.Key)
	}
	if p.Version != nil {
		fmt.Printf("Version: %d\n", p.Version.Number)
		if p.Version.By != nil {
			fmt.Printf("Updated: %s by %s\n", p.Version.When, p.Version.By.DisplayName)
		}
	}
	if len(p.Ancestors) > 0 {
		names := make([]string, len(p.Ancestors))
		for i, a := range p.Ancestors {
			names[i] = fmt.Sprintf("%s (id:%s)", a.Title, a.ID)
		}
		fmt.Printf("Path:    %s\n", strings.Join(names, " > "))
	}
	if p.Body != nil {
		if p.Body.View != nil && p.Body.View.Value != "" {
			fmt.Println("\n--- Body (view) ---")
			fmt.Println(p.Body.View.Value)
		} else if p.Body.Storage != nil && p.Body.Storage.Value != "" {
			fmt.Println("\n--- Body (storage) ---")
			fmt.Println(p.Body.Storage.Value)
		}
	}
}

// printSearchResults renders a SearchResult in human-readable format.
func printSearchResults(r *confluence.SearchResult) {
	fmt.Printf("Search results: %d (showing %d–%d)\n\n", r.Size, r.Start+1, r.Start+len(r.Results))

	if len(r.Results) == 0 {
		fmt.Println("  (no results)")
		return
	}

	for i, p := range r.Results {
		space := ""
		if p.Space != nil {
			space = p.Space.Key
		}
		fmt.Printf("  %d. [%s] %s (id:%s)\n", i+1, space, p.Title, p.ID)
	}
}

// printSpace renders a Space in human-readable format.
func printSpace(s *confluence.Space) {
	fmt.Printf("Key:    %s\n", s.Key)
	fmt.Printf("Name:   %s\n", s.Name)
	fmt.Printf("Type:   %s\n", s.Type)
	fmt.Printf("Status: %s\n", s.Status)

	if s.Description != nil && s.Description.Plain != nil && s.Description.Plain.Value != "" {
		fmt.Printf("Desc:   %s\n", s.Description.Plain.Value)
	}
	if s.Homepage != nil {
		fmt.Printf("Home:   %s (id:%s)\n", s.Homepage.Title, s.Homepage.ID)
	}
}

// printSpaceList renders a SpaceList in human-readable format.
func printSpaceList(l *confluence.SpaceList) {
	fmt.Printf("Spaces: %d (showing %d–%d)\n\n", l.Size, l.Start+1, l.Start+len(l.Results))

	if len(l.Results) == 0 {
		fmt.Println("  (no spaces)")
		return
	}

	for i, s := range l.Results {
		fmt.Printf("  %d. [%s] %s (%s)\n", i+1, s.Key, s.Name, s.Type)
	}
}

// printChildPages renders a ChildPages result in human-readable format.
func printChildPages(r *confluence.ChildPages) {
	fmt.Printf("Child pages: %d\n\n", r.Size)

	if len(r.Results) == 0 {
		fmt.Println("  (no child pages)")
		return
	}

	for i, p := range r.Results {
		ver := ""
		if p.Version != nil {
			ver = fmt.Sprintf(" v%d", p.Version.Number)
		}
		fmt.Printf("  %d. %s (id:%s)%s\n", i+1, p.Title, p.ID, ver)
	}
}

// printAttachmentList renders an AttachmentList in human-readable format.
func printAttachmentList(list *confluence.AttachmentList) {
	fmt.Printf("Attachments: %d\n\n", list.Size)
	if len(list.Results) == 0 {
		fmt.Println("  (no attachments)")
		return
	}
	for i, a := range list.Results {
		size := fmt.Sprintf("%d B", a.FileSize)
		if a.FileSize >= 1024*1024 {
			size = fmt.Sprintf("%.1f MB", float64(a.FileSize)/(1024*1024))
		} else if a.FileSize >= 1024 {
			size = fmt.Sprintf("%.1f KB", float64(a.FileSize)/1024)
		}
		fmt.Printf("  %d. %s  [%s]  %s  (id:%s)\n", i+1, a.Title, a.MediaType, size, a.ID)
	}
}

// printAttachment renders a single Attachment in human-readable format.
func printAttachment(a *confluence.Attachment) {
	fmt.Printf("Title:     %s\n", a.Title)
	fmt.Printf("ID:        %s\n", a.ID)
	fmt.Printf("MediaType: %s\n", a.MediaType)
	fmt.Printf("Size:      %d bytes\n", a.FileSize)
	if a.Links != nil && a.Links.Download != "" {
		fmt.Printf("Download:  %s\n", a.Links.Download)
	}
}

// printPageMarkdown renders a ConfluencePage as Markdown with YAML frontmatter.
func printPageMarkdown(p *confluence.ConfluencePage) {
	fmt.Println("---")
	fmt.Printf("id: %q\n", p.ID)
	fmt.Printf("title: %q\n", p.Title)
	if p.Space != nil {
		fmt.Printf("space: %q\n", p.Space.Key)
	}
	if p.Version != nil {
		fmt.Printf("version: %d\n", p.Version.Number)
	}
	if len(p.Ancestors) > 0 {
		fmt.Printf("parent_id: %q\n", p.Ancestors[len(p.Ancestors)-1].ID)
	}
	fmt.Println("---")
	fmt.Println()

	// Prefer storage format for roundtrip fidelity
	bodyHTML := ""
	if p.Body != nil {
		if p.Body.Storage != nil && p.Body.Storage.Value != "" {
			bodyHTML = p.Body.Storage.Value
		} else if p.Body.View != nil && p.Body.View.Value != "" {
			bodyHTML = p.Body.View.Value
		}
	}

	if bodyHTML != "" {
		fmt.Print(confluence.HTMLToMarkdown(bodyHTML))
	}
}
