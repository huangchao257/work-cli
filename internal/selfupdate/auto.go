package selfupdate

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type AutoOptions struct {
	CurrentVersion string
	Force          bool
}

type AutoResult struct {
	Checked         bool   `json:"checked"`
	UpdateAvailable bool   `json:"update_available"`
	Updated         bool   `json:"updated"`
	Latest          string `json:"latest,omitempty"`
	Message         string `json:"message,omitempty"`
}

// ShouldAutoUpdate 判断是否应在本次启动时尝试自动更新。
func ShouldAutoUpdate(currentVersion string, cfg Config) bool {
	if strings.TrimSpace(currentVersion) == "" || currentVersion == "dev" {
		return false
	}
	return cfg.Enabled
}

// TryAuto 在后台策略下检查并自动更新 work 自身。
func TryAuto(ctx context.Context, opts AutoOptions) (*AutoResult, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	if opts.Force {
		cfg.Enabled = true
	}
	if !ShouldAutoUpdate(opts.CurrentVersion, cfg) {
		return &AutoResult{}, nil
	}

	checkNow, err := shouldCheckNow(cfg.CheckInterval, opts.Force)
	if err != nil {
		return nil, err
	}
	if !checkNow {
		return &AutoResult{}, nil
	}

	updater := NewUpdater(opts.CurrentVersion)
	res, err := updater.Upgrade(ctx, UpgradeOptions{})
	if err != nil {
		_ = markChecked()
		return nil, err
	}
	_ = markChecked()

	out := &AutoResult{
		Checked:         true,
		UpdateAvailable: res.UpdateAvailable,
		Latest:          res.Latest,
	}
	if !res.UpdateAvailable {
		return out, nil
	}
	out.Updated = true
	out.Message = fmt.Sprintf("work 已自动更新到 %s", res.Latest)
	return out, nil
}

func NotifyAutoUpdate(stderr *os.File, res *AutoResult) {
	if res == nil || !res.Updated || res.Message == "" {
		return
	}
	_, _ = fmt.Fprintf(stderr, "==> %s，正在重新执行命令...\n", res.Message)
}
