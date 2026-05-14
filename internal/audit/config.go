package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Backend    string `yaml:"backend"`
	Path       string `yaml:"path"`
	HMACSecret string `yaml:"hmac_secret"`
	Endpoint   string `yaml:"endpoint"`
	Token      string `yaml:"token"`
	TLSVerify  bool   `yaml:"tls_verify"`
	BatchSize  int    `yaml:"batch_size"`
	// FlushInterval controls periodic batch delivery.
	FlushInterval time.Duration `yaml:"-"`
}

type fileConfig struct {
	Audit struct {
		Backend    string `yaml:"backend"`
		Path       string `yaml:"path"`
		HMACSecret string `yaml:"hmac_secret"`
		Endpoint   string `yaml:"endpoint"`
		Token      string `yaml:"token"`
		TLSVerify  *bool  `yaml:"tls_verify"`
		BatchSize  int    `yaml:"batch_size"`
		FlushEvery string `yaml:"flush_interval"`
		File       struct {
			Path string `yaml:"path"`
		} `yaml:"file"`
		Splunk struct {
			Endpoint  string `yaml:"endpoint"`
			Token     string `yaml:"token"`
			TLSVerify *bool  `yaml:"tls_verify"`
		} `yaml:"splunk"`
	} `yaml:"audit"`
}

// FindProjectConfigPath locates .m/config.yaml from cwd up to fs root.
func FindProjectConfigPath(cwd string) string {
	dir := cwd
	for {
		p := filepath.Join(dir, ".m", "config.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func LoadConfig(paths ...string) (Config, error) {
	cfg := Config{Backend: "none", TLSVerify: true, BatchSize: 1, FlushInterval: 3 * time.Second}
	for _, p := range paths {
		if p == "" {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Config{}, err
		}
		var f fileConfig
		if err := yaml.Unmarshal(raw, &f); err != nil {
			return Config{}, fmt.Errorf("%s: %w", p, err)
		}
		if f.Audit.Backend != "" {
			cfg.Backend = strings.ToLower(strings.TrimSpace(f.Audit.Backend))
		}
		if f.Audit.Path != "" {
			cfg.Path = expandEnvToken(f.Audit.Path)
		}
		if f.Audit.File.Path != "" {
			cfg.Path = expandEnvToken(f.Audit.File.Path)
		}
		if f.Audit.HMACSecret != "" {
			cfg.HMACSecret = expandEnvToken(f.Audit.HMACSecret)
		}
		if f.Audit.Endpoint != "" {
			cfg.Endpoint = expandEnvToken(f.Audit.Endpoint)
		}
		if f.Audit.Token != "" {
			cfg.Token = expandEnvToken(f.Audit.Token)
		}
		if f.Audit.TLSVerify != nil {
			cfg.TLSVerify = *f.Audit.TLSVerify
		}
		if f.Audit.BatchSize > 0 {
			cfg.BatchSize = f.Audit.BatchSize
		}
		if f.Audit.FlushEvery != "" {
			d, err := time.ParseDuration(strings.TrimSpace(f.Audit.FlushEvery))
			if err != nil {
				return Config{}, fmt.Errorf("%s: invalid audit.flush_interval: %w", p, err)
			}
			cfg.FlushInterval = d
		}
		if f.Audit.Splunk.Endpoint != "" {
			cfg.Endpoint = expandEnvToken(f.Audit.Splunk.Endpoint)
		}
		if f.Audit.Splunk.Token != "" {
			cfg.Token = expandEnvToken(f.Audit.Splunk.Token)
		}
		if f.Audit.Splunk.TLSVerify != nil {
			cfg.TLSVerify = *f.Audit.Splunk.TLSVerify
		}
	}
	return cfg, nil
}

func expandEnvToken(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		key := strings.TrimSuffix(strings.TrimPrefix(s, "${"), "}")
		return os.Getenv(key)
	}
	return s
}
