package confluence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── ExtractPageIDFromURL ───────────────────────────────────

func TestExtractPageIDFromURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name: "standard Confluence URL",
			url:  "https://garykww.atlassian.net/wiki/spaces/TEST/pages/131166",
			want: "131166",
		},
		{
			name: "URL with page title slug after ID",
			url:  "https://garykww.atlassian.net/wiki/spaces/TEST/pages/123/My+Page+Title",
			want: "123",
		},
		{
			name:    "no /pages/ segment",
			url:     "https://garykww.atlassian.net/wiki/spaces/TEST",
			wantErr: true,
		},
		{
			name:    "empty string",
			url:     "",
			wantErr: true,
		},
		{
			name:    "/pages/ without numeric ID",
			url:     "https://example.com/wiki/spaces/XX/pages/",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractPageIDFromURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ─── NewClient ──────────────────────────────────────────────

func TestNewClient_BaseURL(t *testing.T) {
	c := NewClient(Config{BaseURL: "https://example.atlassian.net", Email: "a@b.com", APIToken: "tok"})
	want := "https://example.atlassian.net/wiki/rest/api"
	if c.baseURL != want {
		t.Errorf("baseURL = %q, want %q", c.baseURL, want)
	}
}

func TestNewClient_AuthHeader(t *testing.T) {
	c := NewClient(Config{BaseURL: "https://x.com", Email: "user@example.com", APIToken: "secret"})
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@example.com:secret"))
	if c.authHeader != want {
		t.Errorf("authHeader = %q, want %q", c.authHeader, want)
	}
}

// ─── HTTP helper ────────────────────────────────────────────

// newTestClient starts a test HTTP server with the given handler and returns
// a Client wired to it, plus a cleanup function.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := NewClient(Config{BaseURL: srv.URL, Email: "a@b.com", APIToken: "tok"})
	return c, srv.Close
}

// ─── GetPage ────────────────────────────────────────────────

func TestGetPage_Success(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/rest/api/content/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		if r.URL.Query().Get("expand") == "" {
			t.Error("expand param should be set by default")
		}
		json.NewEncoder(w).Encode(ConfluencePage{ID: "42", Title: "Hello", Status: "current"})
	})
	defer cleanup()

	page, err := c.GetPage(context.Background(), "42", "")
	if err != nil {
		t.Fatalf("GetPage error: %v", err)
	}
	if page.ID != "42" {
		t.Errorf("page.ID = %q, want %q", page.ID, "42")
	}
	if page.Title != "Hello" {
		t.Errorf("page.Title = %q, want %q", page.Title, "Hello")
	}
}

func TestGetPage_CustomExpand(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("expand") != "space,version" {
			t.Errorf("expand = %q, want %q", r.URL.Query().Get("expand"), "space,version")
		}
		json.NewEncoder(w).Encode(ConfluencePage{ID: "1"})
	})
	defer cleanup()

	c.GetPage(context.Background(), "1", "space,version") //nolint:errcheck
}

func TestGetPage_404(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not found"}`))
	})
	defer cleanup()

	_, err := c.GetPage(context.Background(), "999", "")
	if err == nil {
		t.Error("expected error for 404 response, got nil")
	}
}

func TestGetPage_EmptyID(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not make a request with empty ID")
	})
	defer cleanup()

	_, err := c.GetPage(context.Background(), "", "")
	if err == nil {
		t.Error("expected error for empty page ID")
	}
}

// ─── SearchContent ──────────────────────────────────────────

func TestSearchContent_Success(t *testing.T) {
	expected := SearchResult{
		Results: []ConfluencePage{{ID: "1", Title: "Page One"}},
		Size:    1, Limit: 10, Start: 0,
	}
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/rest/api/content/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("cql") == "" {
			t.Error("cql param missing")
		}
		json.NewEncoder(w).Encode(expected)
	})
	defer cleanup()

	got, err := c.SearchContent(context.Background(), "type=page AND space=TEST", 10, 0)
	if err != nil {
		t.Fatalf("SearchContent error: %v", err)
	}
	if got.Size != 1 {
		t.Errorf("Size = %d, want 1", got.Size)
	}
	if got.Results[0].Title != "Page One" {
		t.Errorf("Results[0].Title = %q, want %q", got.Results[0].Title, "Page One")
	}
}

