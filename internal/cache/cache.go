package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Entry wraps cached data with a timestamp and config key for staleness and
// invalidation checks.
type Entry struct {
	CachedAt  time.Time       `json:"cached_at"`
	ConfigKey string          `json:"config_key"`
	Data      json.RawMessage `json:"data"`
}

// Store provides file-based caching in the XDG cache directory. Cached work
// items can be confidential, so files are readable by the owner only.
type Store struct {
	dir       string
	configKey string
}

// New creates a Store at the given directory with a config key for
// invalidation. The directory is created if it does not exist.
func New(dir, configKey string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	// Tighten a directory created by an older version. Entries become 0600
	// as they are rewritten.
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir, configKey: configKey}, nil
}

// Get reads a cached entry. It returns nil if the entry is missing, unreadable,
// older than ttl (when ttl > 0), or written for a different config key.
func (s *Store) Get(key string, ttl time.Duration) *Entry {
	data, err := os.ReadFile(filepath.Join(s.dir, key+".json"))
	if err != nil {
		return nil
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil
	}
	if e.ConfigKey != s.configKey {
		return nil
	}
	if ttl > 0 && time.Since(e.CachedAt) > ttl {
		return nil
	}
	return &e
}

// Set writes data to the cache, stamped with the current time and config key.
// The file is replaced atomically so concurrent writers and readers never see
// a partial entry.
func (s *Store) Set(key string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	b, err := json.Marshal(Entry{
		CachedAt:  time.Now(),
		ConfigKey: s.configKey,
		Data:      raw,
	})
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(s.dir, key+".*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after a successful rename
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(s.dir, key+".json"))
}

// UIState holds persisted UI preferences.
type UIState struct {
	ActiveTab int `json:"active_tab"`
}

// uiStateKey is the cache key used for UIState entries.
const uiStateKey = "state"

// LoadUIState reads the persisted UI state. Returns a zero-value UIState if
// none exists yet.
func (s *Store) LoadUIState() UIState {
	e := s.Get(uiStateKey, 0)
	if e == nil {
		return UIState{}
	}
	var st UIState
	if err := json.Unmarshal(e.Data, &st); err != nil {
		return UIState{}
	}
	return st
}

// SaveUIState persists the UI state to disk.
func (s *Store) SaveUIState(st UIState) error {
	return s.Set(uiStateKey, st)
}
