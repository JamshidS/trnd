package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jamshids/trnd/internal/cache"
	"github.com/jamshids/trnd/internal/github"
	"github.com/jamshids/trnd/internal/history"
	"github.com/jamshids/trnd/internal/trending"
)

type historyTransport func(*http.Request) (*http.Response, error)

func (f historyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFetcherRecordsFullFeedOnlyOnFreshFetch(t *testing.T) {
	transport := historyTransport(func(r *http.Request) (*http.Response, error) {
		body := `<article class="Box-row"><h2><a href="/a/one">one</a></h2></article><article class="Box-row"><h2><a href="/a/two">two</a></h2></article>`
		if r.URL.Host == "api.github.com" {
			body = `{"name":"one","owner":{"login":"a"},"topics":["cli"]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	client, gh := trending.NewClient(""), github.NewClient("")
	client.HTTP.Transport, gh.HTTP.Transport = transport, transport
	store := history.Store{Dir: t.TempDir()}
	listCache := &cache.Cache{Dir: t.TempDir(), TTL: time.Hour}
	topicCache := &cache.Cache{Dir: t.TempDir(), TTL: time.Hour}
	fetch := cachedFetcher(client, gh, listCache, topicCache, store, false)
	opts := trending.Options{Since: trending.Daily, Topics: []string{"missing"}}
	res, cached, err := fetch(context.Background(), opts, false)
	if err != nil || cached || len(res.Repos) != 0 {
		t.Fatalf("first fetch: %+v, cached %v, %v", res, cached, err)
	}
	report, err := store.Read(opts, time.Time{}, "")
	if err != nil || report.Snapshots != 1 || len(report.Entries) != 2 {
		t.Fatalf("full feed not saved: %+v %v", report, err)
	}
	_, cached, err = fetch(context.Background(), opts, false)
	if err != nil || !cached {
		t.Fatalf("cache miss: %v", err)
	}
	report, err = store.Read(opts, time.Time{}, "")
	if err != nil || report.Snapshots != 1 {
		t.Fatalf("cached read recorded: %+v %v", report, err)
	}
	_, cached, err = fetch(context.Background(), opts, true)
	if err != nil || cached {
		t.Fatalf("refresh failed: %v", err)
	}
	report, err = store.Read(opts, time.Time{}, "")
	if err != nil || report.Snapshots != 2 {
		t.Fatalf("refresh not recorded: %+v %v", report, err)
	}

	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	fetch = cachedFetcher(client, gh, listCache, topicCache, history.Store{Dir: blocked}, true)
	res, _, err = fetch(context.Background(), trending.Options{Since: trending.Daily}, false)
	if err != nil || len(res.Repos) != 2 || !strings.Contains(res.Warning, "could not record") {
		t.Fatalf("record failure blocked browsing: %+v %v", res, err)
	}
}

func TestHistoryCommand(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	opts := trending.Options{Language: "go", Since: trending.Weekly}
	err := history.Default().Record(opts, &trending.Result{Source: "trending", FetchedAt: time.Now(), Repos: []trending.Repo{{Owner: "test", Name: "repo", Stars: 100}}})
	if err != nil {
		t.Fatal(err)
	}
	root := newRoot("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"history", "test/repo", "-l", "go", "-s", "weekly", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var report history.Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Snapshots != 1 || len(report.Entries) != 1 || report.Entries[0].Rank != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestHistoryInvalidFlags(t *testing.T) {
	for _, args := range [][]string{{"--days", "0"}, {"--days", "36501"}, {"--topic", "cli"}, {"--sort", "stars"}, {"invalid"}} {
		root := newRoot("test")
		root.SetArgs(append([]string{"history"}, args...))
		if err := root.Execute(); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
