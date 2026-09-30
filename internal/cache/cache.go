package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"	
)

// Cache stores JSON blobs under DIR, expiring them after TTL. 
type Cache struct {
	Dir string
	TTL time.Duration
}

// New return a cache in the user cache dir (e.g. $HOME/.cache/trnd)
func New(ttl time.Duration) *Cache {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	
	return &Cache{Dir: filepath.Join(dir, "trnd"), TTL: ttl}
}

func (c *Cache) path(key string) string {
	sum := sha1.Sum([]byte(key))
	return filepath.Join(c.Dir, hex.EncodeToString(sum[:8])+".json")
}


// Get decodes the cached value for key into v. It reports false on miss
// or when the entry is expired. 
func (c *Cache) Get(key string, v any) bool {
	p := c.path(key)
	info, err := os.Stat(p)
	if err != nil || time.Since(info.ModTime()) > c.TTL {
		return false
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return false
	}

	return json.Unmarshal(data, v) == nil
}

// Set stores v under key. Errors are ignored: the cache is best-effort.
func (c *Cache) Set(key string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return
	}

	tmp := c.path(key) + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, c.path(key))
	}
}