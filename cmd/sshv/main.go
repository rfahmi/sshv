package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"sshv/internal/config"
	"sshv/internal/tui"
	"sshv/internal/vault"
)

func main() {
	cfg, err := config.NewDefaultStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sshv: %v\n", err)
		os.Exit(1)
	}

	app, err := tui.NewApp(cfg, vault.NewKeychainVault())
	if err != nil {
		fmt.Fprintf(os.Stderr, "sshv: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sshv: %v\n", err)
		os.Exit(1)
	}
}
