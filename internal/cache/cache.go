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

// Store provides file-based caching in the XDG cache directory.
type Store struct {
	dir       string
	configKey string
}

// New creates a Store at the given directory with a config key for
// invalidation. The directory is created if it does not exist.
func New(dir, configKey string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir, configKey: configKey}, nil
}

// Get reads a cached entry. Returns nil if missing, expired, or if the
// config key has changed since the entry was written.
func (s *Store) Get(key string, ttl time.Duration) (*Entry, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, key+".json"))
	if err != nil {
		return nil, nil
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, nil
	}
	if e.ConfigKey != s.configKey {
		return nil, nil
	}
	if ttl > 0 && time.Since(e.CachedAt) > ttl {
		return nil, nil
	}
	return &e, nil
}

// Set writes data to the cache, stamped with the current time and config key.
func (s *Store) Set(key string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	e := Entry{
		CachedAt:  time.Now(),
		ConfigKey: s.configKey,
		Data:      raw,
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, key+".json"), b, 0o644)
}
