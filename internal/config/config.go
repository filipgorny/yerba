// Package config loads yerba's app configuration (separate from the agent config).
package config

import (
	"fmt"
	"os"

	agentcfg "github.com/filipgorny/agent/config"
	"gopkg.in/yaml.v3"
)

// Config is yerba's configuration.
type Config struct {
	// Agent is the embedded agent configuration (see filipgorny/agent). yerba
	// owns this section directly rather than pointing at a separate file.
	Agent agentcfg.Config `yaml:"agent"`

	// Session configures where the session is persisted.
	Session SessionConfig `yaml:"session"`

	// UI configures the terminal interface.
	UI UIConfig `yaml:"ui"`
}

// SessionConfig selects the session store backend.
type SessionConfig struct {
	Backend string `yaml:"backend"` // "sqlite" (default) | "inmemory"
	Path    string `yaml:"path"`
}

// UIConfig controls what the TUI shows.
type UIConfig struct {
	// LogSubtypes are the LOG subtypes shown in the transcript.
	LogSubtypes []string `yaml:"log_subtypes"`
}

// Load reads a yerba config file, applying defaults.
func Load(path string) (Config, error) {
	c := defaults()

	data, err := os.ReadFile(path)

	if err != nil {
		return Config{}, fmt.Errorf("yerba: read config %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("yerba: parse config: %w", err)
	}

	if len(c.UI.LogSubtypes) == 0 {
		c.UI.LogSubtypes = []string{"TOOL_CALL", "TOOL_RESULT", "ERROR"}
	}

	return c, nil
}

func defaults() Config {
	return Config{
		Session: SessionConfig{Backend: "sqlite", Path: "./yerba-session.db"},
		UI:      UIConfig{LogSubtypes: []string{"TOOL_CALL", "TOOL_RESULT", "ERROR"}},
	}
}
