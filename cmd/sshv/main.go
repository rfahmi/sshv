package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"sshv/internal/config"
	"sshv/internal/tui"
	"sshv/internal/vault"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("sshv %s\n", version)
		return
	}

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
