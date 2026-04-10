package main

import (
	"testing"
)

// TestLoadConfig_Success verifies that loadConfig reads env vars and strips
// the trailing slash from CONFLUENCE_BASE_URL.
func TestLoadConfig_Success(t *testing.T) {
	t.Setenv("CONFLUENCE_BASE_URL", "https://example.atlassian.net/")
	t.Setenv("CONFLUENCE_EMAIL", "test@example.com")
	t.Setenv("CONFLUENCE_API_TOKEN", "mytoken")

	cfg := loadConfig()

	if cfg.BaseURL != "https://example.atlassian.net" {
		t.Errorf("BaseURL = %q, want trailing slash stripped", cfg.BaseURL)
	}
	if cfg.Email != "test@example.com" {
		t.Errorf("Email = %q, want test@example.com", cfg.Email)
	}
	if cfg.APIToken != "mytoken" {
		t.Errorf("APIToken = %q, want mytoken", cfg.APIToken)
	}
}

// TestLoadConfig_NoTrailingSlash verifies a URL without a trailing slash is unchanged.
func TestLoadConfig_NoTrailingSlash(t *testing.T) {
	t.Setenv("CONFLUENCE_BASE_URL", "https://example.atlassian.net")
	t.Setenv("CONFLUENCE_EMAIL", "a@b.com")
	t.Setenv("CONFLUENCE_API_TOKEN", "tok")

	cfg := loadConfig()

	if cfg.BaseURL != "https://example.atlassian.net" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
}

// TestLoadConfig_MultipleTrailingSlashes verifies all trailing slashes are stripped.
func TestLoadConfig_MultipleTrailingSlashes(t *testing.T) {
	t.Setenv("CONFLUENCE_BASE_URL", "https://example.atlassian.net///")
	t.Setenv("CONFLUENCE_EMAIL", "a@b.com")
	t.Setenv("CONFLUENCE_API_TOKEN", "tok")

	cfg := loadConfig()

	if cfg.BaseURL != "https://example.atlassian.net" {
		t.Errorf("BaseURL = %q, want all trailing slashes stripped", cfg.BaseURL)
	}
}
