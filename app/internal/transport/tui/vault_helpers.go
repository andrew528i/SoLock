package tui

import (
	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/usecase"
)

const (
	sortByName = usecase.SortByName
	sortByType = usecase.SortByType
	sortByDate = usecase.SortByDate
	sortBySlot = usecase.SortBySlot
	sortModeCount = usecase.SortModeCount
)

var sortLabels = usecase.SortLabels

func (a *App) visibleEntries() []*domain.Entry {
	entries := a.entries
	if a.searching && a.searchInput.Value() != "" {
		entries = usecase.FuzzyFilter(entries, a.searchInput.Value())
	}
	return usecase.SortEntries(entries, a.sortMode)
}
