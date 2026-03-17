package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/solock/solock/internal/application"
	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/repository/adapter"
	"github.com/solock/solock/internal/usecase"
)

type screen int

const (
	screenUnlock screen = iota
	screenDashboard
	screenVault
	screenEntryView
	screenEntryTypeSelect
	screenEntryAdd
	screenEntryEdit
	screenSync
	screenConfig
	screenConfirm
)

type logEntry struct {
	Time time.Time
	Text string
}

type formField struct {
	Key       string
	Label     string
	Input     textinput.Model
	Generable bool
}

type App struct {
	app     *application.App
	program *tea.Program
	screen  screen
	prev    screen
	width   int
	height  int

	passwordInput textinput.Model
	spinner       spinner.Model
	progress      progress.Model
	searchInput   textinput.Model

	network         string
	rpcURL          string
	balance         uint64
	programDeployed bool
	vaultExists     bool
	lastSyncAt      time.Time
	lastError       string
	loading         bool

	entries       []*domain.Entry
	entryCursor   int
	selectedEntry *domain.Entry
	showSecrets   bool
	searching     bool
	sortMode      int

	typeCursor      int
	entryFormType   domain.EntryType
	entryFormFields []formField
	entryFormCursor int
	entryFormName   textinput.Model

	passGenConfig *domain.PasswordGenConfig
	passGenOpen   bool
	passGenCursor int

	syncProgress usecase.SyncProgress
	syncing      bool

	confirmMsg    string
	confirmAction func() tea.Cmd

	logs []logEntry

	statusMsg string
	statusErr bool
}

type balanceMsg struct{ balance uint64; err error }
type deployStatusMsg struct{ deployed bool; err error }
type vaultStatusMsg struct{ exists bool; err error }
type deployResultMsg struct{ err error }
type syncDoneMsg struct{ err error }
type entriesLoadedMsg struct{ entries []*domain.Entry; err error }
type entryPushedMsg struct{ err error; onChain bool }
type entryDeletedMsg struct{ err error; onChain bool }
type setupDoneMsg struct{ err error }
type tickMsg time.Time
type totpTickMsg time.Time
type initVaultMsg struct{ err error }

func NewTUI(app *application.App) *App {
	ti := textinput.New()
	ti.Placeholder = "Enter master password"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	ti.Width = 50
	ti.Prompt = ""
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	prog := progress.New(progress.WithDefaultGradient(), progress.WithWidth(40))

	si := textinput.New()
	si.Placeholder = "Search..."
	si.Width = 30
	si.Prompt = ""

	return &App{
		app:           app,
		screen:        screenUnlock,
		passwordInput: ti,
		spinner:       sp,
		progress:      prog,
		searchInput:   si,
		network:       "devnet",
		rpcURL:        "https://api.devnet.solana.com",
	}
}

