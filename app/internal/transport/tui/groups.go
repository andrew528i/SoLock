package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type groupAddedMsg struct{ err error }
type groupUpdatedMsg struct{ err error }
type groupDeletedMsg struct{ err error }
type groupPurgedMsg struct{ err error }

func (a *App) updateGroupList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case groupDeletedMsg:
		a.loading = false
		if msg.err != nil {
			a.addLog("Delete failed: " + msg.err.Error())
		} else {
			a.addLog("Group deleted")
		}
		a.refreshGroups()
		return a, nil
	case groupPurgedMsg:
		a.loading = false
		if msg.err != nil {
			a.addLog("Purge failed: " + msg.err.Error())
		} else {
			a.addLog("Group purged")
		}
		a.refreshGroups()
		if a.groupCursor >= len(a.groups) && len(a.groups) > 0 {
			a.groupCursor = len(a.groups) - 1
		}
		return a, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			a.screen = screenDashboard
			return a, nil
		case "j", "down":
			if a.groupCursor < len(a.groups)-1 {
				a.groupCursor++
			}
			return a, nil
		case "k", "up":
			if a.groupCursor > 0 {
				a.groupCursor--
			}
			return a, nil
		case "a":
			a.groupNameInput.SetValue("")
			a.groupNameInput.Focus()
			a.editingGroup = nil
			a.screen = screenGroupAdd
			return a, nil
		case "e":
			if len(a.groups) > 0 && a.groupCursor < len(a.groups) {
				g := a.groups[a.groupCursor]
				if g.Deleted() {
					a.addLog("Cannot edit deleted group")
					return a, nil
				}
				a.groupNameInput.SetValue(g.Name())
				a.groupNameInput.Focus()
				a.editingGroup = g
				a.screen = screenGroupEdit
			}
			return a, nil
		case "x":
			if len(a.groups) > 0 && a.groupCursor < len(a.groups) {
				g := a.groups[a.groupCursor]
				if g.Deleted() {
					a.confirmMsg = fmt.Sprintf("Purge group %q permanently?", g.Name())
					a.prev = screenGroupList
					idx := g.Index()
					a.confirmAction = func() tea.Cmd {
						return a.purgeGroup(idx)
					}
					a.screen = screenConfirm
				} else {
					a.confirmMsg = fmt.Sprintf("Delete group %q? Entries will be kept.", g.Name())
					a.prev = screenGroupList
					idx := g.Index()
					a.confirmAction = func() tea.Cmd {
						return a.deleteGroup(idx)
					}
					a.screen = screenConfirm
				}
			}
			return a, nil
		}
	}
	return a, nil
}

func (a *App) updateGroupForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case groupAddedMsg:
		a.loading = false
		if msg.err != nil {
			a.addLog("Add failed: " + msg.err.Error())
			return a, nil
		}
		a.addLog("Group created")
		a.refreshGroups()
		a.screen = screenGroupList
		return a, nil
	case groupUpdatedMsg:
		a.loading = false
		if msg.err != nil {
			a.addLog("Update failed: " + msg.err.Error())
			return a, nil
		}
		a.addLog("Group renamed")
		a.refreshGroups()
		a.screen = screenGroupList
		return a, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			a.screen = screenGroupList
			return a, nil
		case "enter":
			name := strings.TrimSpace(a.groupNameInput.Value())
			if name == "" {
				a.addLog("Name cannot be empty")
				return a, nil
			}
			a.loading = true
			if a.screen == screenGroupAdd {
				return a, a.addGroup(name)
			}
			if a.editingGroup != nil {
				a.editingGroup.SetName(name)
				return a, a.updateGroup(a.editingGroup)
			}
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.groupNameInput, cmd = a.groupNameInput.Update(msg)
	return a, cmd
}

