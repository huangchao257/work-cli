package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultCheckInterval = 2 * time.Hour

type Config struct {
	Enabled       bool
	CheckInterval time.Duration
}

type fileConfig struct {
	SelfUpdate struct {
		Enabled       *bool  `yaml:"enabled"`
		CheckInterval string `yaml:"check_interval"`
	} `yaml:"self_update"`
}

func LoadConfig() (Config, error) {
	cfg := defaultConfig()
	home, err := os.UserHomeDir()
	if err != nil {
		return applyEnv(cfg), nil
	}
	path := filepath.Join(home, ".work", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return applyEnv(cfg), nil
		}
		return cfg, err
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return cfg, err
	}
	if fc.SelfUpdate.Enabled != nil {
		cfg.Enabled = *fc.SelfUpdate.Enabled
	}
	if raw := strings.TrimSpace(fc.SelfUpdate.CheckInterval); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return cfg, fmt.Errorf("解析 self_update.check_interval 失败: %w", err)
		}
		cfg.CheckInterval = d
	}
	return applyEnv(cfg), nil
}

func defaultConfig() Config {
	return Config{
		Enabled:       true,
		CheckInterval: defaultCheckInterval,
	}
}

func applyEnv(cfg Config) Config {
	v, ok := os.LookupEnv("WORK_AUTO_UPDATE")
	if !ok {
		return cfg
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		cfg.Enabled = true
	case "0", "false", "no", "off":
		cfg.Enabled = false
	}
	return cfg
}
