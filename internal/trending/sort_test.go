package trending

import (
	"slices"
	"testing"
)

func names(repos []Repo) []string {
	var out []string
	for _, r := range repos {
		out = append(out, r.Name)
	}
	return out
}

func TestSort(t *testing.T) {
	repos := []Repo{
		{Owner: "x", Name: "b", Stars: 10, StarsPeriod: 5, Forks: 1},
		{Owner: "x", Name: "a", Stars: 30, StarsPeriod: 1, Forks: 9},
		{Owner: "x", Name: "c", Stars: 20, StarsPeriod: 5, Forks: 3},
	}
	for by, want := range map[SortBy][]string{
		SortTrending: {"b", "a", "c"},
		SortGained:   {"b", "c", "a"}, // tie keeps original order
		SortStars:    {"a", "c", "b"},
		SortForks:    {"a", "c", "b"},
		SortName:     {"a", "b", "c"},
	} {
		if got := names(Sort(repos, by)); !slices.Equal(got, want) {
			t.Errorf("Sort(%s) = %v, want %v", by, got, want)
		}
	}
	if repos[0].Name != "b" {
		t.Error("Sort modified its input")
	}
}

func TestTopics(t *testing.T) {
	if got := ParseTopics(" LLM, machine learning ,#cli,llm,"); !slices.Equal(got, []string{"llm", "machine-learning", "cli"}) {
		t.Errorf("ParseTopics = %v", got)
	}
	repos := []Repo{
		{Name: "a", Topics: []string{"go", "cli"}},
		{Name: "b", Topics: []string{"llm"}},
		{Name: "c"},
	}
	if got := names(FilterTopics(repos, []string{"cli", "llm"})); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("FilterTopics = %v", got)
	}
	if got := FilterTopics(repos, nil); len(got) != 3 {
		t.Errorf("empty filter should keep all, got %d", len(got))
	}
}
