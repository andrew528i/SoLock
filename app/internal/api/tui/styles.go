package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorPrimary   = lipgloss.Color("#10B981")
	colorSecondary = lipgloss.Color("#6EE7B7")
	colorAccent    = lipgloss.Color("#34D399")
	colorWarning   = lipgloss.Color("#FBBF24")
	colorDanger    = lipgloss.Color("#F87171")
	colorMuted     = lipgloss.Color("#6B7280")
	colorText      = lipgloss.Color("#E5E7EB")
	colorDim       = lipgloss.Color("#9CA3AF")
	colorBorder    = lipgloss.Color("#374151")
	colorHighlight = lipgloss.Color("#6EE7B7")
	colorSlot      = lipgloss.Color("#475569")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	labelStyle = lipgloss.NewStyle().Foreground(colorMuted)
	valueStyle = lipgloss.NewStyle().Foreground(colorText)
	accentStyle = lipgloss.NewStyle().Foreground(colorAccent)
	warningStyle = lipgloss.NewStyle().Foreground(colorWarning)
	dangerStyle = lipgloss.NewStyle().Foreground(colorDanger)
	dimStyle = lipgloss.NewStyle().Foreground(colorDim)
	keyStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
	slotStyle = lipgloss.NewStyle().Foreground(colorSlot)

	sectionStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2)
)

func inputBox(view string, focused bool, w int, indent string) string {
	c := "55;65;81"
	if focused {
		c = "16;185;129"
	}
	esc := "\x1b[38;2;" + c + "m"
	rst := "\x1b[0m"

	col := len(indent) + w + 4
	colEsc := fmt.Sprintf("\x1b[%dG", col)

	h := strings.Repeat("─", w+2)
	top := indent + esc + "╭" + h + "╮" + rst
	mid := indent + esc + "│" + rst + " " + view + colEsc + esc + "│" + rst
	bot := indent + esc + "╰" + h + "╯" + rst
	return top + "\n" + mid + "\n" + bot
}

func formWidth(termWidth int) int {
	w := termWidth - 10
	if w > 50 {
		w = 50
	}
	if w < 25 {
		w = 25
	}
	return w
}

func helpKey(key, desc string) string {
	return keyStyle.Render(key) + " " + dimStyle.Render(desc)
}

func helpBar(items ...string) string {
	return "  " + strings.Join(items, "  ")
}
