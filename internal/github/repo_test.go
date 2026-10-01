package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamshids/trnd/internal/cache"
	"github.com/jamshids/trnd/internal/trending"
)

func TestEnrichTopics(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/repos/acme/a":
			w.Write([]byte(`{"name":"a","owner":{"login":"acme"},"topics":["go","cli"]}`))
		case "/repos/acme/b":
			w.Write([]byte(`{"name":"b","owner":{"login":"acme"},"topics":[]}`))
		default:
			w.WriteHeader(http.StatusForbidden) // simulate rate limit
		}
	}))
	defer srv.Close()
	old := apiURL
	apiURL = srv.URL
	defer func() { apiURL = old }()

	c := &cache.Cache{Dir: t.TempDir(), TTL: time.Hour}
	repos := []trending.Repo{{Owner: "acme", Name: "a"}, {Owner: "acme", Name: "b"}, {Owner: "acme", Name: "limited"}}

	out, err := NewClient("").EnrichTopics(context.Background(), repos, c)
	if err != ErrRateLimited {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
	if got := out[0].Topics; len(got) != 2 || got[0] != "go" {
		t.Errorf("a topics = %v", got)
	}
	if out[1].Topics == nil || len(out[1].Topics) != 0 {
		t.Errorf("b should have known-empty topics, got %#v", out[1].Topics)
	}
	if out[2].Topics != nil {
		t.Errorf("limited should have unknown topics, got %v", out[2].Topics)
	}
	if repos[0].Topics != nil {
		t.Error("EnrichTopics modified its input")
	}

	// Second call: a and b come from the cache.
	before := hits.Load()
	NewClient("").EnrichTopics(context.Background(), repos[:2], c)
	if hits.Load() != before {
		t.Errorf("expected cache hits, got %d new requests", hits.Load()-before)
	}
}
