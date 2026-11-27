package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sshv/internal/model"
	"sshv/internal/ssh"
)

type viewMode int

const (
	modeBrowse viewMode = iota
	modeSearch
	modeForm
	modeConfirmDelete
)

type sshDoneMsg struct {
	alias string
	err   error
}

// App is the root Bubble Tea model for sshv.
type App struct {
	config        model.ConfigStore
	vault         model.VaultStore
	hosts         []model.HostEntry
	filtered      []model.HostEntry
	cursor        int
	mode          viewMode
	search        textinput.Model
	form          hostForm
	revealedAlias string
	revealedPass  string
	revealedEmpty bool
	status        string
	statusErr     bool
	width         int
	height        int
}

// NewApp initializes the TUI model with existing hosts from ConfigStore.
func NewApp(cfg model.ConfigStore, vault model.VaultStore) (*App, error) {
	hosts, err := cfg.List()
	if err != nil {
		return nil, err
	}

	ti := textinput.New()
	ti.Placeholder = "Search hosts..."
	ti.CharLimit = 64

	app := &App{
		config:   cfg,
		vault:    vault,
		hosts:    hosts,
		filtered: hosts,
		search:   ti,
	}
	return app, nil
}

func (a *App) Init() tea.Cmd {
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case sshDoneMsg:
		if msg.err != nil {
			a.setError(fmt.Sprintf("ssh %s exited: %v", msg.alias, msg.err))
		} else {
			a.setStatus(fmt.Sprintf("Closed session: %s", msg.alias))
		}
		return a, nil
	}

	switch a.mode {
	case modeForm:
		cmd, submitted, cancelled := a.form.Update(msg)
		if cancelled {
			a.mode = modeBrowse
			return a, nil
		}
		if submitted {
			a.saveForm()
			return a, nil
		}
		return a, cmd

	case modeSearch:
		return a.updateSearch(msg)

	case modeConfirmDelete:
		return a.updateConfirmDelete(msg)

	default:
		return a.updateBrowse(msg)
	}
}

func (a *App) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}

	switch keyMsg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit
	case "j", "down":
		if a.cursor < len(a.filtered)-1 {
			a.cursor++
			a.clearReveal()
		}
	case "k", "up":
		if a.cursor > 0 {
			a.cursor--
			a.clearReveal()
		}
	case "p":
		if host, ok := a.selectedHost(); ok {
			if a.revealedAlias == host.Alias {
				a.clearReveal()
				return a, nil
			}
			pass, hasPass, err := a.vault.GetPassword(host.Alias)
			if err != nil {
				a.setError(fmt.Sprintf("keychain error: %v", err))
				return a, nil
			}
			a.revealedAlias = host.Alias
			a.revealedPass = pass
			a.revealedEmpty = !hasPass
		}
	case "/":
		a.clearReveal()
		a.mode = modeSearch
		a.search.Focus()
		return a, textinput.Blink
	case "a":
		a.clearReveal()
		a.form = newHostForm(nil, "", false)
		a.mode = modeForm
		return a, textinput.Blink
	case "e":
		if host, ok := a.selectedHost(); ok {
			a.clearReveal()
			pass, hasPass, _ := a.vault.GetPassword(host.Alias)
			a.form = newHostForm(&host, pass, hasPass)
			a.mode = modeForm
			return a, textinput.Blink
		}
	case "d":
		if _, ok := a.selectedHost(); ok {
			a.mode = modeConfirmDelete
		}
	case "enter":
		if host, ok := a.selectedHost(); ok {
			pass, hasPass, err := a.vault.GetPassword(host.Alias)
			if err != nil {
				a.setError(fmt.Sprintf("keychain error: %v", err))
				return a, nil
			}
			alias := host.Alias
			return a, ssh.NewConnectCmd(alias, pass, hasPass, func(err error) tea.Msg {
				return sshDoneMsg{alias: alias, err: err}
			})
		}
	}
	return a, nil
}

