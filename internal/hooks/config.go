package hooks

import (
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/huangchao257/work-cli/internal/configcache"
	"github.com/huangchao257/work-cli/internal/platform"
)

type TelemetryConfig struct {
	Enabled      bool     `yaml:"enabled"`
	URL          string   `yaml:"url"`
	BatchSize    int      `yaml:"batch_size"`
	SyncInterval string   `yaml:"sync_interval"`
	MaxRetries   int      `yaml:"max_retries"`
	Events       []string `yaml:"events"`
	Redact       []string `yaml:"redact"`
}

type userConfigFile struct {
	Telemetry TelemetryConfig `yaml:"telemetry"`
}

func LoadTelemetryConfig() (TelemetryConfig, error) {
	cfg := defaultTelemetryConfig()
	path, err := platform.ConfigFilePath()
	if err != nil {
		return cfg, err
	}
	data, err := configcache.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	var file userConfigFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return cfg, err
	}
	mergeTelemetryConfig(&cfg, file.Telemetry)
	return cfg, nil
}

func defaultTelemetryConfig() TelemetryConfig {
	return TelemetryConfig{
		Enabled:      true,
		BatchSize:    50,
		SyncInterval: "5m",
		MaxRetries:   10,
		Redact: []string{
			"prompt",
			"file_content",
			"tool_input.content",
			"content",
		},
	}
}

func mergeTelemetryConfig(dst *TelemetryConfig, src TelemetryConfig) {
	if src.URL != "" {
		dst.URL = src.URL
	}
	if src.BatchSize > 0 {
		dst.BatchSize = src.BatchSize
	}
	if src.SyncInterval != "" {
		dst.SyncInterval = src.SyncInterval
	}
	if src.MaxRetries > 0 {
		dst.MaxRetries = src.MaxRetries
	}
	if len(src.Events) > 0 {
		dst.Events = src.Events
	}
	if len(src.Redact) > 0 {
		dst.Redact = src.Redact
	}
	// enabled: only override if explicitly set in yaml — use pointer would be better;
	// for MVP treat URL presence as signal; zero value enabled stays true unless WORK_TELEMETRY_ENABLED=false
	if v := os.Getenv("WORK_TELEMETRY_ENABLED"); v != "" {
		dst.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if u := os.Getenv("WORK_TELEMETRY_URL"); u != "" {
		dst.URL = u
	}
}

func (c TelemetryConfig) SyncIntervalDuration() time.Duration {
	if c.SyncInterval == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(c.SyncInterval)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}

func ResolveRedactFields(m *Manifest, cfg TelemetryConfig) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, s := range cfg.Redact {
		add(s)
	}
	if m != nil {
		for _, s := range m.Telemetry.Redact {
			add(s)
		}
	}
	return out
}

func TelemetryDir() (string, error) {
	return platform.WorkSubDir("telemetry")
}

func HooksInstalledDir() (string, error) {
	return platform.WorkSubDir("hooks-installed")
}
