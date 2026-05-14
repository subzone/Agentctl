package policy

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type configFile struct {
	Policy struct {
		Rules []Rule `yaml:"rules"`
	} `yaml:"policy"`
}

// LoadRules reads policy.rules from the given config files and concatenates them.
func LoadRules(paths ...string) ([]Rule, error) {
	out := make([]Rule, 0)
	for _, p := range paths {
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		var cfg configFile
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return nil, err
		}
		out = append(out, cfg.Policy.Rules...)
	}
	return out, nil
}

// FindProjectConfigPath locates .m/config.yaml from cwd to filesystem root.
func FindProjectConfigPath(cwd string) string {
	dir := cwd
	for {
		path := filepath.Join(dir, ".m", "config.yaml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
