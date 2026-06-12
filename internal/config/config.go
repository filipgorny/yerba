// Package config loads yerba's app configuration (separate from the agent config).
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is yerba's configuration.
type Config struct {
	// AgentConfig is the path to the agent config (see filipgorny/agent).
	AgentConfig string `yaml:"agent_config"`

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

	if c.AgentConfig == "" {
		c.AgentConfig = "default_agent_config.yaml"
	}

	if len(c.UI.LogSubtypes) == 0 {
		c.UI.LogSubtypes = []string{"TOOL_CALL", "TOOL_RESULT", "ERROR"}
	}

	return c, nil
}

func defaults() Config {
	return Config{
		AgentConfig: "default_agent_config.yaml",
		Session:     SessionConfig{Backend: "sqlite", Path: "./yerba-session.db"},
		UI:          UIConfig{LogSubtypes: []string{"TOOL_CALL", "TOOL_RESULT", "ERROR"}},
	}
}
