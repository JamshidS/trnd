// Package bookmarks persists the user's saved repositories.
package bookmarks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/jamshids/trnd/internal/trending"
)

// Bookmark is a saved repo, as it looked when it was saved.
type Bookmark struct {
	trending.Repo
	SavedAt time.Time `json:"saved_at"`
}

// Store is a JSON file of bookmarks, newest first.
type Store struct {
	path  string
	items []Bookmark
}

// DefaultPath is ~/.local/share/trnd/bookmarks.json on Linux (respecting
// XDG_DATA_HOME) and the user config dir elsewhere.
func DefaultPath() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "trnd", "bookmarks.json")
	}
	if runtime.GOOS == "linux" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "trnd", "bookmarks.json")
		}
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "trnd", "bookmarks.json")
}

// Open loads the store at path; a missing file is an empty store.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.items); err != nil {
		return nil, errors.New("bookmarks file is corrupted: " + path)
	}
	return s, nil
}

// Path returns the file backing the store.
func (s *Store) Path() string { return s.path }

// All returns the bookmarks, newest first.
func (s *Store) All() []Bookmark { return slices.Clone(s.items) }

func (s *Store) index(fullName string) int {
	return slices.IndexFunc(s.items, func(b Bookmark) bool { return strings.EqualFold(b.FullName(), fullName) })
}

// Has reports whether owner/name is saved.
func (s *Store) Has(fullName string) bool { return s.index(fullName) >= 0 }

// Add saves repo (updating it if already saved) and persists the store.
func (s *Store) Add(repo trending.Repo) error {
	if i := s.index(repo.FullName()); i >= 0 {
		s.items = slices.Delete(s.items, i, i+1)
	}
	s.items = slices.Insert(s.items, 0, Bookmark{Repo: repo, SavedAt: time.Now()})
	return s.save()
}

// Remove deletes owner/name, reporting whether it was saved.
func (s *Store) Remove(fullName string) (bool, error) {
	i := s.index(fullName)
	if i < 0 {
		return false, nil
	}
	s.items = slices.Delete(s.items, i, i+1)
	return true, s.save()
}

// Toggle saves repo if it isn't saved, and removes it if it is. It reports
// whether the repo is saved afterwards.
func (s *Store) Toggle(repo trending.Repo) (bool, error) {
	if s.Has(repo.FullName()) {
		_, err := s.Remove(repo.FullName())
		return false, err
	}
	return true, s.Add(repo)
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
