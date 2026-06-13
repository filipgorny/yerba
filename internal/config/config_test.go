package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "y.yaml")
	_ = os.WriteFile(p, []byte("agent:\n  language: Polish\nsession:\n  backend: inmemory\n"), 0o644)

	c, err := Load(p)

	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if c.Agent.Language != "Polish" {
		t.Errorf("agent.language = %q", c.Agent.Language)
	}

	if c.Session.Backend != "inmemory" {
		t.Errorf("backend = %q", c.Session.Backend)
	}

	if len(c.UI.LogSubtypes) == 0 {
		t.Error("default log_subtypes not applied")
	}
}
