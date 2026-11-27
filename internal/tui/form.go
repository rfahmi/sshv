package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sshv/internal/model"
)

const (
	fieldAlias = iota
	fieldHostName
	fieldUser
	fieldPort
	fieldIdentityFile
	fieldPassword
	fieldCount
)

type hostForm struct {
	inputs        []textinput.Model
	focus         int
	editing       bool
	originalAlias string
	hasStoredPass bool
	clearPass     bool
	errMsg        string
}

func newHostForm(entry *model.HostEntry, storedPass string, hasStoredPass bool) hostForm {
	inputs := make([]textinput.Model, fieldCount)

	makeInput := func(placeholder string, echo textinput.EchoMode) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.EchoMode = echo
		ti.CharLimit = 256
		ti.Width = 36
		return ti
	}

	inputs[fieldAlias] = makeInput("e.g. harnic", textinput.EchoNormal)
	inputs[fieldHostName] = makeInput("e.g. 103.10.10.1 or vps.example.com", textinput.EchoNormal)
	inputs[fieldUser] = makeInput("e.g. root", textinput.EchoNormal)
	inputs[fieldPort] = makeInput("22", textinput.EchoNormal)
	inputs[fieldIdentityFile] = makeInput("~/.ssh/id_ed25519 (optional)", textinput.EchoNormal)
	passPlaceholder := "optional (stored in macOS Keychain)"
	if hasStoredPass {
		passPlaceholder = "ctrl+r reveal · ctrl+x clear"
	}
	inputs[fieldPassword] = makeInput(passPlaceholder, textinput.EchoPassword)

	f := hostForm{
		inputs:        inputs,
		hasStoredPass: hasStoredPass,
	}

	if entry != nil {
		f.editing = true
		f.originalAlias = entry.Alias
		f.inputs[fieldAlias].SetValue(entry.Alias)
		f.inputs[fieldHostName].SetValue(entry.HostName)
		f.inputs[fieldUser].SetValue(entry.User)
		if entry.Port != "" && entry.Port != "22" {
			f.inputs[fieldPort].SetValue(entry.Port)
		}
		f.inputs[fieldIdentityFile].SetValue(entry.IdentityFile)
		if hasStoredPass {
			f.inputs[fieldPassword].SetValue(storedPass)
		}
	}

	f.inputs[0].Focus()
	return f
}

func (f *hostForm) Update(msg tea.Msg) (tea.Cmd, bool, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return nil, false, true
		case "tab", "down":
			f.moveFocus(1)
			return nil, false, false
		case "shift+tab", "up":
			f.moveFocus(-1)
			return nil, false, false
		case "ctrl+r":
			if f.focus == fieldPassword {
				if f.inputs[fieldPassword].EchoMode == textinput.EchoPassword {
					f.inputs[fieldPassword].EchoMode = textinput.EchoNormal
				} else {
					f.inputs[fieldPassword].EchoMode = textinput.EchoPassword
				}
				return nil, false, false
			}
		case "ctrl+x":
			if f.focus == fieldPassword && (f.hasStoredPass || f.inputs[fieldPassword].Value() != "") {
				f.clearPass = true
				f.hasStoredPass = false
				f.inputs[fieldPassword].SetValue("")
				f.inputs[fieldPassword].Placeholder = "password cleared (save to apply)"
				return nil, false, false
			}
		case "enter":
			if f.focus < fieldCount-1 {
				f.moveFocus(1)
				return nil, false, false
			}
			if err := f.validate(); err != "" {
				f.errMsg = err
				return nil, false, false
			}
			return nil, true, false
		}
	}

	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	return cmd, false, false
}

func (f *hostForm) moveFocus(delta int) {
	f.inputs[f.focus].Blur()
	f.focus = (f.focus + delta + fieldCount) % fieldCount
	f.inputs[f.focus].Focus()
}

func (f *hostForm) validate() string {
	alias := strings.TrimSpace(f.inputs[fieldAlias].Value())
	host := strings.TrimSpace(f.inputs[fieldHostName].Value())
	if alias == "" {
		return "Alias is required"
	}
	if strings.ContainsAny(alias, " \t*?") {
		return "Alias cannot contain spaces or wildcards"
	}
	if host == "" {
		return "HostName is required"
	}
	return ""
}

func (f *hostForm) result() model.HostInput {
	port := strings.TrimSpace(f.inputs[fieldPort].Value())
	if port == "" {
		port = "22"
	}
	return model.HostInput{
		HostEntry: model.HostEntry{
			Alias:        strings.TrimSpace(f.inputs[fieldAlias].Value()),
			HostName:     strings.TrimSpace(f.inputs[fieldHostName].Value()),
			User:         strings.TrimSpace(f.inputs[fieldUser].Value()),
			Port:         port,
			IdentityFile: strings.TrimSpace(f.inputs[fieldIdentityFile].Value()),
		},
		Password:  f.inputs[fieldPassword].Value(),
		ClearPass: f.clearPass,
	}
}

func (f *hostForm) View(width int) string {
	heading := "Add Host"
	if f.editing {
		heading = fmt.Sprintf("Edit Host (%s)", f.originalAlias)
	}

	labels := []string{
		"Alias",
		"HostName",
		"User",
		"Port",
		"IdentityFile",
		"Password",
	}

	var rows []string
	rows = append(rows, titleStyle.Render(heading), "")
	for i, lbl := range labels {
		row := lipgloss.JoinHorizontal(
			lipgloss.Left,
			labelStyle.Render(lbl+":"),
			f.inputs[i].View(),
		)
		rows = append(rows, row)
	}

	if f.errMsg != "" {
		rows = append(rows, "", errorStyle.Render(f.errMsg))
	}

	rows = append(rows, "", mutedStyle.Render("tab/shift+tab move · ctrl+r reveal · enter next/save · esc cancel"))

	boxWidth := width - 4
	if boxWidth < 50 {
		boxWidth = 50
	}
	return activePaneStyle.Width(boxWidth).Render(strings.Join(rows, "\n"))
}
