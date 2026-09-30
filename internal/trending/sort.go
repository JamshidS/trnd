package trending

import (
	"fmt"
	"slices"
	"strings"
)

// SortBy orders a list of repos.
type SortBy string

const (
	SortTrending SortBy = "trending" // GitHub's order (or saved order for bookmarks)
	SortGained   SortBy = "gained"   // stars gained in the period
	SortStars    SortBy = "stars"    // total stars
	SortForks    SortBy = "forks"
	SortName     SortBy = "name"
)

var sortOrder = []SortBy{SortTrending, SortGained, SortStars, SortForks, SortName}

// ParseSort validates a sort key.
func ParseSort(s string) (SortBy, error) {
	switch SortBy(strings.ToLower(s)) {
	case SortTrending, "", "rank", "default":
		return SortTrending, nil
	case SortGained, "today", "period":
		return SortGained, nil
	case SortStars:
		return SortStars, nil
	case SortForks:
		return SortForks, nil
	case SortName:
		return SortName, nil
	}
	return "", fmt.Errorf("invalid sort %q (use trending, gained, stars, forks or name)", s)
}

// Next cycles through the sort keys.
func (s SortBy) Next() SortBy {
	i := slices.Index(sortOrder, s)
	return sortOrder[(i+1)%len(sortOrder)]
}

// Sort returns a sorted copy of repos. Ties keep their original order.
func Sort(repos []Repo, by SortBy) []Repo {
	out := slices.Clone(repos)
	var cmp func(a, b Repo) int
	switch by {
	case SortGained:
		cmp = func(a, b Repo) int { return b.StarsPeriod - a.StarsPeriod }
	case SortStars:
		cmp = func(a, b Repo) int { return b.Stars - a.Stars }
	case SortForks:
		cmp = func(a, b Repo) int { return b.Forks - a.Forks }
	case SortName:
		cmp = func(a, b Repo) int {
			return strings.Compare(strings.ToLower(a.FullName()), strings.ToLower(b.FullName()))
		}
	default:
		return out
	}
	slices.SortStableFunc(out, cmp)
	return out
}

// ParseTopics splits "LLM, machine learning" into ["llm", "machine-learning"],
// matching how GitHub normalizes topic names.
func ParseTopics(s string) []string {
	var topics []string
	for _, t := range strings.Split(s, ",") {
		t = strings.Join(strings.Fields(strings.ToLower(t)), "-")
		t = strings.TrimPrefix(t, "#")
		if t != "" && !slices.Contains(topics, t) {
			topics = append(topics, t)
		}
	}
	return topics
}

// FilterTopics keeps repos that have at least one of the wanted topics.
func FilterTopics(repos []Repo, want []string) []Repo {
	if len(want) == 0 {
		return repos
	}
	var out []Repo
	for _, r := range repos {
		if slices.ContainsFunc(r.Topics, func(t string) bool { return slices.Contains(want, t) }) {
			out = append(out, r)
		}
	}
	return out
}
