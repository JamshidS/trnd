package trending

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var trendingURL = "https://github.com/trending"
var digits = regexp.MustCompile(`[\d,]+`)

func (c *Client) scrape(ctx context.Context, opts Options) ([]Repo, error) {
	u := trendingURL
	if opts.Language != "" {
		u += "/" + url.PathEscape(strings.ToLower(opts.Language))
	}
	
	q := url.Values{}
	q.Set("since", string(opts.Since))
	if opts.SpokenLanguage != "" {
		q.Set("spoken_language_code", opts.SpokenLanguage)
	}
	u += "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github.com/trending returned %s", resp.Status)
	}

	return parseTrending(resp.Body)
}

// parseTrending extracts repos from the trending page HTML.
func parseTrending(r io.Reader) ([]Repo, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}

	var repos []Repo
	doc.Find("article.Box-row").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Find("h2 a").Attr("href")
		if !ok {
			return
		}
		parts := strings.Split(strings.Trim(href, "/"), "/")
		if len(parts) != 2 {
			return
		}
		repo := Repo{
			Owner:       parts[0],
			Name:        parts[1],
			URL:         "https://github.com/" + parts[0] + "/" + parts[1],
			Description: squash(s.Find("p").First().Text()),
			Language:    squash(s.Find(`[itemprop="programmingLanguage"]`).Text()),
			Stars:       atoi(s.Find(`a[href$="/stargazers"]`).Text()),
			Forks:       atoi(s.Find(`a[href$="/forks"]`).Text()),
			StarsPeriod: atoi(s.Find("span.float-sm-right").Text()),
		}
		s.Find("span:contains('Built by') img.avatar").Each(func(_ int, img *goquery.Selection) {
			if alt, ok := img.Attr("alt"); ok {
				repo.BuiltBy = append(repo.BuiltBy, strings.TrimPrefix(alt, "@"))
			}
		})
		repos = append(repos, repo)
	})

	return repos, nil
}

func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.ReplaceAll(digits.FindString(s), ",", ""))
	return n
}