func TestSearchContent_PassesLimitAndStart(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "5" {
			t.Errorf("limit = %q, want 5", r.URL.Query().Get("limit"))
		}
		if r.URL.Query().Get("start") != "20" {
			t.Errorf("start = %q, want 20", r.URL.Query().Get("start"))
		}
		json.NewEncoder(w).Encode(SearchResult{})
	})
	defer cleanup()

	c.SearchContent(context.Background(), "type=page", 5, 20) //nolint:errcheck
}

func TestSearchContent_EmptyCQL(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not make a request with empty CQL")
	})
	defer cleanup()

	_, err := c.SearchContent(context.Background(), "", 10, 0)
	if err == nil {
		t.Error("expected error for empty CQL query")
	}
}

// ─── GetSpace ───────────────────────────────────────────────

func TestGetSpace_Success(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/rest/api/space/TEST" {
			t.Errorf("path = %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(Space{Key: "TEST", Name: "Developer Experience", Type: "global", Status: "current"})
	})
	defer cleanup()

	space, err := c.GetSpace(context.Background(), "TEST")
	if err != nil {
		t.Fatalf("GetSpace error: %v", err)
	}
	if space.Key != "TEST" {
		t.Errorf("Key = %q, want TEST", space.Key)
	}
}

func TestGetSpace_Error(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	})
	defer cleanup()

	_, err := c.GetSpace(context.Background(), "TEST")
	if err == nil {
		t.Error("expected error for 401 response")
	}
}

func TestGetSpace_EmptyKey(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not make a request with empty key")
	})
	defer cleanup()

	_, err := c.GetSpace(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty space key")
	}
}

// ─── ListSpaces ─────────────────────────────────────────────

func TestListSpaces_Success(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/rest/api/space" {
			t.Errorf("path = %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SpaceList{
			Results: []Space{{Key: "TEST"}, {Key: "TEST"}},
			Size:    2,
		})
	})
	defer cleanup()

	list, err := c.ListSpaces(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("ListSpaces error: %v", err)
	}
	if len(list.Results) != 2 {
		t.Errorf("got %d spaces, want 2", len(list.Results))
	}
}

// ─── GetChildPages ──────────────────────────────────────────

func TestGetChildPages_Success(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/rest/api/content/99/child/page" {
			t.Errorf("path = %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ChildPages{
			Results: []ConfluencePage{{ID: "100", Title: "Child"}},
			Size:    1,
		})
	})
	defer cleanup()

	children, err := c.GetChildPages(context.Background(), "99", 25)
	if err != nil {
		t.Fatalf("GetChildPages error: %v", err)
	}
	if len(children.Results) != 1 || children.Results[0].ID != "100" {
		t.Errorf("unexpected children: %+v", children)
	}
}

func TestGetChildPages_EmptyID(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not make a request with empty parent ID")
	})
	defer cleanup()

	_, err := c.GetChildPages(context.Background(), "", 25)
	if err == nil {
		t.Error("expected error for empty parent page ID")
	}
}

// ─── UpdatePage ─────────────────────────────────────────────

func TestUpdatePage_Success(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/wiki/rest/api/content/77" {
			t.Errorf("path = %q", r.URL.Path)
		}

		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)

		// version should be current+1
		ver := payload["version"].(map[string]any)
		if ver["number"].(float64) != 6 {
			t.Errorf("version.number = %v, want 6 (current 5 + 1)", ver["number"])
		}

		json.NewEncoder(w).Encode(ConfluencePage{
			ID: "77", Title: "Updated", Version: &Version{Number: 6},
		})
	})
	defer cleanup()

	page, err := c.UpdatePage(context.Background(), "77", "Updated", 5, "<p>new content</p>")
	if err != nil {
		t.Fatalf("UpdatePage error: %v", err)
	}
	if page.Version.Number != 6 {
		t.Errorf("version = %d, want 6", page.Version.Number)
	}
}

