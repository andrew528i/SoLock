package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type transferDoneMsg struct{ err error }

func (a *App) updateTransfer(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case transferDoneMsg:
		a.loading = false
		if msg.err != nil {
			a.addLog("Transfer failed: " + msg.err.Error())
		} else {
			a.addLog("Transfer complete")
			a.screen = screenConfig
		}
		return a, a.checkBalance()
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			a.screen = screenConfig
			return a, nil
		case "enter":
			addr := strings.TrimSpace(a.transferInput.Value())
			if addr == "" {
				a.addLog("Address required")
				return a, nil
			}
			if len(addr) < 32 || len(addr) > 44 {
				a.addLog("Invalid SOL address")
				return a, nil
			}
			sol := float64(a.balance) / 1_000_000_000
			a.confirmMsg = fmt.Sprintf("Transfer %.4f SOL to %s...%s?", sol, addr[:6], addr[len(addr)-4:])
			a.prev = screenTransfer
			a.confirmAction = func() tea.Cmd {
				a.addLog("Transferring...")
				return a.doTransfer(addr)
			}
			a.screen = screenConfirm
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.transferInput, cmd = a.transferInput.Update(msg)
	return a, cmd
}

func (a *App) viewTransfer() string {
	var lines []string

	sol := float64(a.balance) / 1_000_000_000

	lines = append(lines, "", "  "+titleStyle.Render("Withdraw SOL"), "")
	lines = append(lines, "  "+dimStyle.Render(fmt.Sprintf("Balance: %.4f SOL on %s", sol, a.network)), "")

	w := formWidth(a.width)
	a.transferInput.Width = w

	lines = append(lines, "  "+labelStyle.Render("Recipient Address"))
	lines = append(lines, inputBox(a.transferInput.View(), true, w, "  "))

	lines = append(lines, "")
	if a.loading {
		lines = append(lines, "  "+a.spinner.View()+" "+dimStyle.Render("Sending..."))
	}

	lines = append(lines, "", helpBar(helpKey("enter", "send all"), helpKey("esc", "cancel")))
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) doTransfer(to string) tea.Cmd {
	uc := a.app.TransferAll
	return func() tea.Msg {
		if uc == nil {
			return transferDoneMsg{err: fmt.Errorf("not connected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return transferDoneMsg{err: uc.Execute(ctx, to)}
	}
}