func (a *App) viewGroupList() string {
	var lines []string

	header := "  " + titleStyle.Render("Groups")
	if len(a.groups) > 0 {
		active := 0
		for _, g := range a.groups {
			if !g.Deleted() {
				active++
			}
		}
		header += "  " + dimStyle.Render(fmt.Sprintf("%d groups", active))
	}
	lines = append(lines, "", header, "")

	if len(a.groups) == 0 {
		lines = append(lines, "  "+dimStyle.Render("No groups yet. Press 'a' to create one."))
	} else {
		maxVisible := max(a.height-9, 5)
		start := 0
		if a.groupCursor >= maxVisible {
			start = a.groupCursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(a.groups))

		for i := start; i < end; i++ {
			g := a.groups[i]
			selected := i == a.groupCursor

			cursor := "  "
			nStyle := dimStyle
			if selected {
				cursor = accentStyle.Render("> ")
				nStyle = valueStyle
			}

			line := cursor
			line += slotStyle.Render(fmt.Sprintf("#%-3d", g.Index())) + " "

			if g.Deleted() {
				line += dangerStyle.Render(g.Name()) + " "
				line += dimStyle.Render("[deleted]")
			} else {
				line += nStyle.Render(g.Name())
			}

			lines = append(lines, line)
		}

		if len(a.groups) > maxVisible {
			scrollInfo := fmt.Sprintf("%d-%d of %d", start+1, end, len(a.groups))
			lines = append(lines, "  "+dimStyle.Render(scrollInfo))
		}
	}

	lines = append(lines, "", a.loadingLine())
	lines = append(lines, helpBar(
		helpKey("jk", "nav"), helpKey("a", "add"), helpKey("e", "rename"),
		helpKey("x", "del/purge"), helpKey("esc", "back"),
	))
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) viewGroupForm() string {
	var lines []string

	title := "Add Group"
	if a.screen == screenGroupEdit {
		title = "Rename Group"
	}
	lines = append(lines, "", "  "+titleStyle.Render(title), "")

	w := formWidth(a.width)
	a.groupNameInput.Width = w

	lines = append(lines, "  "+labelStyle.Render("Name"))
	lines = append(lines, inputBox(a.groupNameInput.View(), true, w, "  "))

	lines = append(lines, "")
	if a.loading {
		lines = append(lines, "  "+a.spinner.View()+" "+dimStyle.Render("Saving..."))
	}

	lines = append(lines, "", helpBar(helpKey("enter", "save"), helpKey("esc", "cancel")))
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) addGroup(name string) tea.Cmd {
	return func() tea.Msg {
		if a.app.AddGroup == nil {
			return groupAddedMsg{err: fmt.Errorf("not connected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := a.app.AddGroup.Execute(ctx, name, "")
		return groupAddedMsg{err: err}
	}
}

func (a *App) updateGroup(g interface{ Index() uint32; Name() string }) tea.Cmd {
	return func() tea.Msg {
		if a.app.UpdateGroup == nil {
			return groupUpdatedMsg{err: fmt.Errorf("not connected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		group, err := a.app.Groups().Get(ctx, g.Index())
		if err != nil || group == nil {
			return groupUpdatedMsg{err: fmt.Errorf("group not found")}
		}
		group.SetName(g.Name())
		_, err = a.app.UpdateGroup.Execute(ctx, group)
		return groupUpdatedMsg{err: err}
	}
}

func (a *App) deleteGroup(index uint32) tea.Cmd {
	return func() tea.Msg {
		if a.app.DeleteGroup == nil {
			return groupDeletedMsg{err: fmt.Errorf("not connected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := a.app.DeleteGroup.Execute(ctx, index, false)
		return groupDeletedMsg{err: err}
	}
}

func (a *App) purgeGroup(index uint32) tea.Cmd {
	return func() tea.Msg {
		if a.app.PurgeGroup == nil {
			return groupPurgedMsg{err: fmt.Errorf("not connected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := a.app.PurgeGroup.Execute(ctx, index)
		return groupPurgedMsg{err: err}
	}
}
