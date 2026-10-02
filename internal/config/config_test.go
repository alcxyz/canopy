package config

import (
	"os"
	"path/filepath"
	"strings"
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

	cfg, _ := Load()
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

	cfg, _ := Load()
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

	cfg, _ := Load()
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

	cfg, _ := Load()
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

func TestParse_ReportsUnknownKeysButKeepsConfig(t *testing.T) {
	cfg, problems := parse([]byte(`
profiles:
  - name: Work
    backend: azure-boards
    org: o
    project: p
    token_file: /run/secret
`))
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Org != "o" {
		t.Fatalf("lenient decode lost the profile: %+v", cfg.Profiles)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "token_file") {
		t.Errorf("problems = %q", problems)
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	cfg, problems := parse([]byte("profiles: [\n"))
	if len(cfg.Profiles) != 0 || cfg.RefreshSecs != Default.RefreshSecs {
		t.Errorf("invalid YAML should yield defaults, got %+v", cfg)
	}
	if len(problems) != 1 {
		t.Errorf("problems = %q", problems)
	}
}

func TestParse_EmptyFile(t *testing.T) {
	if _, problems := parse(nil); len(problems) != 0 {
		t.Errorf("empty config should not be a problem: %q", problems)
	}
}

func TestProblems_InvalidViewFilters(t *testing.T) {
	cfg := Config{Views: []View{{Name: "V", Filters: Filter{
		Types:        []string{"feature", "story"},
		Status:       []string{"in_progress"},
		UpdatedSince: "last_fortnight",
	}}}}
	if got := cfg.Problems(); len(got) != 3 {
		t.Errorf("problems = %q", got)
	}
}

func TestUpdatedSinceDays(t *testing.T) {
	cases := map[string]int{"today": 0, "last_week": 7, "last_45_days": 45}
	for in, want := range cases {
		if got, ok := UpdatedSinceDays(in); !ok || got != want {
			t.Errorf("UpdatedSinceDays(%q) = %d, %v", in, got, ok)
		}
	}
	for _, in := range []string{"someday", "last_x_days", "last_-3_days"} {
		if _, ok := UpdatedSinceDays(in); ok {
			t.Errorf("UpdatedSinceDays(%q) should be invalid", in)
		}
	}
}

func TestExampleConfigParsesCleanly(t *testing.T) {
	data, err := os.ReadFile("../../cmd/canopy/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, problems := parse(data); len(problems) != 0 {
		t.Errorf("example config problems: %q", problems)
	}
}

func TestParse_WrongTypeKeepsOtherFields(t *testing.T) {
	cfg, problems := parse([]byte(`
profiles:
  - name: Work
    backend: azure-boards
    org: o
    project: p
refresh_secs: 5m
`))
	if len(cfg.Profiles) != 1 {
		t.Fatalf("profiles lost on a type error: %+v", cfg)
	}
	if cfg.RefreshSecs != Default.RefreshSecs {
		t.Errorf("refresh_secs = %d, want default", cfg.RefreshSecs)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "5m") {
		t.Errorf("problems = %q", problems)
	}
}

func TestUniqueProfileNamesAvoidsExistingNames(t *testing.T) {
	profiles := []Profile{{Name: "Work (2)"}, {Name: "Work"}, {Name: "Work"}, {}}
	problems := uniqueProfileNames(profiles)
	seen := map[string]bool{}
	for _, p := range profiles {
		if seen[p.Name] {
			t.Fatalf("duplicate name %q in %+v", p.Name, profiles)
		}
		seen[p.Name] = true
	}
	if profiles[2].Name != "Work (3)" || profiles[3].Name != "profile 4" || len(problems) != 1 {
		t.Errorf("profiles = %+v, problems = %q", profiles, problems)
	}
}

func TestParse_DuplicateProfileNamesAreMadeUnique(t *testing.T) {
	cfg, problems := parse([]byte(`
profiles:
  - name: Work
    backend: azure-boards
    org: a
    project: p
  - name: Work
    backend: azure-boards
    org: b
    project: p
`))
	if cfg.Profiles[0].Name == cfg.Profiles[1].Name {
		t.Fatalf("names not unique: %q", cfg.Profiles[1].Name)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "duplicate") {
		t.Errorf("problems = %q", problems)
	}
}
