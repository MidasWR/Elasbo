package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MidasWR/Elasbo/internal/cloaks"
	"github.com/MidasWR/Elasbo/internal/config"
	"github.com/MidasWR/Elasbo/internal/tui"
)

func main() {
	cfgPath := flag.String("config", config.DefaultPath(), "path to config YAML")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			d := config.Defaults()
			cfg = &d
			if wErr := config.Save(*cfgPath, cfg); wErr != nil {
				fmt.Fprintf(os.Stderr, "write default config: %v\n", wErr)
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "config: %v\n", err)
			os.Exit(1)
		}
	}

	vault, err := cloaks.Open(cfg.VaultDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vault: %v\n", err)
		os.Exit(1)
	}

	m := tui.New(cfg, *cfgPath, vault)
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithInputTTY(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
