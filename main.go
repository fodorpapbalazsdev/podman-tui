package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fpbpi/podman-tui/internal/config"
	"github.com/fpbpi/podman-tui/internal/podman"
	"github.com/fpbpi/podman-tui/internal/ui"
)

func main() {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not load presets:", err)
		cfg = &config.Config{}
	}

	svc := podman.NewService()
	m := ui.NewAppModel(svc, cfg.Presets)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error running podman-tui:", err)
		os.Exit(1)
	}
}
