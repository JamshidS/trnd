package bookmarks

import (
	"path/filepath"
	"testing"

	"github.com/jamshids/trnd/internal/trending"
)

func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "bookmarks.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a := trending.Repo{Owner: "acme", Name: "a"}
	b := trending.Repo{Owner: "acme", Name: "b"}
	if err := s.Add(a); err != nil {
		t.Fatal(err)
	}
	if saved, err := s.Toggle(b); err != nil || !saved {
		t.Fatalf("Toggle(b) = %v, %v; want saved", saved, err)
	}
	if !s.Has("ACME/A") {
		t.Error("Has should be case-insensitive")
	}

	// Reopen from disk: newest first.
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	all := s.All()
	if len(all) != 2 || all[0].Name != "b" || all[1].Name != "a" {
		t.Fatalf("All() = %+v, want [b a]", all)
	}
	if all[0].SavedAt.IsZero() {
		t.Error("SavedAt not set")
	}

	if saved, _ := s.Toggle(b); saved {
		t.Error("second Toggle should unsave")
	}
	if ok, _ := s.Remove("acme/a"); !ok {
		t.Error("Remove(acme/a) = false")
	}
	if ok, _ := s.Remove("acme/a"); ok {
		t.Error("removing twice should report false")
	}
	if s, _ := Open(path); len(s.All()) != 0 {
		t.Errorf("expected empty store, got %d", len(s.All()))
	}
}
