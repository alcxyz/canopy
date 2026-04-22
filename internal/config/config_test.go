package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_BasicConfig(t *testing.T) {
	dir := t.TempDir()
	xdgPath := filepath.Join(dir, "canopy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdgPath, []byte(`
profiles:
  - name: Work
    backend: azure-boards
    org: my-org
    project: my-project
    team:
      - alice
      - bob
views:
  - name: Weekly Standup
    filters:
      updated_since: last_week
      status:
        - done
        - in-progress
refresh_secs: 600
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := Load()
	if len(cfg.Profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(cfg.Profiles))
	}
	if cfg.Profiles[0].Backend != BackendAzureBoards {
		t.Errorf("expected azure-boards, got %q", cfg.Profiles[0].Backend)
	}
	if cfg.Profiles[0].Org != "my-org" {
		t.Errorf("expected my-org, got %q", cfg.Profiles[0].Org)
	}
	if len(cfg.Profiles[0].Team) != 2 {
		t.Errorf("expected 2 team members, got %d", len(cfg.Profiles[0].Team))
	}
	if len(cfg.Views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(cfg.Views))
	}
	if cfg.Views[0].Name != "Weekly Standup" {
		t.Errorf("expected Weekly Standup, got %q", cfg.Views[0].Name)
	}
	if cfg.Views[0].Filters.UpdatedSince != "last_week" {
		t.Errorf("expected last_week, got %q", cfg.Views[0].Filters.UpdatedSince)
	}
	if cfg.RefreshSecs != 600 {
		t.Errorf("expected 600, got %d", cfg.RefreshSecs)
	}
}

func TestLoad_DefaultRefreshSecs(t *testing.T) {
	dir := t.TempDir()
	xdgPath := filepath.Join(dir, "canopy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdgPath, []byte(`
profiles:
  - name: Test
    backend: github
    owner: test
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := Load()
	if cfg.RefreshSecs != 300 {
		t.Errorf("expected default 300, got %d", cfg.RefreshSecs)
	}
}

func TestLoad_ProfileNameFallback(t *testing.T) {
	dir := t.TempDir()
	xdgPath := filepath.Join(dir, "canopy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdgPath, []byte(`
profiles:
  - backend: github
    owner: test
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := Load()
	if cfg.Profiles[0].Name != "profile 1" {
		t.Errorf("expected fallback name, got %q", cfg.Profiles[0].Name)
	}
}

func TestViewByName(t *testing.T) {
	cfg := Config{
		Views: []View{
			{Name: "Standup"},
			{Name: "Review"},
		},
	}
	v := cfg.ViewByName("Standup")
	if v == nil || v.Name != "Standup" {
		t.Error("expected to find Standup view")
	}
	if cfg.ViewByName("nonexistent") != nil {
		t.Error("expected nil for nonexistent view")
	}
}

func TestMultipleProfiles(t *testing.T) {
	dir := t.TempDir()
	xdgPath := filepath.Join(dir, "canopy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdgPath, []byte(`
profiles:
  - name: Work
    backend: azure-boards
    org: acme
    project: alpha
  - name: OSS
    backend: github
    owner: alice
    repos:
      - repo-a
      - repo-b
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := Load()
	if len(cfg.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cfg.Profiles))
	}
	if cfg.Profiles[1].Backend != BackendGitHub {
		t.Errorf("expected github, got %q", cfg.Profiles[1].Backend)
	}
	if len(cfg.Profiles[1].Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(cfg.Profiles[1].Repos))
	}
}
