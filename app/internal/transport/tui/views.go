package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/solock/solock/internal/domain"
)

func (a *App) loadingLine() string {
	if a.loading {
		return "  " + a.spinner.View() + " " + dimStyle.Render("Working...") + "\n"
	}
	return ""
}

func (a *App) viewUnlock() string {
	w := formWidth(a.width)
	a.passwordInput.Width = w

	title := titleStyle.Render("  SoLock")
	subtitle := dimStyle.Render("  Decentralized Password Manager on Solana")

	var lines []string
	lines = append(lines, "", title, subtitle, "")
	lines = append(lines, "  "+labelStyle.Render("Master Password"))
	lines = append(lines, inputBox(a.passwordInput.View(), true, w, "  "))
	lines = append(lines, "")

	if a.loading {
		lines = append(lines, "  "+a.spinner.View()+" "+dimStyle.Render("Deriving keys..."))
	} else if a.statusMsg != "" {
		s := dimStyle
		if a.statusErr {
			s = dangerStyle
		}
		lines = append(lines, "  "+s.Render(a.statusMsg))
	}

	lines = append(lines, "", helpBar(helpKey("enter", "unlock"), helpKey("esc", "quit")))
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) viewDashboard() string {
	keys := a.app.Keys()
	if keys == nil {
		return "  Loading..."
	}

	var lines []string

	header := "  " + titleStyle.Render("SoLock") + "  "
	netColor := colorAccent
	if a.network == "mainnet-beta" {
		netColor = colorWarning
	}
	header += lipglossStyle(netColor, true).Render(a.network)
	lines = append(lines, "", header, "")

	sol := float64(a.balance) / 1_000_000_000
	balStyle := accentStyle
	if a.balance == 0 {
		balStyle = dangerStyle
	} else if a.balance < 1_000_000_000 {
		balStyle = warningStyle
	}

	lbl := func(l string) string { return labelStyle.Render(fmt.Sprintf("  %-10s", l)) }

	lines = append(lines, lbl("Deployer")+dimStyle.Render(keys.DeployerAddress))
	lines = append(lines, lbl("Balance")+balStyle.Render(fmt.Sprintf("%.4f SOL", sol)))
	lines = append(lines, "")

	programStatus := warningStyle.Render("not deployed")
	if a.programDeployed {
		programStatus = accentStyle.Render("deployed")
	}
	lines = append(lines, lbl("Program")+dimStyle.Render(keys.ProgramID))
	lines = append(lines, lbl("Status")+programStatus)

	if a.programDeployed {
		vaultStatus := warningStyle.Render("not initialized")
		if a.vaultExists {
			vaultStatus = accentStyle.Render("ready")
		}
		lines = append(lines, "")
		lines = append(lines, lbl("Vault")+vaultStatus)
		if a.vaultExists {
			n := len(a.entries)
			rent := 0.007 + float64(n)*0.002
			lines = append(lines, lbl("Entries")+valueStyle.Render(fmt.Sprintf("%d", n))+"  "+dimStyle.Render(fmt.Sprintf("~%.3f SOL rent", rent)))
			stats := a.entryStatsLines(lbl)
			lines = append(lines, stats...)
		}
	}

	if !a.lastSyncAt.IsZero() {
		lines = append(lines, "")
		lines = append(lines, lbl("Synced")+dimStyle.Render(a.lastSyncAt.Format("2006-01-02 15:04:05")))
	}

	lines = append(lines, "", a.loadingLine())

	if len(a.logs) > 0 {
		maxLogs := max(min(a.height/3, 10), 3)
		start := max(0, len(a.logs)-maxLogs)
		for _, l := range a.logs[start:] {
			lines = append(lines, "  "+dimStyle.Render(l.Time.Format("15:04:05")+" "+l.Text))
		}
	}
	lines = append(lines, "")

	var hints []string
	hints = append(hints, helpKey("v", "vault"))
	if a.vaultExists {
		hints = append(hints, helpKey("a", "add"), helpKey("s", "sync"))
	}
	if a.programDeployed {
		hints = append(hints, helpKey("d", "redeploy"))
	} else {
		hints = append(hints, helpKey("d", "deploy"))
	}
	if a.programDeployed {
		if a.vaultExists {
			hints = append(hints, helpKey("i", "reset vault"))
		} else {
			hints = append(hints, helpKey("i", "init vault"))
		}
	}
	if a.lastError != "" {
		hints = append(hints, helpKey("r", "retry"))
	} else {
		hints = append(hints, helpKey("r", "refresh"))
	}
	hints = append(hints, helpKey("x", "clear local"), helpKey("p", "copy addr"), helpKey("n", "net"), helpKey("c", "cfg"), helpKey("q", "quit"))
	lines = append(lines, helpBar(hints...))

	return strings.Join(lines, "\n") + "\n"
}