func (a *App) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			a.search.SetValue("")
			a.applyFilter()
			a.search.Blur()
			a.mode = modeBrowse
			return a, nil
		case "enter":
			a.search.Blur()
			a.mode = modeBrowse
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.search, cmd = a.search.Update(msg)
	a.applyFilter()
	return a, cmd
}

func (a *App) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}
	switch strings.ToLower(keyMsg.String()) {
	case "y":
		if host, ok := a.selectedHost(); ok {
			if err := a.config.Delete(host.Alias); err != nil {
				a.setError(err.Error())
			} else {
				_ = a.vault.DeletePassword(host.Alias)
				_ = a.reloadHosts(host.Alias)
				a.setStatus(fmt.Sprintf("Deleted %s", host.Alias))
			}
		}
		a.mode = modeBrowse
	case "n", "esc", "enter":
		a.mode = modeBrowse
	}
	return a, nil
}

func (a *App) saveForm() {
	in := a.form.result()
	if err := a.config.Upsert(a.form.originalAlias, in.HostEntry); err != nil {
		a.form.errMsg = err.Error()
		return
	}

	if a.form.editing && a.form.originalAlias != in.Alias {
		// Migrate password if alias was renamed and user didn't overwrite or clear it.
		if oldPass, ok, _ := a.vault.GetPassword(a.form.originalAlias); ok && in.Password == "" && !in.ClearPass {
			_ = a.vault.SetPassword(in.Alias, oldPass)
		}
		_ = a.vault.DeletePassword(a.form.originalAlias)
	}

	if in.ClearPass {
		if err := a.vault.DeletePassword(in.Alias); err != nil {
			a.setError(fmt.Sprintf("saved host, keychain delete failed: %v", err))
		}
	} else if in.Password != "" {
		if err := a.vault.SetPassword(in.Alias, in.Password); err != nil {
			a.setError(fmt.Sprintf("saved host, keychain save failed: %v", err))
		}
	}

	_ = a.reloadHosts(in.Alias)
	a.mode = modeBrowse
	a.setStatus(fmt.Sprintf("Saved %s", in.Alias))
}

func (a *App) reloadHosts(preferAlias string) error {
	hosts, err := a.config.List()
	if err != nil {
		return err
	}
	a.hosts = hosts
	a.applyFilter()
	for i, h := range a.filtered {
		if h.Alias == preferAlias {
			a.cursor = i
			break
		}
	}
	return nil
}

func (a *App) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(a.search.Value()))
	if q == "" {
		a.filtered = a.hosts
	} else {
		var out []model.HostEntry
		for _, h := range a.hosts {
			if strings.Contains(strings.ToLower(h.Alias), q) ||
				strings.Contains(strings.ToLower(h.HostName), q) ||
				strings.Contains(strings.ToLower(h.User), q) {
				out = append(out, h)
			}
		}
		a.filtered = out
	}
	if a.cursor >= len(a.filtered) {
		a.cursor = max(0, len(a.filtered)-1)
	}
}

func (a *App) selectedHost() (model.HostEntry, bool) {
	if len(a.filtered) == 0 || a.cursor < 0 || a.cursor >= len(a.filtered) {
		return model.HostEntry{}, false
	}
	return a.filtered[a.cursor], true
}

func (a *App) clearReveal() {
	a.revealedAlias = ""
	a.revealedPass = ""
	a.revealedEmpty = false
}

func (a *App) setStatus(msg string) {
	a.status = msg
	a.statusErr = false
}

func (a *App) setError(msg string) {
	a.status = msg
	a.statusErr = true
}

