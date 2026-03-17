package tui

import (
	"sort"
	"strings"

	"github.com/solock/solock/internal/domain"
)

const (
	sortByName = iota
	sortByType
	sortByDate
	sortBySlot
	sortModeCount
)

var sortLabels = []string{"name", "type", "date", "slot"}

func (a *App) visibleEntries() []*domain.Entry {
	entries := a.entries
	if a.searching && a.searchInput.Value() != "" {
		entries = fuzzyFilter(entries, a.searchInput.Value())
	}
	return sortEntries(entries, a.sortMode)
}

func fuzzyFilter(entries []*domain.Entry, query string) []*domain.Entry {
	query = strings.ToLower(query)
	var result []*domain.Entry
	for _, e := range entries {
		if fuzzyMatch(strings.ToLower(e.Name()), query) ||
			fuzzyMatch(strings.ToLower(e.Field("username")), query) ||
			fuzzyMatch(strings.ToLower(e.Field("site")), query) {
			result = append(result, e)
		}
	}
	return result
}

func fuzzyMatch(s, pattern string) bool {
	if strings.Contains(s, pattern) {
		return true
	}
	pi := 0
	for i := 0; i < len(s) && pi < len(pattern); i++ {
		if s[i] == pattern[pi] {
			pi++
		}
	}
	return pi == len(pattern)
}

func sortEntries(entries []*domain.Entry, mode int) []*domain.Entry {
	if len(entries) <= 1 {
		return entries
	}
	sorted := make([]*domain.Entry, len(entries))
	copy(sorted, entries)

	switch mode {
	case sortByName:
		sort.Slice(sorted, func(i, j int) bool {
			return strings.ToLower(sorted[i].Name()) < strings.ToLower(sorted[j].Name())
		})
	case sortByType:
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Type() != sorted[j].Type() {
				return sorted[i].Type() < sorted[j].Type()
			}
			return strings.ToLower(sorted[i].Name()) < strings.ToLower(sorted[j].Name())
		})
	case sortByDate:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].UpdatedAt().After(sorted[j].UpdatedAt())
		})
	case sortBySlot:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].SlotIndex() < sorted[j].SlotIndex()
		})
	}
	return sorted
}