func TestUpdatePage_ConflictError(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"message":"Version conflict"}`))
	})
	defer cleanup()

	_, err := c.UpdatePage(context.Background(), "77", "Title", 1, "<p>body</p>")
	if err == nil {
		t.Fatal("expected error for 409 response")
	}
	var conflictErr *ConflictError
	if !isConflictError(err, &conflictErr) {
		t.Errorf("expected ConflictError, got %T: %v", err, err)
	}
}

func TestUpdatePage_ServerError(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"Internal error"}`))
	})
	defer cleanup()

	_, err := c.UpdatePage(context.Background(), "77", "Title", 1, "<p>body</p>")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestUpdatePage_ValidationErrors(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach server on validation failure")
	})
	defer cleanup()

	ctx := context.Background()

	if _, err := c.UpdatePage(ctx, "", "Title", 1, "body"); err == nil {
		t.Error("expected error for empty page ID")
	}
	if _, err := c.UpdatePage(ctx, "1", "", 1, "body"); err == nil {
		t.Error("expected error for empty title")
	}
	if _, err := c.UpdatePage(ctx, "1", "Title", 0, "body"); err == nil {
		t.Error("expected error for zero version")
	}
}

// ─── doGet: Auth and Accept headers ─────────────────────────

func TestDoGet_SetsRequiredHeaders(t *testing.T) {
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@ex.com:token"))

	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expectedAuth {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), expectedAuth)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q, want application/json", r.Header.Get("Accept"))
		}
		json.NewEncoder(w).Encode(ConfluencePage{ID: "1"})
	})
	defer cleanup()

	// Override the client to use our test credentials
	c.authHeader = expectedAuth
	c.GetPage(context.Background(), "1", "") //nolint:errcheck
}

// ─── CreatePage ─────────────────────────────────────────────

func TestCreatePage_Success(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		if payload["title"] != "My Page" {
			t.Errorf("title = %v, want %q", payload["title"], "My Page")
		}
		space, _ := payload["space"].(map[string]any)
		if space["key"] != "ENG" {
			t.Errorf("space.key = %v, want %q", space["key"], "ENG")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ConfluencePage{ID: "999", Title: "My Page"})
	})
	defer cleanup()

	page, err := c.CreatePage(context.Background(), "ENG", "My Page", "", "<p>Hello</p>")
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}
	if page.ID != "999" {
		t.Errorf("page.ID = %q, want %q", page.ID, "999")
	}
}

func TestCreatePage_WithParent(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		ancestors, _ := payload["ancestors"].([]any)
		if len(ancestors) == 0 {
			t.Error("expected ancestors array with parentID")
		} else {
			anc, _ := ancestors[0].(map[string]any)
			if anc["id"] != "42" {
				t.Errorf("ancestors[0].id = %v, want %q", anc["id"], "42")
			}
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ConfluencePage{ID: "100"})
	})
	defer cleanup()

	c.CreatePage(context.Background(), "ENG", "Child", "42", "") //nolint:errcheck
}

func TestCreatePage_NoParent(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		if _, hasAncestors := payload["ancestors"]; hasAncestors {
			t.Error("ancestors key should be absent when parentID is empty")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ConfluencePage{ID: "101"})
	})
	defer cleanup()

	c.CreatePage(context.Background(), "ENG", "Root Page", "", "") //nolint:errcheck
}

func TestCreatePage_ValidationErrors(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach server on validation failure")
	})
	defer cleanup()

	ctx := context.Background()
	if _, err := c.CreatePage(ctx, "", "Title", "", "body"); err == nil {
		t.Error("expected error for empty spaceKey")
	}
	if _, err := c.CreatePage(ctx, "ENG", "", "", "body"); err == nil {
		t.Error("expected error for empty title")
	}
}

func TestCreatePage_ServerError(t *testing.T) {
	c, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"server error"}`))
	})
	defer cleanup()

	_, err := c.CreatePage(context.Background(), "ENG", "Title", "", "body")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

// ─── helpers ────────────────────────────────────────────────

func isConflictError(err error, target **ConflictError) bool {
	if err == nil {
		return false
	}
	var ce *ConflictError
	ok := false
	// Walk the error chain manually since errors.As requires a pointer to interface
	type unwrapper interface{ Unwrap() error }
	for e := err; e != nil; {
		if v, matched := e.(*ConflictError); matched {
			ce = v
			ok = true
			break
		}
		if u, canUnwrap := e.(unwrapper); canUnwrap {
			e = u.Unwrap()
		} else {
			break
		}
	}
	if ok && target != nil {
		*target = ce
	}
	return ok
}