func (a *App) SetProgram(p *tea.Program) {
	a.program = p
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, a.spinner.Tick)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			a.app.Shutdown()
			return a, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd

	case progress.FrameMsg:
		model, cmd := a.progress.Update(msg)
		a.progress = model.(progress.Model)
		return a, cmd

	case balanceMsg:
		if msg.err == nil {
			a.balance = msg.balance
		}
		return a, nil

	case deployStatusMsg:
		if msg.err == nil {
			a.programDeployed = msg.deployed
		}
		return a, nil

	case vaultStatusMsg:
		wasReady := a.vaultExists
		if msg.err == nil {
			a.vaultExists = msg.exists
		}
		if !wasReady && a.vaultExists && a.app.Sync != nil && a.lastSyncAt.IsZero() {
			a.addLog("Auto-syncing...")
			a.syncing = true
			a.screen = screenSync
			return a, a.startSync()
		}
		return a, nil

	case deployResultMsg:
		a.loading = false
		if msg.err != nil {
			a.addLog("Deploy failed: " + msg.err.Error())
		} else {
			a.programDeployed = true
			a.addLog("Program deployed")
		}
		return a, tea.Batch(a.checkBalance(), a.checkDeployStatus())

	case initVaultMsg:
		a.loading = false
		if msg.err != nil {
			a.addLog("Vault init failed: " + msg.err.Error())
		} else {
			a.vaultExists = true
			a.addLog("Vault initialized")
		}
		return a, nil

	case syncProgressMsg:
		a.syncProgress = msg.progress
		if msg.progress.Total > 0 {
			return a, a.progress.SetPercent(float64(msg.progress.Current) / float64(msg.progress.Total))
		}
		return a, nil

	case syncDoneMsg:
		a.syncing = false
		if msg.err != nil {
			a.lastError = "sync"
			a.addLog("Sync failed: " + msg.err.Error())
		} else {
			a.lastError = ""
			a.lastSyncAt = time.Now()
			a.addLog("Sync completed")
		}
		if a.screen == screenSync {
			a.screen = screenDashboard
		}
		return a, a.loadEntries()

	case entriesLoadedMsg:
		a.loading = false
		if msg.err == nil {
			a.entries = msg.entries
			if a.entryCursor >= len(a.entries) && len(a.entries) > 0 {
				a.entryCursor = len(a.entries) - 1
			}
		}
		return a, nil

	case entryPushedMsg:
		a.loading = false
		if msg.err != nil {
			a.lastError = "save"
			a.addLog("Save failed: " + msg.err.Error())
		} else {
			a.lastError = ""
			if msg.onChain {
				a.addLog("Saved to Solana")
			} else {
				a.addLog("Saved locally (not on-chain)")
			}
		}
		a.screen = screenVault
		return a, tea.Batch(a.loadEntries(), a.checkBalance())

	case entryDeletedMsg:
		a.loading = false
		if msg.err != nil {
			a.lastError = "delete"
			a.addLog("Delete failed: " + msg.err.Error())
		} else {
			a.lastError = ""
			if msg.onChain {
				a.addLog("Deleted from Solana")
			} else {
				a.addLog("Deleted locally")
			}
		}
		a.screen = screenVault
		return a, tea.Batch(a.loadEntries(), a.checkBalance())

	case tickMsg:
		if a.screen == screenDashboard {
			return a, tea.Batch(a.checkBalance(), a.checkDeployStatus(), a.checkVaultStatus(), a.scheduleTick())
		}
		return a, a.scheduleTick()

	case totpTickMsg:
		if a.screen == screenEntryView {
			return a, a.scheduleTotpTick()
		}
		return a, nil

	case setupDoneMsg:
		a.loading = false
		if msg.err != nil {
			a.statusMsg = msg.err.Error()
			a.statusErr = true
			return a, nil
		}
		return a, a.afterUnlock()
	}

	switch a.screen {
	case screenUnlock:
		return a.updateUnlock(msg)
	case screenDashboard:
		return a.updateDashboard(msg)
	case screenVault:
		return a.updateVault(msg)
	case screenEntryView:
		return a.updateEntryView(msg)
	case screenEntryTypeSelect:
		return a.updateTypeSelect(msg)
	case screenEntryAdd, screenEntryEdit:
		return a.updateEntryForm(msg)
	case screenSync:
		return a.updateSync(msg)
	case screenConfig:
		return a.updateConfig(msg)
	case screenConfirm:
		return a.updateConfirm(msg)
	}
	return a, nil
}

func (a *App) View() string {
	switch a.screen {
	case screenUnlock:
		return a.viewUnlock()
	case screenDashboard:
		return a.viewDashboard()
	case screenVault:
		return a.viewVault()
	case screenEntryView:
		return a.viewEntryDetail()
	case screenEntryTypeSelect:
		return a.viewTypeSelect()
	case screenEntryAdd, screenEntryEdit:
		return a.viewEntryForm()
	case screenSync:
		return a.viewSync()
	case screenConfig:
		return a.viewConfig()
	case screenConfirm:
		return a.viewConfirm()
	}
	return ""
}

func (a *App) updateUnlock(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			password := a.passwordInput.Value()
			if password == "" {
				return a, nil
			}
			a.loading = true
			a.statusMsg = ""
			a.statusErr = false
			rpcURL := a.rpcURL
			appRef := a.app
			return a, func() tea.Msg {
				err := appRef.OnUnlock(context.Background(), password, rpcURL)
				return setupDoneMsg{err: err}
			}
		case "esc":
			a.app.Shutdown()
			return a, tea.Quit
		}
	}
	var cmd tea.Cmd
	a.passwordInput, cmd = a.passwordInput.Update(msg)
	return a, cmd
}

