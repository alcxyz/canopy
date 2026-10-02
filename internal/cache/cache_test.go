package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSetGetRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "canopy")
	s, err := New(dir, "key-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set("tasks", map[string]int{"n": 3}); err != nil {
		t.Fatal(err)
	}

	e := s.Get("tasks", time.Hour)
	if e == nil || string(e.Data) != `{"n":3}` {
		t.Fatalf("Get = %+v", e)
	}

	info, err := os.Stat(filepath.Join(dir, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("cache file mode = %o, want 600", perm)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func TestGetRejectsOtherConfigKeyAndExpired(t *testing.T) {
	dir := t.TempDir()
	old, _ := New(dir, "old")
	if err := old.Set("tasks", 1); err != nil {
		t.Fatal(err)
	}
	cur, _ := New(dir, "new")
	if cur.Get("tasks", 0) != nil {
		t.Error("entry from another config key should be ignored")
	}
	time.Sleep(2 * time.Millisecond)
	if old.Get("tasks", time.Millisecond) != nil {
		t.Error("expired entry should be ignored")
	}
	if old.Get("missing", 0) != nil {
		t.Error("missing entry should be nil")
	}
}

func TestUIStateRoundTrip(t *testing.T) {
	s, _ := New(t.TempDir(), "k")
	if got := s.LoadUIState(); got.ActiveTab != 0 {
		t.Errorf("default state = %+v", got)
	}
	if err := s.SaveUIState(UIState{ActiveTab: 2}); err != nil {
		t.Fatal(err)
	}
	if got := s.LoadUIState(); got.ActiveTab != 2 {
		t.Errorf("state = %+v", got)
	}
}
