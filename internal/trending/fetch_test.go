package trending

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchFallsBackToSearch(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("<html>markup changed</html>"))
	}))
	defer broken.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got == "" {
			t.Errorf("missing search query")
		}
		w.Write([]byte(`{"items":[{"name":"cli","html_url":"https://github.com/acme/cli","language":"Go","stargazers_count":42,"owner":{"login":"acme"}}]}`))
	}))
	defer api.Close()

	oldTrending, oldSearch := trendingURL, searchURL
	trendingURL, searchURL = broken.URL, api.URL
	defer func() { trendingURL, searchURL = oldTrending, oldSearch }()

	res, err := NewClient("").Fetch(context.Background(), Options{Language: "go", Since: Weekly})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "search" {
		t.Errorf("source = %q, want search", res.Source)
	}
	if len(res.Repos) != 1 || res.Repos[0].FullName() != "acme/cli" || res.Repos[0].Stars != 42 {
		t.Errorf("unexpected repos: %+v", res.Repos)
	}
}