func (a *App) afterUnlock() tea.Cmd {
	a.passwordInput.SetValue("")
	a.passGenConfig = domain.DefaultPasswordGenConfig()

	ctx := context.Background()
	if cfg := a.app.Config(); cfg != nil {
		a.loadPassGenConfig(ctx, cfg)
		if savedNet, _ := cfg.Get(ctx, "network"); savedNet != "" {
			a.network = savedNet
		}
		if savedRPC, _ := cfg.Get(ctx, "rpc_url"); savedRPC != "" {
			a.rpcURL = savedRPC
		}
	}
	if ss := a.app.SyncState(); ss != nil {
		if state, err := ss.Get(ctx); err == nil && !state.LastSyncAt.IsZero() {
			a.lastSyncAt = state.LastSyncAt
		}
	}

	a.addLog("Connected to " + a.network)
	a.screen = screenDashboard

	return tea.Batch(
		a.checkBalance(),
		a.checkDeployStatus(),
		a.checkVaultStatus(),
		a.loadEntries(),
		a.scheduleTick(),
	)
}

func (a *App) updateDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "q":
			a.app.Shutdown()
			return a, tea.Quit
		case "v":
			a.screen = screenVault
			a.entryCursor = 0
			return a, nil
		case "s":
			if a.app.Vault() == nil || !a.vaultExists {
				a.addLog("Vault not ready")
				return a, nil
			}
			a.screen = screenSync
			a.syncing = true
			return a, a.startSync()
		case "d":
			if a.balance < 3_000_000_000 {
				a.addLog("Need ~3 SOL to deploy")
				return a, nil
			}
			if a.programDeployed {
				a.confirmMsg = "Redeploy program?"
				a.prev = screenDashboard
				a.confirmAction = func() tea.Cmd {
					a.addLog("Redeploying...")
					return a.deployProgram()
				}
				a.screen = screenConfirm
				return a, nil
			}
			a.loading = true
			a.addLog("Deploying...")
			return a, a.deployProgram()
		case "i":
			if !a.programDeployed {
				a.addLog("Deploy program first")
				return a, nil
			}
			if a.vaultExists {
				a.confirmMsg = "Reset vault on-chain?"
				a.prev = screenDashboard
				a.confirmAction = func() tea.Cmd {
					a.addLog("Resetting vault...")
					return a.resetVault()
				}
				a.screen = screenConfirm
				return a, nil
			}
			a.loading = true
			a.addLog("Initializing vault...")
			return a, a.initVault()
		case "n":
			a.toggleNetwork()
			a.addLog("Switched to " + a.network)
			return a, tea.Batch(a.checkBalance(), a.checkDeployStatus(), a.checkVaultStatus())
		case "r":
			if a.lastError == "sync" && a.vaultExists {
				a.lastError = ""
				a.screen = screenSync
				a.syncing = true
				return a, a.startSync()
			}
			return a, tea.Batch(a.checkBalance(), a.checkDeployStatus(), a.checkVaultStatus(), a.loadEntries())
		case "p":
			keys := a.app.Keys()
			if keys != nil {
				clipboard.WriteAll(keys.DeployerAddress)
				a.addLog("Deployer address copied")
			}
			return a, nil
		case "a":
			if !a.vaultExists {
				a.addLog("Initialize vault first")
				return a, nil
			}
			a.typeCursor = 0
			a.screen = screenEntryTypeSelect
			return a, nil
		case "x":
			a.confirmMsg = "Clear local database?"
			a.prev = screenDashboard
			a.confirmAction = func() tea.Cmd {
				entries := a.app.Entries()
				return func() tea.Msg {
					if entries != nil {
						entries.ClearAll(context.Background())
					}
					return entriesLoadedMsg{entries: nil}
				}
			}
			a.screen = screenConfirm
			return a, nil
		case "c":
			a.screen = screenConfig
			return a, nil
		}
	}
	return a, nil
}

// visibleEntries moved to vault_helpers.go