func (a *App) viewVault() string {
	visible := a.visibleEntries()
	var lines []string

	header := "  " + titleStyle.Render("Vault")
	if len(a.entries) > 0 {
		header += "  " + dimStyle.Render(fmt.Sprintf("%d entries", len(a.entries)))
		header += "  " + dimStyle.Render("sort:"+sortLabels[a.sortMode])
	}
	lines = append(lines, "", header)

	if a.searching {
		lines = append(lines, "  "+accentStyle.Render("/ ")+a.searchInput.View())
	}
	lines = append(lines, "")

	if len(visible) == 0 {
		if a.searching {
			lines = append(lines, "  "+dimStyle.Render("No matches"))
		} else {
			lines = append(lines, "  "+dimStyle.Render("No entries yet. Press 'a' to add your first secret."))
		}
	} else {
		maxVisible := max(a.height-9, 5)
		start := 0
		if a.entryCursor >= maxVisible {
			start = a.entryCursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(visible))

		colName := min(a.width/3, 25)
		colUser := min(a.width/4, 18)

		col := func(text string, width int, style lipgloss.Style) string {
			if len(text) > width {
				text = text[:width-2] + ".."
			}
			padded := text
			for len(padded) < width {
				padded += " "
			}
			return style.Render(padded)
		}

		for i := start; i < end; i++ {
			entry := visible[i]
			selected := i == a.entryCursor

			cursor := "  "
			nStyle := dimStyle
			if selected {
				cursor = accentStyle.Render("> ")
				nStyle = valueStyle
			}

			user := entry.Field("username")
			if user == "" {
				user = entry.Field("site")
			}

			line := cursor
			line += col(fmt.Sprintf("#%d", entry.SlotIndex()), 5, slotStyle)
			line += col(entry.Type().ShortLabel(), 6, labelStyle)
			line += col(entry.Name(), colName, nStyle)
			line += col(user, colUser, dimStyle)

			if entry.HasTOTP() {
				line += warningStyle.Render("2FA") + " "
			}

			age := entryAge(entry)
			if age != "" {
				line += age
			}

			lines = append(lines, line)
		}

		if len(visible) > maxVisible {
			scrollInfo := fmt.Sprintf("%d-%d of %d", start+1, end, len(visible))
			lines = append(lines, "  "+dimStyle.Render(scrollInfo))
		}
	}

	lines = append(lines, "", a.loadingLine())
	lines = append(lines, helpBar(
		helpKey("hjkl", "nav"), helpKey("/", "search"), helpKey("s", "sort"),
		helpKey("a", "add"), helpKey("d", "dup"), helpKey("x", "del"),
	))
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) viewEntryDetail() string {
	if a.selectedEntry == nil {
		return "  No entry selected"
	}
	entry := a.selectedEntry
	var lines []string

	lines = append(lines, "", "  "+titleStyle.Render(entry.Name())+"  "+dimStyle.Render(entry.Type().Label()), "")

	schema := entry.Schema()
	if schema != nil {
		for _, def := range schema.Fields {
			val := entry.Field(def.Key)
			if val == "" {
				continue
			}
			display := val
			if def.Sensitive && !a.showSecrets {
				display = strings.Repeat("*", min(len(val), 20))
			}
			lines = append(lines, fmt.Sprintf("  %-14s %s", labelStyle.Render(def.Label), valueStyle.Render(display)))
		}
	}

	if secret := entry.TOTPSecret(); secret != "" {
		code, remaining, err := a.generateTOTP(secret, 6, 30)
		lines = append(lines, "")
		if err == nil {
			lines = append(lines, fmt.Sprintf("  %-14s %s  %s",
				labelStyle.Render("2FA Code"),
				accentStyle.Bold(true).Render(code),
				totpCountdownBar(remaining, 30),
			))
		} else {
			lines = append(lines, fmt.Sprintf("  %-14s %s", labelStyle.Render("2FA Code"), dangerStyle.Render("invalid secret")))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "  "+dimStyle.Render(fmt.Sprintf("slot %d  created %s  updated %s",
		entry.SlotIndex(), entry.CreatedAt().Format("2006-01-02"), entry.UpdatedAt().Format("2006-01-02 15:04"),
	)))

	lines = append(lines, "")
	var hints []string
	hints = append(hints, helpKey("c", "copy pass"))
	if entry.HasTOTP() {
		hints = append(hints, helpKey("t", "copy 2fa"))
	}
	if a.showSecrets {
		hints = append(hints, helpKey("s", "hide"))
	} else {
		hints = append(hints, helpKey("s", "reveal"))
	}
	hints = append(hints, helpKey("e", "edit"), helpKey("x", "del"), helpKey("esc", "back"))
	lines = append(lines, helpBar(hints...))

	return strings.Join(lines, "\n") + "\n"
}

func (a *App) viewTypeSelect() string {
	var lines []string
	lines = append(lines, "", "  "+titleStyle.Render("New Entry"), "")

	for i, t := range domain.CreatableEntryTypes {
		cursor := "  "
		style := dimStyle
		if i == a.typeCursor {
			cursor = accentStyle.Render("> ")
			style = valueStyle
		}
		lines = append(lines, cursor+style.Render(t.Label()))
	}

	lines = append(lines, "", helpBar(helpKey("hjkl", "nav"), helpKey("esc", "back")))
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) viewEntryForm() string {
	var lines []string

	action := "Add"
	if a.screen == screenEntryEdit {
		action = "Edit"
	}
	lines = append(lines, "", "  "+titleStyle.Render(action)+"  "+dimStyle.Render(a.entryFormType.Label()), "")

	w := formWidth(a.width)
	a.entryFormName.Width = w
	lines = append(lines, "  "+labelStyle.Render("Name"))
	lines = append(lines, inputBox(a.entryFormName.View(), a.entryFormCursor == 0, w, "  "))

	for i := range a.entryFormFields {
		a.entryFormFields[i].Input.Width = w
		focused := a.entryFormCursor == i+1
		label := "  " + labelStyle.Render(a.entryFormFields[i].Label)
		if a.entryFormFields[i].Generable && focused {
			if a.entryFormFields[i].Input.Value() == "" {
				label += "  " + dimStyle.Render("enter = generate  -> = options")
			} else {
				label += "  " + dimStyle.Render("enter = regenerate")
			}
		}
		lines = append(lines, label)
		lines = append(lines, inputBox(a.entryFormFields[i].Input.View(), focused, w, "  "))

		if a.entryFormFields[i].Generable && focused && a.passGenOpen {
			lines = append(lines, a.renderPassGenPanel())
		}
	}

	lines = append(lines, "", a.loadingLine())
	hints := []string{helpKey("tab", "next"), helpKey("enter", "next/save"), helpKey("esc", "cancel")}
	lines = append(lines, helpBar(hints...))
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) viewSync() string {
	var lines []string
	lines = append(lines, "", "  "+titleStyle.Render("Sync"), "")

	if a.syncing {
		lines = append(lines, "  "+a.spinner.View()+" "+a.syncProgress.Phase)
		if a.syncProgress.Total > 0 {
			lines = append(lines, "", "  "+a.progress.View())
			lines = append(lines, "  "+dimStyle.Render(fmt.Sprintf("%d / %d entries", a.syncProgress.Current, a.syncProgress.Total)))
		}
	} else {
		lines = append(lines, "  "+accentStyle.Render("Sync complete"), "", helpBar(helpKey("esc", "back")))
	}
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) viewConfig() string {
	var lines []string
	lines = append(lines, "", "  "+titleStyle.Render("Config"), "")

	netColor := colorAccent
	if a.network == "mainnet-beta" {
		netColor = colorWarning
	}
	netLabel := lipglossStyle(netColor, true).Render(a.network)
	lines = append(lines, fmt.Sprintf("  %s  %s  %s",
		labelStyle.Render("Network"),
		netLabel,
		dimStyle.Render(a.rpcURL),
	))

	lines = append(lines, "", helpBar(
		helpKey("n", "network"),
		helpKey("esc", "back"),
	))
	return strings.Join(lines, "\n") + "\n"
}

func (a *App) viewConfirm() string {
	w := min(a.width-8, 40)
	if w < 20 {
		w = 20
	}
	box := sectionStyle.Width(w).Render(warningStyle.Render(a.confirmMsg) + "\n\n" + helpKey("y", "yes") + "  " + helpKey("n", "no"))
	return "\n\n  " + box + "\n"
}

func totpCountdownBar(remaining, period int) string {
	filled := (remaining * 10) / period
	var b strings.Builder
	for i := range 10 {
		if i < filled {
			b.WriteString(accentStyle.Render("█"))
		} else {
			b.WriteString(dimStyle.Render("░"))
		}
	}
	b.WriteString(" " + dimStyle.Render(fmt.Sprintf("%ds", remaining)))
	return b.String()
}

func (a *App) renderPassGenPanel() string {
	cfg := a.passGenConfig
	if cfg == nil {
		cfg = domain.DefaultPasswordGenConfig()
	}

	var lines []string
	lines = append(lines, "")

	options := []struct {
		label   string
		enabled bool
		isLen   bool
	}{
		{fmt.Sprintf("Length: %d", cfg.Length), true, true},
		{"Uppercase (A-Z)", cfg.Uppercase, false},
		{"Digits (0-9)", cfg.Digits, false},
		{"Special (!@#)", cfg.Special, false},
	}

	for i, opt := range options {
		cursor := "    "
		style := dimStyle
		if i == a.passGenCursor {
			cursor = "  " + accentStyle.Render("> ")
			style = valueStyle
		}

		indicator := dimStyle.Render("[ ]")
		if opt.enabled {
			indicator = accentStyle.Render("[x]")
		}
		if opt.isLen {
			indicator = dimStyle.Render("<") + valueStyle.Render(fmt.Sprintf(" %d ", cfg.Length)) + dimStyle.Render(">")
		}

		lines = append(lines, cursor+indicator+" "+style.Render(opt.label))
	}

	lines = append(lines, "")
	lines = append(lines, "    "+helpBar(helpKey("j/k", "nav"), helpKey("space", "toggle"), helpKey("g", "generate"), helpKey("esc", "close")))
	if a.passGenCursor == 0 {
		lines = append(lines, "    "+dimStyle.Render("left/right to adjust length"))
	}

	return strings.Join(lines, "\n")
}

func entryAge(entry *domain.Entry) string {
	age := time.Since(entry.CreatedAt())
	days := int(age.Hours() / 24)

	var text string
	switch {
	case days == 0:
		text = "today"
	case days == 1:
		text = "1d"
	case days < 30:
		text = fmt.Sprintf("%dd", days)
	case days < 365:
		text = fmt.Sprintf("%dmo", days/30)
	default:
		text = fmt.Sprintf("%dy", days/365)
	}

	if days > 90 {
		return warningStyle.Render(text)
	}
	return dimStyle.Render(text)
}

func (a *App) entryStatsLines(lbl func(string) string) []string {
	var passwords, notes, cards, withTOTP int
	for _, e := range a.entries {
		switch e.Type() {
		case domain.EntryTypePassword:
			passwords++
		case domain.EntryTypeNote:
			notes++
		case domain.EntryTypeCard:
			cards++
		}
		if e.HasTOTP() {
			withTOTP++
		}
	}

	var lines []string
	if passwords > 0 {
		lines = append(lines, lbl("Passwords")+dimStyle.Render(fmt.Sprintf("%d", passwords)))
	}
	if notes > 0 {
		lines = append(lines, lbl("Notes")+dimStyle.Render(fmt.Sprintf("%d", notes)))
	}
	if cards > 0 {
		lines = append(lines, lbl("Cards")+dimStyle.Render(fmt.Sprintf("%d", cards)))
	}
	if withTOTP > 0 {
		lines = append(lines, lbl("With 2FA")+dimStyle.Render(fmt.Sprintf("%d", withTOTP)))
	}
	return lines
}

func lipglossStyle(color lipgloss.Color, bold bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(color)
	if bold {
		s = s.Bold(true)
	}
	return s
}
