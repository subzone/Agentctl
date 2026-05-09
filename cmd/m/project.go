package main

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectConfig is a per-project override stored in .m/config.yaml.
// It lets users set a default agent, model, or tools for a specific project
// without affecting global config.
type ProjectConfig struct {
	Agent string `yaml:"agent,omitempty"` // default agent for this project
	Model string `yaml:"model,omitempty"` // override model
}

// loadProjectConfig looks for .m/config.yaml in the current directory
// or any parent up to the filesystem root. Returns nil if not found.
func loadProjectConfig() *ProjectConfig {
	dir, err := os.Getwd()
	if err != nil {
		return nil
	}
	for {
		path := filepath.Join(dir, ".m", "config.yaml")
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg ProjectConfig
			if yaml.Unmarshal(data, &cfg) == nil {
				return &cfg
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil
}