func (a *App) updateVault(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.searching {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "esc":
				a.searching = false
				a.searchInput.SetValue("")
				a.searchInput.Blur()
				a.entryCursor = 0
				return a, nil
			case "enter":
				a.searching = false
				a.searchInput.Blur()
				a.entryCursor = 0
				return a, nil
			}
		}
		var cmd tea.Cmd
		a.searchInput, cmd = a.searchInput.Update(msg)
		a.entryCursor = 0
		return a, cmd
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		visible := a.visibleEntries()
		switch keyMsg.String() {
		case "esc", "h":
			a.searching = false
			a.searchInput.SetValue("")
			a.screen = screenDashboard
			return a, nil
		case "q":
			a.app.Shutdown()
			return a, tea.Quit
		case "/":
			a.searching = true
			a.searchInput.Focus()
			return a, textinput.Blink
		case "up", "k":
			if a.entryCursor > 0 {
				a.entryCursor--
			}
		case "down", "j":
			if a.entryCursor < len(visible)-1 {
				a.entryCursor++
			}
		case "enter", "l":
			if len(visible) > 0 && a.entryCursor < len(visible) {
				a.selectedEntry = visible[a.entryCursor]
				a.screen = screenEntryView
				return a, a.scheduleTotpTick()
			}
		case "s":
			a.sortMode = (a.sortMode + 1) % sortModeCount
			a.entryCursor = 0
			a.addLog("Sort: " + sortLabels[a.sortMode])
			return a, nil
		case "d":
			if len(visible) > 0 && a.entryCursor < len(visible) {
				return a, a.duplicateEntry(visible[a.entryCursor])
			}
		case "a":
			if !a.vaultExists {
				a.addLog("Initialize vault first")
				return a, nil
			}
			a.typeCursor = 0
			a.screen = screenEntryTypeSelect
			return a, nil
		case "x", "delete":
			if len(visible) > 0 && a.entryCursor < len(visible) {
				entry := visible[a.entryCursor]
				a.confirmMsg = fmt.Sprintf("Delete %q?", entry.Name())
				a.prev = screenVault
				capturedEntry := entry
				a.confirmAction = func() tea.Cmd {
					return a.deleteEntry(capturedEntry)
				}
				a.screen = screenConfirm
			}
		}
	}
	return a, nil
}

func (a *App) updateEntryView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.selectedEntry == nil {
		a.screen = screenVault
		return a, nil
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc", "h":
			a.screen = screenVault
			a.showSecrets = false
			a.selectedEntry = nil
			return a, nil
		case "e":
			a.initEntryForm(a.selectedEntry.Type(), a.selectedEntry)
			a.screen = screenEntryEdit
			return a, nil
		case "s":
			a.showSecrets = !a.showSecrets
			return a, nil
		case "c":
			a.copyEntrySecret()
			return a, nil
		case "t":
			a.copyTOTPCode()
			return a, nil
		case "x":
			entry := a.selectedEntry
			a.showSecrets = false
			a.confirmMsg = fmt.Sprintf("Delete %q?", entry.Name())
			a.prev = screenVault
			a.confirmAction = func() tea.Cmd {
				return a.deleteEntry(entry)
			}
			a.screen = screenConfirm
			return a, nil
		case "q":
			a.app.Shutdown()
			return a, tea.Quit
		}
	}
	return a, nil
}

func (a *App) updateTypeSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc", "h":
			a.screen = screenVault
			return a, nil
		case "up", "k":
			if a.typeCursor > 0 {
				a.typeCursor--
			}
		case "down", "j":
			if a.typeCursor < len(domain.CreatableEntryTypes)-1 {
				a.typeCursor++
			}
		case "enter", "l":
			a.initEntryForm(domain.CreatableEntryTypes[a.typeCursor], nil)
			a.screen = screenEntryAdd
			return a, nil
		}
	}
	return a, nil
}

