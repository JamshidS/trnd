package trending

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var searchURL = "https://api.github.com/search/repositories"

type searchResponse struct {
	Items []struct {
		Name        string `json:"name"`
		HTMLURL     string `json:"html_url"`
		Description string `json:"description"`
		Language    string `json:"language"`
		Stars       int    `json:"stargazers_count"`
		Forks       int    `json:"forks_count"`
		Owner       struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"items"`
}

// search approximates trending with the official Search API: repos created
// within the time range, sorted by stars. StarsPeriod is left at 0 because the
// API has no notion of stars gained in a period.
func (c *Client) search(ctx context.Context, opts Options) ([]Repo, error) {
	
	since := time.Now().AddDate(0, 0, -opts.Since.days()).Format("2006-01-02")
	query := "created:>" + since
	if opts.Language != "" {
		query += " language:" + opts.Language
	}
	q := url.Values{}
	q.Set("q", query)
	q.Set("sort", "stars")
	q.Set("order", "desc")
	q.Set("per_page", "25")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search API returned %s", resp.Status)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	repos := make([]Repo, 0, len(sr.Items))
	for _, it := range sr.Items {
		repos = append(repos, Repo{
			Owner:       it.Owner.Login,
			Name:        it.Name,
			URL:         it.HTMLURL,
			Description: it.Description,
			Language:    it.Language,
			Stars:       it.Stars,
			Forks:       it.Forks,
		})
	}
	return repos, nil
}
