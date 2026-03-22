package tui

import (
	"context"
	"time"

	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/usecase"
)

const (
	sortByName    = usecase.SortByName
	sortByType    = usecase.SortByType
	sortByDate    = usecase.SortByDate
	sortBySlot    = usecase.SortBySlot
	sortModeCount = usecase.SortModeCount

	groupFilterAll       = -1
	groupFilterUngrouped = -2
)

var sortLabels = usecase.SortLabels

func (a *App) visibleEntries() []*domain.Entry {
	entries := a.entries
	if a.groupFilter >= 0 {
		var filtered []*domain.Entry
		for _, e := range entries {
			if e.GroupIndex() != nil && int(*e.GroupIndex()) == a.groupFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	if a.searching && a.searchInput.Value() != "" {
		entries = usecase.FuzzyFilter(entries, a.searchInput.Value())
	}
	return usecase.SortEntries(entries, a.sortMode)
}

func (a *App) refreshGroups() {
	if a.app.ListGroups == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	groups, err := a.app.ListGroups.Execute(ctx)
	if err != nil {
		return
	}
	a.groups = groups
	a.groupMap = make(map[uint32]*domain.Group, len(groups))
	for _, g := range groups {
		a.groupMap[g.Index()] = g
	}
}

func (a *App) groupName(idx *uint32) string {
	if idx == nil {
		return ""
	}
	if g, ok := a.groupMap[*idx]; ok {
		if g.Deleted() {
			return "[deleted]"
		}
		return g.Name()
	}
	return "[deleted]"
}

func (a *App) activeGroups() []*domain.Group {
	var result []*domain.Group
	for _, g := range a.groups {
		if !g.Deleted() {
			result = append(result, g)
		}
	}
	return result
}

func (a *App) groupFilterLabel() string {
	switch a.groupFilter {
	case groupFilterAll:
		return "all"
	case groupFilterUngrouped:
		return "ungrouped"
	default:
		if g, ok := a.groupMap[uint32(a.groupFilter)]; ok {
			return g.Name()
		}
		return "unknown"
	}
}

func (a *App) cycleGroupFilter() {
	active := a.activeGroups()
	// cycle: all -> group0 -> group1 -> ... -> all
	if a.groupFilter == groupFilterAll {
		if len(active) > 0 {
			a.groupFilter = int(active[0].Index())
		}
	} else {
		found := false
		for i, g := range active {
			if int(g.Index()) == a.groupFilter && i+1 < len(active) {
				a.groupFilter = int(active[i+1].Index())
				found = true
				break
			}
		}
		if !found {
			a.groupFilter = groupFilterAll
		}
	}
	a.entryCursor = 0
}

func (a *App) cycleGroupFilterBack() {
	active := a.activeGroups()
	if len(active) == 0 {
		return
	}
	if a.groupFilter == groupFilterAll {
		a.groupFilter = int(active[len(active)-1].Index())
	} else {
		prev := groupFilterAll
		for _, g := range active {
			if int(g.Index()) == a.groupFilter {
				break
			}
			prev = int(g.Index())
		}
		a.groupFilter = prev
	}
	a.entryCursor = 0
}
