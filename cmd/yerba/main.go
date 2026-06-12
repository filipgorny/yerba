// Command yerba is a terminal (TUI) interface to a filipgorny/agent session.
package main

import (
	"fmt"
	"os"

	"github.com/filipgorny/yerba/internal/app"
	"github.com/filipgorny/yerba/internal/config"
	"github.com/filipgorny/yerba/internal/tui"
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

	cfg, err := config.Load(cfgPath)

	if err != nil {
		return err
	}

	sess, err := app.Build(cfg)

	if err != nil {
		return err
	}

	return tui.Run(sess, cfg.UI)
}
