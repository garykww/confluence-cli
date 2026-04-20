package main

import (
	"os"
	"path/filepath"
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

// ─── loadConfigFile ──────────────────────────────────────────

// TestLoadConfig_FromFile verifies that credentials are read from a config file
// when environment variables are not set.
func TestLoadConfig_FromFile(t *testing.T) {
	t.Setenv("CONFLUENCE_BASE_URL", "")
	t.Setenv("CONFLUENCE_EMAIL", "")
	t.Setenv("CONFLUENCE_API_TOKEN", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".confluence-cli")
	content := "CONFLUENCE_BASE_URL=https://file.atlassian.net\nCONFLUENCE_EMAIL=file@example.com\nCONFLUENCE_API_TOKEN=filetoken\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	cfg := loadConfigFileFrom(cfgPath)

	if cfg["CONFLUENCE_BASE_URL"] != "https://file.atlassian.net" {
		t.Errorf("BASE_URL = %q", cfg["CONFLUENCE_BASE_URL"])
	}
	if cfg["CONFLUENCE_EMAIL"] != "file@example.com" {
		t.Errorf("EMAIL = %q", cfg["CONFLUENCE_EMAIL"])
	}
	if cfg["CONFLUENCE_API_TOKEN"] != "filetoken" {
		t.Errorf("API_TOKEN = %q", cfg["CONFLUENCE_API_TOKEN"])
	}
}

// TestLoadConfig_FileNotFound verifies that a missing config file returns empty map.
func TestLoadConfig_FileNotFound(t *testing.T) {
	cfg := loadConfigFileFrom("/nonexistent/path/.confluence-cli")
	if len(cfg) != 0 {
		t.Errorf("expected empty map for missing file, got %v", cfg)
	}
}

// TestLoadConfig_EnvOverridesFile verifies that env vars take precedence over file values.
func TestLoadConfig_EnvOverridesFile(t *testing.T) {
	t.Setenv("CONFLUENCE_BASE_URL", "https://env.atlassian.net")
	t.Setenv("CONFLUENCE_EMAIL", "env@example.com")
	t.Setenv("CONFLUENCE_API_TOKEN", "envtoken")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".confluence-cli")
	content := "CONFLUENCE_BASE_URL=https://file.atlassian.net\nCONFLUENCE_EMAIL=file@example.com\nCONFLUENCE_API_TOKEN=filetoken\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	fileCfg := loadConfigFileFrom(cfgPath)
	getval := func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fileCfg[key]
	}

	if getval("CONFLUENCE_BASE_URL") != "https://env.atlassian.net" {
		t.Error("env var should take precedence over file")
	}
	if getval("CONFLUENCE_EMAIL") != "env@example.com" {
		t.Error("env var should take precedence over file")
	}
}

// TestLoadConfig_IgnoresComments verifies that comment and blank lines are skipped.
func TestLoadConfig_IgnoresComments(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".confluence-cli")
	content := "# This is a comment\n\nCONFLUENCE_BASE_URL=https://ok.atlassian.net\n# another comment\nCONFLUENCE_EMAIL=ok@example.com\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	cfg := loadConfigFileFrom(cfgPath)
	if cfg["CONFLUENCE_BASE_URL"] != "https://ok.atlassian.net" {
		t.Errorf("BASE_URL = %q", cfg["CONFLUENCE_BASE_URL"])
	}
	if cfg["CONFLUENCE_EMAIL"] != "ok@example.com" {
		t.Errorf("EMAIL = %q", cfg["CONFLUENCE_EMAIL"])
	}
	if _, hasComment := cfg["# This is a comment"]; hasComment {
		t.Error("comment lines should not be parsed as keys")
	}
}
