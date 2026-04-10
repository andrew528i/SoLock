package usecase

import (
	"context"
	"sort"
	"strings"

	"github.com/solock/solock/internal/domain"
)

type ListEntriesUseCase struct {
	entries domain.EntryRepository
}

func NewListEntriesUseCase(entries domain.EntryRepository) *ListEntriesUseCase {
	return &ListEntriesUseCase{entries: entries}
}

func (uc *ListEntriesUseCase) Execute(ctx context.Context) ([]*domain.Entry, error) {
	return uc.entries.List(ctx)
}

type SearchEntriesUseCase struct {
	entries domain.EntryRepository
	groups  domain.GroupRepository
}

func NewSearchEntriesUseCase(entries domain.EntryRepository, groups domain.GroupRepository) *SearchEntriesUseCase {
	return &SearchEntriesUseCase{entries: entries, groups: groups}
}

type SearchResult struct {
	Entries []*domain.Entry
}

func (uc *SearchEntriesUseCase) Execute(ctx context.Context, query string) (*SearchResult, error) {
	all, err := uc.entries.List(ctx)
	if err != nil {
		return nil, err
	}
	if query == "" {
		return &SearchResult{Entries: all}, nil
	}
	var groups []*domain.Group
	if uc.groups != nil {
		groups, _ = uc.groups.List(ctx)
	}
	return &SearchResult{Entries: FuzzyFilterWithGroups(all, groups, query)}, nil
}

func FuzzyFilter(entries []*domain.Entry, query string) []*domain.Entry {
	return FuzzyFilterWithGroups(entries, nil, query)
}

func FuzzyFilterWithGroups(entries []*domain.Entry, groups []*domain.Group, query string) []*domain.Entry {
	query = strings.ToLower(query)
	if query == "" {
		return entries
	}

	groupNames := make(map[uint32]string, len(groups))
	for _, g := range groups {
		groupNames[g.Index()] = strings.ToLower(g.Name())
	}

	result := make([]*domain.Entry, 0, len(entries))
	for _, e := range entries {
		if entryMatches(e, groupNames, query) {
			result = append(result, e)
		}
	}
	return result
}

func entryMatches(e *domain.Entry, groupNames map[uint32]string, query string) bool {
	if fuzzyMatch(strings.ToLower(e.Name()), query) {
		return true
	}
	if fuzzyMatch(strings.ToLower(string(e.Type())), query) {
		return true
	}
	for _, v := range e.Fields() {
		if v == "" {
			continue
		}
		if fuzzyMatch(strings.ToLower(v), query) {
			return true
		}
	}
	if gi := e.GroupIndex(); gi != nil {
		if name, ok := groupNames[*gi]; ok && fuzzyMatch(name, query) {
			return true
		}
	}
	return false
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

const (
	SortByName = iota
	SortByType
	SortByDate
	SortBySlot
	SortModeCount
)

var SortLabels = []string{"name", "type", "date", "slot"}

func SortEntries(entries []*domain.Entry, mode int) []*domain.Entry {
	if len(entries) <= 1 {
		return entries
	}
	sorted := make([]*domain.Entry, len(entries))
	copy(sorted, entries)

	switch mode {
	case SortByName:
		sort.Slice(sorted, func(i, j int) bool {
			return strings.ToLower(sorted[i].Name()) < strings.ToLower(sorted[j].Name())
		})
	case SortByType:
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Type() != sorted[j].Type() {
				return sorted[i].Type() < sorted[j].Type()
			}
			return strings.ToLower(sorted[i].Name()) < strings.ToLower(sorted[j].Name())
		})
	case SortByDate:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].UpdatedAt().After(sorted[j].UpdatedAt())
		})
	case SortBySlot:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].SlotIndex() < sorted[j].SlotIndex()
		})
	}
	return sorted
}
