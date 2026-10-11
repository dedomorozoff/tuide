package fuzzy

import (
	"slices"
	"testing"
)

func TestMatchSubsequence(t *testing.T) {
	cases := []struct {
		pattern string
		target  string
		want    bool
	}{
		{"no", "Notes", true},
		{"nt", "Notes", true},
		{"cl", "Calculator", true},
		{"clock", "Clock", true},
		{"xyz", "Notes", false},
		{"sn", "Notes", false},
		{"", "Anything", true},
		{"NOTES", "notes", true},
		{"c a", "Calculator", false},
	}

	for _, c := range cases {
		if _, ok := Match(c.pattern, c.target); ok != c.want {
			t.Errorf("Match(%q, %q) matched = %v, want %v", c.pattern, c.target, ok, c.want)
		}
	}
}

func TestMatchPrefersBoundaries(t *testing.T) {
	boundary, _ := Match("c", "Clock")
	middle, _ := Match("c", "Ascend")

	if boundary <= middle {
		t.Errorf("a leading-letter match should outscore a middle one: %d vs %d", boundary, middle)
	}
}

func TestMatchPrefersRuns(t *testing.T) {
	run, _ := Match("no", "Notes")
	split, _ := Match("no", "Nxo")

	if run <= split {
		t.Errorf("adjacent characters should outscore a split match: %d vs %d", run, split)
	}
}

func TestRankOrdersBestFirst(t *testing.T) {
	candidates := []string{"Welcome", "Notes", "Clock", "Calculator"}
	got := Rank("cl", candidates)

	if len(got) != 2 {
		t.Fatalf("Rank returned %d matches, want 2: %v", len(got), got)
	}
	if candidates[got[0]] != "Clock" {
		t.Errorf("best match = %q, want Clock", candidates[got[0]])
	}
	if !slices.Contains(got, 3) {
		t.Errorf("Calculator should match: %v", got)
	}
}

func TestRankKeepsOrderOnTies(t *testing.T) {
	candidates := []string{"Apple", "Apricot", "Avocado", "Almond"}
	got := Rank("a", candidates)

	if !slices.Equal(got, []int{0, 1, 2, 3}) {
		t.Errorf("Rank = %v, want the original order for equal scores", got)
	}
}

func TestRankEmptyPatternKeepsEverything(t *testing.T) {
	candidates := []string{"a", "b", "c"}
	got := Rank("", candidates)

	if !slices.Equal(got, []int{0, 1, 2}) {
		t.Errorf("Rank with an empty pattern = %v, want every index", got)
	}
}
