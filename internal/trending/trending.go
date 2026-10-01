// Package trending fetches GitHub's trending repositories.
//
// GitHub has no official trending API, so the primary source is the
// github.com/trending HTML page. If scraping fails (e.g. GitHub changes its
// markup), Fetch falls back to the official Search API, which approximates
// trending as "recently created repos with the most stars".
package trending

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Repo is a single trending repository.
type Repo struct {
	Owner       string   `json:"owner"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	Language    string   `json:"language"`
	Stars       int      `json:"stars"`
	Forks       int      `json:"forks"`
	StarsPeriod int      `json:"stars_period"`
	BuiltBy     []string `json:"built_by,omitempty"`
	Topics      []string `json:"topics,omitempty"` // only filled when a topic filter is used
}

// FullName returns "owner/name".
func (r Repo) FullName() string { return r.Owner + "/" + r.Name }

// CloneURL returns the HTTPS clone URL.
func (r Repo) CloneURL() string { return r.URL + ".git" }

// Since is the trending time range.
type Since string

const (
	Daily   Since = "daily"
	Weekly  Since = "weekly"
	Monthly Since = "monthly"
)

// ParseSince validates a time range string.
func ParseSince(s string) (Since, error) {
	switch Since(strings.ToLower(s)) {
	case Daily, "day", "today", "":
		return Daily, nil
	case Weekly, "week":
		return Weekly, nil
	case Monthly, "month":
		return Monthly, nil
	}
	return "", fmt.Errorf("invalid time range %q (use daily, weekly or monthly)", s)
}

// Next cycles daily -> weekly -> monthly -> daily.
func (s Since) Next() Since {
	switch s {
	case Daily:
		return Weekly
	case Weekly:
		return Monthly
	}
	return Daily
}

// PeriodLabel is the human label for StarsPeriod, e.g. "today".
func (s Since) PeriodLabel() string {
	switch s {
	case Weekly:
		return "this week"
	case Monthly:
		return "this month"
	}
	return "today"
}

func (s Since) days() int {
	switch s {
	case Weekly:
		return 7
	case Monthly:
		return 30
	}
	return 1
}

// Options controls what is fetched.
type Options struct {
	Language       string // programming language slug, e.g. "go"; empty = all
	Since          Since
	SpokenLanguage string   // ISO 639-1 code, e.g. "en"; empty = all
	Topics         []string // keep repos having any of these topics; empty = all
}

// Result is the outcome of a fetch.
type Result struct {
	Repos     []Repo    `json:"repos"`
	Source    string    `json:"source"` // "trending" or "search"
	FetchedAt time.Time `json:"fetched_at"`
	Warning   string    `json:"warning,omitempty"`
}

// Client fetches trending data.
type Client struct {
	HTTP  *http.Client
	Token string // optional GitHub token, used for the Search API fallback
}

// NewClient returns a Client with sane timeouts.
func NewClient(token string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}, Token: token}
}

// Fetch scrapes github.com/trending and falls back to the Search API on failure.
func (c *Client) Fetch(ctx context.Context, opts Options) (*Result, error) {
	repos, scrapeErr := c.scrape(ctx, opts)
	if scrapeErr == nil && len(repos) > 0 {
		return &Result{Repos: repos, Source: "trending", FetchedAt: time.Now()}, nil
	}
	repos, searchErr := c.search(ctx, opts)
	if searchErr != nil {
		if scrapeErr == nil {
			scrapeErr = fmt.Errorf("no repositories found")
		}
		return nil, fmt.Errorf("scrape failed: %v; search fallback failed: %v", scrapeErr, searchErr)
	}
	return &Result{Repos: repos, Source: "search", FetchedAt: time.Now()}, nil
}

const userAgent = "trnd (+https://github.com/jamshids/trnd)"
