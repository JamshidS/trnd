package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/jamshids/trnd/internal/cache"
	"github.com/jamshids/trnd/internal/trending"
)

// ErrRateLimited means GitHub refused the request because of rate limits.
var ErrRateLimited = errors.New("GitHub API rate limit hit; set GITHUB_TOKEN (or run `gh auth login`) for higher limits")

type repoResponse struct {
	Name        string   `json:"name"`
	HTMLURL     string   `json:"html_url"`
	Description string   `json:"description"`
	Language    string   `json:"language"`
	Stars       int      `json:"stargazers_count"`
	Forks       int      `json:"forks_count"`
	Topics      []string `json:"topics"`
	Owner       struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// Repo fetches a repository's details, including its topics.
func (c *Client) Repo(ctx context.Context, fullName string) (trending.Repo, error) {
	fullName = strings.Trim(strings.TrimPrefix(strings.TrimPrefix(fullName, "https://"), "github.com/"), "/")
	if strings.Count(fullName, "/") != 1 {
		return trending.Repo{}, fmt.Errorf("%q is not in owner/name form", fullName)
	}
	resp, err := c.do(ctx, http.MethodGet, "/repos/"+fullName, "application/vnd.github+json")
	if err != nil {
		return trending.Repo{}, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return trending.Repo{}, fmt.Errorf("repository %s not found", fullName)
	case http.StatusForbidden, http.StatusTooManyRequests:
		return trending.Repo{}, ErrRateLimited
	default:
		return trending.Repo{}, fmt.Errorf("fetching %s: %s", fullName, resp.Status)
	}
	var r repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return trending.Repo{}, err
	}
	return trending.Repo{
		Owner: r.Owner.Login, Name: r.Name, URL: r.HTMLURL, Description: r.Description,
		Language: r.Language, Stars: r.Stars, Forks: r.Forks, Topics: r.Topics,
	}, nil
}

// EnrichTopics fills in Topics for each repo, using c (if non-nil) to avoid
// refetching. It returns the repos it could enrich; err is non-nil if any
// lookup failed, so callers can warn about incomplete results.
func (c *Client) EnrichTopics(ctx context.Context, repos []trending.Repo, tc *cache.Cache) ([]trending.Repo, error) {
	out := make([]trending.Repo, len(repos))
	copy(out, repos)

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		sem      = make(chan struct{}, 8)
	)
	for i := range out {
		key := "topics|" + strings.ToLower(out[i].FullName())
		var topics []string
		if tc != nil && tc.Get(key, &topics) {
			out[i].Topics = topics
			continue
		}
		wg.Add(1)
		go func(i int, key string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			r, err := c.Repo(ctx, out[i].FullName())
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			topics := r.Topics
			if topics == nil {
				topics = []string{} // cache "no topics" too
			}
			out[i].Topics = topics
			if tc != nil {
				tc.Set(key, topics)
			}
		}(i, key)
	}
	wg.Wait()
	return out, firstErr
}
