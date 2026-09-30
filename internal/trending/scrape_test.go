package trending

import (
	"os"
	"testing"
)

func TestParseTrending(t *testing.T) {
	f, err := os.Open("testdata/trending.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	repos, err := parseTrending(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) == 0 {
		t.Fatal("expected repos, got none")
	}

	r := repos[0]
	if r.Owner != "NVIDIA" || r.Name != "OpenShell" {
		t.Errorf("got %s, want NVIDIA/OpenShell", r.FullName())
	}
	if r.Language != "Rust" {
		t.Errorf("language = %q, want Rust", r.Language)
	}
	if r.Stars != 12422 || r.Forks != 1533 || r.StarsPeriod != 1280 {
		t.Errorf("stars/forks/period = %d/%d/%d, want 12422/1533/1280", r.Stars, r.Forks, r.StarsPeriod)
	}
	if r.Description == "" {
		t.Error("expected description")
	}
	if len(r.BuiltBy) == 0 || r.BuiltBy[0] != "drew" {
		t.Errorf("built by = %v, want first = drew", r.BuiltBy)
	}
}

func TestParseSince(t *testing.T) {
	for in, want := range map[string]Since{"": Daily, "week": Weekly, "Monthly": Monthly} {
		got, err := ParseSince(in)
		if err != nil || got != want {
			t.Errorf("ParseSince(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ParseSince("yearly"); err == nil {
		t.Error("expected error for yearly")
	}
}