func (a *App) View() string {
	w := a.width
	h := a.height
	if w < 60 {
		w = 80
	}
	if h < 18 {
		h = 24
	}

	if a.mode == modeForm {
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, a.form.View(w))
	}

	bodyHeight := h - 4
	leftWidth := w*35/100 - 2
	if leftWidth < 24 {
		leftWidth = 24
	}
	rightWidth := w - leftWidth - 6
	if rightWidth < 30 {
		rightWidth = 30
	}

	leftPane := a.renderLeftPane(leftWidth, bodyHeight)
	rightPane := a.renderRightPane(rightWidth, bodyHeight)
	main := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)

	footer := a.renderFooter()
	return lipgloss.JoinVertical(lipgloss.Left, main, footer)
}

func (a *App) renderLeftPane(width, height int) string {
	searchPrefix := "/ "
	if a.mode == modeSearch {
		searchPrefix = "> "
	}
	a.search.Width = width - 4

	var lines []string
	lines = append(lines, titleStyle.Render("sshv")+"  "+mutedStyle.Render(fmt.Sprintf("(%d hosts)", len(a.filtered))))
	lines = append(lines, searchPrefix+a.search.View())
	lines = append(lines, strings.Repeat("─", max(1, width-2)))

	listHeight := max(1, height-4)
	start := 0
	if a.cursor >= listHeight {
		start = a.cursor - listHeight + 1
	}

	if len(a.filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No hosts. Press 'a' to add."))
	} else {
		end := min(len(a.filtered), start+listHeight)
		for i := start; i < end; i++ {
			host := a.filtered[i]
			label := host.Alias
			if len(label) > width-4 {
				label = label[:width-5] + "…"
			}
			if i == a.cursor {
				lines = append(lines, selectedItemStyle.Width(width-2).Render(" "+label))
			} else {
				lines = append(lines, normalItemStyle.Render(" "+label))
			}
		}
	}

	style := paneStyle
	if a.mode == modeSearch {
		style = activePaneStyle
	}
	return style.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (a *App) renderRightPane(width, height int) string {
	host, ok := a.selectedHost()
	if !ok {
		content := titleStyle.Render("Host Details") + "\n\n" +
			mutedStyle.Render("Select a host or press 'a' to create one.\nEntries are saved to ~/.ssh/config and passwords to macOS Keychain.")
		return paneStyle.Width(width).Height(height).Render(content)
	}

	valOrDash := func(s string) string {
		if strings.TrimSpace(s) == "" {
			return mutedStyle.Render("-")
		}
		return valueStyle.Render(s)
	}

	passDisplay := mutedStyle.Render("press p to reveal")
	if a.revealedAlias == host.Alias {
		if a.revealedEmpty {
			passDisplay = mutedStyle.Render("- (none stored)")
		} else {
			passDisplay = valueStyle.Render(a.revealedPass)
		}
	}

	rows := []string{
		titleStyle.Render(host.Alias),
		"",
		labelStyle.Render("HostName:") + valOrDash(host.HostName),
		labelStyle.Render("User:") + valOrDash(host.User),
		labelStyle.Render("Port:") + valOrDash(host.Port),
		labelStyle.Render("IdentityFile:") + valOrDash(host.IdentityFile),
		labelStyle.Render("Password:") + passDisplay,
		labelStyle.Render("Command:") + valueStyle.Render("ssh "+host.Alias),
		"",
		mutedStyle.Render("Press Enter to connect (auto-fills Keychain password if saved)."),
	}
	return paneStyle.Width(width).Height(height).Render(strings.Join(rows, "\n"))
}

func (a *App) renderFooter() string {
	if a.mode == modeConfirmDelete {
		if host, ok := a.selectedHost(); ok {
			return errorStyle.Render(fmt.Sprintf("Delete %q from ~/.ssh/config and Keychain? (y/N)", host.Alias))
		}
	}

	help := mutedStyle.Render("enter connect · p reveal · a add · e edit · d delete · / search · q quit")
	if a.status == "" {
		return help
	}
	if a.statusErr {
		return errorStyle.Render(a.status) + "   " + help
	}
	return successStyle.Render(a.status) + "   " + help
}
