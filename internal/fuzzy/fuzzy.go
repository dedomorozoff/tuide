// Package fuzzy matches short patterns against names the way a command palette
// does: every pattern character must appear in order, and matches at word
// boundaries and in runs score higher.
package fuzzy

import "strings"

// Match reports whether pattern is a subsequence of target, ignoring case, and
// returns a score where higher means a better match. An empty pattern matches
// everything with a score of zero.
func Match(pattern, target string) (int, bool) {
	if pattern == "" {
		return 0, true
	}

	pattern = strings.ToLower(pattern)
	target = strings.ToLower(target)

	patternRunes := []rune(pattern)
	targetRunes := []rune(target)

	score := 0
	matched := 0
	previous := -2

	for index, r := range targetRunes {
		if matched >= len(patternRunes) {
			break
		}
		if r != patternRunes[matched] {
			continue
		}

		score += 2
		if index == previous+1 {
			score += 3
		}
		if index == 0 || isBoundary(targetRunes[index-1]) {
			score += 4
		}

		previous = index
		matched++
	}

	if matched < len(patternRunes) {
		return 0, false
	}
	return score, true
}

// Rank returns the indices of the candidates that match pattern, best first.
// Ties keep the original order.
func Rank(pattern string, candidates []string) []int {
	type scored struct {
		index int
		score int
	}

	var matched []scored
	for index, candidate := range candidates {
		score, ok := Match(pattern, candidate)
		if !ok {
			continue
		}
		matched = append(matched, scored{index: index, score: score})
	}

	// Insertion sort by score, descending, so equal scores stay stable.
	for i := 1; i < len(matched); i++ {
		for j := i; j > 0 && matched[j].score > matched[j-1].score; j-- {
			matched[j], matched[j-1] = matched[j-1], matched[j]
		}
	}

	ranked := make([]int, len(matched))
	for i, entry := range matched {
		ranked[i] = entry.index
	}
	return ranked
}

func isBoundary(r rune) bool {
	switch r {
	case ' ', '-', '_', '.', '/', ':':
		return true
	}
	return false
}
