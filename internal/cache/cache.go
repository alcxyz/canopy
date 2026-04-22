package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Entry wraps cached data with a timestamp for TTL checks.
type Entry struct {
	CachedAt time.Time       `json:"cached_at"`
	Data     json.RawMessage `json:"data"`
}

// Store provides file-based caching in the XDG cache directory.
type Store struct {
	dir string
}

// New creates a Store at the given directory, creating it if needed.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Get reads a cached entry. Returns nil if missing or expired.
func (s *Store) Get(key string, ttl time.Duration) (*Entry, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, key+".json"))
	if err != nil {
		return nil, nil
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, nil
	}
	if time.Since(e.CachedAt) > ttl {
		return nil, nil
	}
	return &e, nil
}

// Set writes data to the cache.
func (s *Store) Set(key string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	e := Entry{
		CachedAt: time.Now(),
		Data:     raw,
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, key+".json"), b, 0o644)
}
