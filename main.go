// Command yerba is a terminal app (TUI) that talks to a filipgorny/agent.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/filipgorny/agent"
	"github.com/filipgorny/agent/config"

	"github.com/filipgorny/yerba/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "yerba:", err)
		os.Exit(1)
	}
}

func run() error {
	cfgPath := "yerba.yaml"

	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	ag, err := agent.NewAgentFrom(config.YamlFile(cfgPath))

	if err != nil {
		return err
	}

	program := tea.NewProgram(tui.New(ag), tea.WithAltScreen())

	_, err = program.Run()

	return err
}
