// Package app wires the agent and session from yerba config (no UI here, so the
// app is not coupled to the TUI).
package app

import (
	"fmt"

	agentpkg "github.com/filipgorny/agent"
	session "github.com/filipgorny/agent-session"
	agentcfg "github.com/filipgorny/agent/config"

	"github.com/filipgorny/yerba/internal/config"
)

// Build constructs an interactive agent and wraps it in a persisted session.
func Build(cfg config.Config) (*session.Session, error) {
	ag, err := agentpkg.NewAgentFrom(agentcfg.YamlFile(cfg.AgentConfig))

	if err != nil {
		return nil, err
	}

	store, err := newStore(cfg.Session)

	if err != nil {
		return nil, err
	}

	return session.New(ag, store), nil
}

func newStore(c config.SessionConfig) (session.Store, error) {
	switch c.Backend {

	case "", "sqlite":
		return session.NewSQLite(c.Path)

	case "inmemory":
		return session.NewInMemory(), nil

	default:
		return nil, fmt.Errorf("yerba: unknown session backend %q", c.Backend)
	}
}
