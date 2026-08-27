package api

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/huangchao257/work-cli/internal/openapi"
)

// BuildShortcuts 合并系统级 shortcut（编译期）与配置型 shortcut（system.yaml）。
// 配置型 shortcut 校验 target 存在且风险不低于底层 operation；同名时内置优先。
// 校验产生的 warning 由调用方经 CollectShortcutWarnings 并入目录 warnings。
func BuildShortcuts(s System, cfg *SystemConfig) ([]Shortcut, error) {
	var out []Shortcut
	seen := map[string]bool{}

	if provider, ok := s.(Shortcuts); ok {
		for _, sc := range provider.Shortcuts() {
			if !strings.HasPrefix(sc.Name, "+") {
				sc.Name = "+" + sc.Name
			}
			if seen[sc.Name] {
				continue
			}
			seen[sc.Name] = true
			out = append(out, sc)
		}
	}
	if cfg != nil {
		names := make([]string, 0, len(cfg.Shortcuts))
		for name := range cfg.Shortcuts {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			def := cfg.Shortcuts[name]
			full := name
			if !strings.HasPrefix(full, "+") {
				full = "+" + full
			}
			if seen[full] {
				shortcutWarningsMu.Lock()
				shortcutWarnings = append(shortcutWarnings,
					fmt.Sprintf("配置型快捷方式 %s 与内置同名，配置被忽略", full))
				shortcutWarningsMu.Unlock()
				continue
			}
			seen[full] = true
			out = append(out, Shortcut{
				Name: full, Description: def.Description, Target: def.Target,
				Params: def.Params, Risk: def.Risk,
			})
		}
	}
	return out, nil
}

// shortcutWarnings 收集 BuildShortcuts 的配置校验警告（同名覆盖等）。
// 取走即清空（TakeShortcutWarnings），避免跨调用累积。
var (
	shortcutWarnings   []string
	shortcutWarningsMu sync.Mutex
)

// TakeShortcutWarnings 取出并清空 shortcut 装配期收集的警告。
func TakeShortcutWarnings() []string {
	shortcutWarningsMu.Lock()
	defer shortcutWarningsMu.Unlock()
	warnings := shortcutWarnings
	shortcutWarnings = nil
	return warnings
}

// ValidateShortcutTargets 校验配置型 shortcut 的 target 在目录中存在，
// 不存在时返回 warning（悬空 target 的执行错误延迟到调用期 fail-closed）。
func ValidateShortcutTargets(catalog *openapi.Catalog, cfg *SystemConfig) []string {
	if cfg == nil || len(cfg.Shortcuts) == 0 {
		return nil
	}
	var warnings []string
	for name, def := range cfg.Shortcuts {
		if def.Target == "" {
			continue // shortcutWarnings 已覆盖空 target
		}
		if _, ok := catalog.FindByID(def.Target); !ok {
			warnings = append(warnings, fmt.Sprintf("快捷方式 %s 的 target %q 不在目录中，执行将失败", name, def.Target))
		}
	}
	return warnings
}

// EffectiveShortcutRisk 计算 shortcut 的有效风险：不低于其目标 operation。
// handler 型 shortcut 无底层 operation 可比对：声明为空时按 dangerous 处理。
func EffectiveShortcutRisk(catalog *openapi.Catalog, sc Shortcut) RiskLevel {
	declared, err := ParseRiskLevel(sc.Risk)
	if err != nil {
		return RiskDangerous
	}
	if sc.Handler != nil {
		if strings.TrimSpace(sc.Risk) == "" {
			return RiskDangerous
		}
		return declared
	}
	if op, ok := catalog.FindByID(sc.Target); ok {
		underlying := AssessRisk(op)
		if underlying > declared {
			return underlying
		}
	}
	return declared
}

// ExecuteShortcut 执行快捷方式。自定义 Handler 直接编排；否则合并默认参数后走统一 Call。
func ExecuteShortcut(ctx context.Context, s System, sc Shortcut, opts CallOptions) (*CallResult, error) {
	if sc.Handler != nil {
		systemName := opts.System
		call := func(inner context.Context, innerOpts CallOptions) (*CallResult, error) {
			if innerOpts.System == "" {
				innerOpts.System = systemName
			}
			if innerOpts.AuthConfig == (AuthConfig{}) {
				innerOpts.AuthConfig = opts.AuthConfig
			}
			if innerOpts.Timeout == "" {
				innerOpts.Timeout = opts.Timeout
			}
			return Call(inner, s, innerOpts)
		}
		data, err := sc.Handler(ctx, s, call, sc.Params)
		if err != nil {
			return nil, err
		}
		if result, ok := data.(*CallResult); ok {
			return result, nil
		}
		return &CallResult{
			OK: true, System: opts.System, Operation: sc.Name,
			Data: data, DryRun: opts.DryRun,
		}, nil
	}

	// 配置型：合并默认参数（CLI 显式参数优先）
	merged := map[string]string{}
	for key, value := range sc.Params {
		merged[key] = value
	}
	for key, value := range opts.Params {
		merged[key] = value
	}
	opts.Params = merged
	opts.Operation = sc.Target
	return Call(ctx, s, opts)
}

// ShortcutsForSystem 返回系统的全部快捷方式。
func ShortcutsForSystem(s System, cfg *SystemConfig) ([]Shortcut, error) {
	return BuildShortcuts(s, cfg)
}

// FindShortcut 按名称（带或不带 + 前缀）查找快捷方式。
func FindShortcut(shortcuts []Shortcut, name string) (Shortcut, bool) {
	if !strings.HasPrefix(name, "+") {
		name = "+" + name
	}
	for _, sc := range shortcuts {
		if sc.Name == name {
			return sc, true
		}
	}
	return Shortcut{}, false
}