func (a *App) updateEntryForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.passGenOpen {
		return a.updatePassGenPanel(msg)
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			a.passGenOpen = false
			if a.screen == screenEntryAdd {
				a.screen = screenVault
			} else {
				a.screen = screenEntryView
			}
			return a, nil
		case "tab", "down":
			a.nextFormField()
			return a, nil
		case "shift+tab", "up":
			a.prevFormField()
			return a, nil
		case "right":
			if a.isOnGenerableField() && a.currentFieldValue() == "" {
				a.passGenOpen = true
				a.passGenCursor = 0
				return a, nil
			}
		case "enter":
			total := 1 + len(a.entryFormFields)
			if a.entryFormCursor == total-1 {
				return a, a.saveEntryForm()
			}
			if a.isOnGenerableField() {
				idx := a.entryFormCursor - 1
				a.entryFormFields[idx].Input.SetValue(generatePasswordWithConfig(a.passGenConfig))
				a.addLog("Password generated")
				return a, nil
			}
			a.nextFormField()
			return a, nil
		}
	}

	if a.entryFormCursor == 0 {
		var cmd tea.Cmd
		a.entryFormName, cmd = a.entryFormName.Update(msg)
		return a, cmd
	}
	idx := a.entryFormCursor - 1
	if idx < len(a.entryFormFields) {
		var cmd tea.Cmd
		a.entryFormFields[idx].Input, cmd = a.entryFormFields[idx].Input.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a *App) currentFieldValue() string {
	if a.entryFormCursor == 0 {
		return a.entryFormName.Value()
	}
	idx := a.entryFormCursor - 1
	if idx < len(a.entryFormFields) {
		return a.entryFormFields[idx].Input.Value()
	}
	return ""
}

func (a *App) isOnGenerableField() bool {
	if a.entryFormCursor == 0 {
		return false
	}
	idx := a.entryFormCursor - 1
	return idx < len(a.entryFormFields) && a.entryFormFields[idx].Generable
}

var passGenOptions = []string{"Length", "Uppercase", "Digits", "Special"}

func (a *App) updatePassGenPanel(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc", "h":
			a.passGenOpen = false
			a.savePassGenConfig()
			return a, nil
		case "tab":
			a.passGenOpen = false
			a.savePassGenConfig()
			a.nextFormField()
			return a, nil
		case "up", "k":
			if a.passGenCursor > 0 {
				a.passGenCursor--
			}
		case "down", "j":
			if a.passGenCursor < len(passGenOptions)-1 {
				a.passGenCursor++
			}
		case "enter", " ", "l":
			a.togglePassGenOption()
			return a, nil
		case "left":
			if a.passGenCursor == 0 && a.passGenConfig.Length > 8 {
				a.passGenConfig.Length--
			}
		case "right":
			if a.passGenCursor == 0 && a.passGenConfig.Length < 64 {
				a.passGenConfig.Length++
			}
		case "g":
			idx := a.entryFormCursor - 1
			if idx >= 0 && idx < len(a.entryFormFields) {
				a.entryFormFields[idx].Input.SetValue(generatePasswordWithConfig(a.passGenConfig))
				a.addLog("Password generated")
			}
			a.passGenOpen = false
			a.savePassGenConfig()
			return a, nil
		}
	}
	return a, nil
}

func (a *App) togglePassGenOption() {
	switch a.passGenCursor {
	case 1:
		a.passGenConfig.Uppercase = !a.passGenConfig.Uppercase
	case 2:
		a.passGenConfig.Digits = !a.passGenConfig.Digits
	case 3:
		a.passGenConfig.Special = !a.passGenConfig.Special
	}
}

func (a *App) loadPassGenConfig(ctx context.Context, cfg domain.ConfigRepository) {
	if raw, _ := cfg.Get(ctx, "passgen_config"); raw != "" {
		var c domain.PasswordGenConfig
		if err := json.Unmarshal([]byte(raw), &c); err == nil {
			a.passGenConfig = &c
		}
	}
}

func (a *App) savePassGenConfig() {
	if a.passGenConfig == nil || a.app.Config() == nil {
		return
	}
	data, _ := json.Marshal(a.passGenConfig)
	a.app.Config().Set(context.Background(), "passgen_config", string(data))
}

func (a *App) updateSync(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "esc" && !a.syncing {
			a.screen = screenDashboard
			return a, nil
		}
	}
	return a, nil
}

func (a *App) toggleNetwork() {
	if a.network == "devnet" {
		a.network = "mainnet-beta"
		a.rpcURL = "https://api.mainnet-beta.solana.com"
	} else {
		a.network = "devnet"
		a.rpcURL = "https://api.devnet.solana.com"
	}
}

func (a *App) updateConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			a.saveConfig()
			a.screen = screenDashboard
			return a, tea.Batch(a.checkBalance(), a.checkDeployStatus(), a.checkVaultStatus())
		case "n":
			a.toggleNetwork()
			return a, nil
		}
	}
	return a, nil
}

func (a *App) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "y", "enter":
			a.loading = true
			cmd := a.confirmAction()
			a.confirmAction = nil
			a.screen = a.prev
			return a, cmd
		case "n", "esc":
			a.confirmAction = nil
			a.screen = a.prev
			return a, nil
		}
	}
	return a, nil
}

func (a *App) addLog(text string) {
	if len(text) > 200 {
		text = text[:197] + "..."
	}
	a.logs = append(a.logs, logEntry{Time: time.Now(), Text: text})
	if len(a.logs) > 15 {
		a.logs = a.logs[len(a.logs)-15:]
	}
}

func (a *App) checkBalance() tea.Cmd {
	vault := a.app.Vault()
	return func() tea.Msg {
		if vault == nil {
			return balanceMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		bal, err := vault.GetBalance(ctx)
		return balanceMsg{balance: bal, err: err}
	}
}

func (a *App) checkDeployStatus() tea.Cmd {
	vault := a.app.Vault()
	return func() tea.Msg {
		if vault == nil {
			return deployStatusMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		deployed, err := vault.IsProgramDeployed(ctx)
		return deployStatusMsg{deployed: deployed, err: err}
	}
}

func (a *App) checkVaultStatus() tea.Cmd {
	vault := a.app.Vault()
	return func() tea.Msg {
		if vault == nil {
			return vaultStatusMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		exists, err := vault.Exists(ctx)
		return vaultStatusMsg{exists: exists, err: err}
	}
}

func (a *App) loadEntries() tea.Cmd {
	entries := a.app.Entries()
	return func() tea.Msg {
		if entries == nil {
			return entriesLoadedMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		list, err := entries.List(ctx)
		return entriesLoadedMsg{entries: list, err: err}
	}
}

type syncProgressMsg struct{ progress usecase.SyncProgress }

func (a *App) startSync() tea.Cmd {
	syncUC := a.app.Sync
	prog := a.program
	return func() tea.Msg {
		if syncUC == nil {
			return syncDoneMsg{err: fmt.Errorf("not connected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		err := syncUC.Execute(ctx, func(p usecase.SyncProgress) {
			if prog != nil {
				prog.Send(syncProgressMsg{progress: p})
			}
		})
		return syncDoneMsg{err: err}
	}
}

func (a *App) deployProgram() tea.Cmd {
	vault := a.app.Vault()
	keys := a.app.Keys()
	return func() tea.Msg {
		if vault == nil || keys == nil {
			return deployResultMsg{err: fmt.Errorf("not connected")}
		}
		binary, err := adapter.PatchedProgramBinary(keys.ProgramID)
		if err != nil {
			return deployResultMsg{err: fmt.Errorf("patch program: %w", err)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		return deployResultMsg{err: vault.DeployProgram(ctx, binary)}
	}
}

func (a *App) initVault() tea.Cmd {
	vault := a.app.Vault()
	return func() tea.Msg {
		if vault == nil {
			return initVaultMsg{err: fmt.Errorf("not connected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return initVaultMsg{err: vault.Initialize(ctx)}
	}
}

func (a *App) resetVault() tea.Cmd {
	vault := a.app.Vault()
	return func() tea.Msg {
		if vault == nil {
			return initVaultMsg{err: fmt.Errorf("not connected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return initVaultMsg{err: vault.Reset(ctx)}
	}
}

func (a *App) scheduleTick() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (a *App) scheduleTotpTick() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg { return totpTickMsg(t) })
}

func (a *App) initEntryForm(entryType domain.EntryType, existing *domain.Entry) {
	a.entryFormType = entryType
	a.entryFormCursor = 0

	nameInput := textinput.New()
	nameInput.Placeholder = "Entry name"
	nameInput.Width = 40
	nameInput.Prompt = ""
	nameInput.Focus()
	if existing != nil {
		nameInput.SetValue(existing.Name())
	}
	a.entryFormName = nameInput

	schema := domain.SchemaFor(entryType)
	if schema == nil {
		return
	}
	a.entryFormFields = make([]formField, len(schema.Fields))
	for i, def := range schema.Fields {
		input := textinput.New()
		input.Placeholder = def.Label
		input.Width = 40
		input.Prompt = ""
		if def.Sensitive {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '*'
		}
		if existing != nil {
			input.SetValue(existing.Field(def.Key))
		}
		a.entryFormFields[i] = formField{
			Key:       def.Key,
			Label:     def.Label,
			Input:     input,
			Generable: def.Generable,
		}
	}
}

func (a *App) nextFormField() {
	total := 1 + len(a.entryFormFields)
	a.entryFormCursor = (a.entryFormCursor + 1) % total
	a.focusFormField()
}

func (a *App) prevFormField() {
	total := 1 + len(a.entryFormFields)
	a.entryFormCursor = (a.entryFormCursor - 1 + total) % total
	a.focusFormField()
}

func (a *App) focusFormField() {
	a.entryFormName.Blur()
	for i := range a.entryFormFields {
		a.entryFormFields[i].Input.Blur()
	}
	if a.entryFormCursor == 0 {
		a.entryFormName.Focus()
	} else {
		a.entryFormFields[a.entryFormCursor-1].Input.Focus()
	}
}

func (a *App) saveEntryForm() tea.Cmd {
	name := a.entryFormName.Value()
	if name == "" {
		a.addLog("Name is required")
		return nil
	}

	fields := make(map[string]string)
	for _, f := range a.entryFormFields {
		if f.Input.Value() != "" {
			fields[f.Key] = f.Input.Value()
		}
	}

	isNew := a.screen == screenEntryAdd
	a.loading = true

	if isNew {
		addUC := a.app.AddEntry
		return func() tea.Msg {
			id := fmt.Sprintf("%d", time.Now().UnixNano())
			entry, err := domain.NewEntry(id, a.entryFormType, name, fields)
			if err != nil {
				return entryPushedMsg{err: err}
			}
			if addUC == nil {
				return entryPushedMsg{err: fmt.Errorf("not ready")}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			result, err := addUC.Execute(ctx, entry)
			if err != nil {
				return entryPushedMsg{err: err}
			}
			return entryPushedMsg{onChain: result.OnChain}
		}
	}

	entry := a.selectedEntry
	entry.SetName(name)
	entry.SetFields(fields)
	updateUC := a.app.UpdateEntry
	return func() tea.Msg {
		if updateUC == nil {
			return entryPushedMsg{err: fmt.Errorf("not ready")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		result, err := updateUC.Execute(ctx, entry)
		if err != nil {
			return entryPushedMsg{err: err}
		}
		return entryPushedMsg{onChain: result.OnChain}
	}
}

func (a *App) duplicateEntry(entry *domain.Entry) tea.Cmd {
	a.initEntryForm(entry.Type(), entry)
	a.entryFormName.SetValue(entry.Name() + " (copy)")
	a.screen = screenEntryAdd
	return nil
}

func (a *App) deleteEntry(entry *domain.Entry) tea.Cmd {
	deleteUC := a.app.DeleteEntry
	return func() tea.Msg {
		if deleteUC == nil {
			return entryDeletedMsg{err: fmt.Errorf("not ready")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := deleteUC.Execute(ctx, entry)
		if err != nil {
			return entryDeletedMsg{err: err}
		}
		return entryDeletedMsg{onChain: result.OnChain}
	}
}

func (a *App) saveConfig() {
	if a.app.Config() != nil {
		ctx := context.Background()
		a.app.Config().Set(ctx, "network", a.network)
		a.app.Config().Set(ctx, "rpc_url", a.rpcURL)
	}
	a.addLog("Config saved")
}

func (a *App) copyEntrySecret() {
	if a.selectedEntry == nil {
		return
	}
	value := a.selectedEntry.CopyableSecret()
	if value == "" {
		a.addLog("Nothing to copy")
		return
	}
	if err := clipboard.WriteAll(value); err != nil {
		a.addLog("Copy failed: " + err.Error())
		return
	}
	a.addLog("Copied to clipboard")
}

func (a *App) copyTOTPCode() {
	if a.selectedEntry == nil {
		return
	}
	secret := a.selectedEntry.TOTPSecret()
	if secret == "" {
		a.addLog("No TOTP secret")
		return
	}
	code, _, err := generateTOTP(secret, 6, 30)
	if err != nil {
		a.addLog("TOTP failed: " + err.Error())
		return
	}
	if err := clipboard.WriteAll(code); err != nil {
		a.addLog("Copy failed: " + err.Error())
		return
	}
	a.addLog("TOTP code copied")
}
