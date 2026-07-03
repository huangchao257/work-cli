package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/huangchao257/work-cli/internal/adapter"
	"github.com/huangchao257/work-cli/internal/hooks"
	"github.com/huangchao257/work-cli/internal/pkg/copyutil"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/state"
)

func installHooks(ctx context.Context, pkgDir string, opts Options, refRaw string) (Result, error) {
	manifest, err := hooks.ParseDir(pkgDir)
	if err != nil {
		return Result{}, fmt.Errorf("解析 hooks.yaml 失败: %w", err)
	}
	if missing := hooks.CheckRequiredEnv(manifest); len(missing) > 0 {
		var b strings.Builder
		b.WriteString("缺少必需的环境变量：")
		b.WriteString(strings.Join(missing, ", "))
		b.WriteString("\n")
		for _, name := range missing {
			b.WriteString(platform.EnvSetHint(name))
			b.WriteString("\n")
		}
		return Result{}, fmt.Errorf("%s", b.String())
	}

	tcfg, _ := hooks.LoadTelemetryConfig()
	events, err := hooks.ResolveEvents(manifest, tcfg.Events)
	if err != nil {
		return Result{}, fmt.Errorf("解析 hooks 事件失败: %w", err)
	}

	targetIDEs := manifest.Targets
	if len(opts.IDEs) > 0 {
		targetIDEs = opts.IDEs
	}

	workBin, err := os.Executable()
	if err != nil {
		workBin = "work"
	}

	var installedIDEs []string
	var skipped []string
	var warnings []string
	var files []string

	sidecar := &hooks.Sidecar{
		Name:    manifest.Name,
		Version: manifest.Version,
		Scope:   opts.Scope,
		WorkBin: workBin,
		IDEs:    map[string]hooks.SidecarIDE{},
	}

	ideList := targetIDEs
	if len(ideList) == 0 {
		for _, a := range adapter.All() {
			ideList = append(ideList, a.Name())
		}
	}

	for _, ideName := range ideList {
		a, ok := adapter.ByName(ideName)
		if !ok {
			return Result{}, fmt.Errorf("未知 IDE: %s", ideName)
		}
		if !a.Detect() {
			if len(opts.IDEs) > 0 {
				return Result{}, fmt.Errorf("未检测到 IDE: %s", ideName)
			}
			skipped = append(skipped, ideName)
			warnings = append(warnings, fmt.Sprintf("未检测到 %s，已跳过", ideName))
			continue
		}

		bindings, bindWarnings := hooks.BindingsForIDE(ideName, events)
		warnings = append(warnings, bindWarnings...)

		configPath, err := hooks.HooksConfigPath(ideName, opts.Scope)
		if err != nil {
			return Result{}, fmt.Errorf("定位 hooks 配置路径失败: %w", err)
		}
		scriptDir, err := hooks.HooksScriptDir(ideName, opts.Scope, manifest.Name)
		if err != nil {
			return Result{}, fmt.Errorf("定位 hooks 脚本目录失败: %w", err)
		}

		if opts.DryRun {
			files = append(files, configPath, scriptDir)
			installedIDEs = append(installedIDEs, ideName)
			continue
		}

		if err := os.MkdirAll(scriptDir, 0o755); err != nil {
			return Result{}, fmt.Errorf("创建脚本目录失败: %w", err)
		}

		for _, hr := range manifest.Resources.Hooks {
			src := filepath.Join(pkgDir, hr.Source)
			dst := filepath.Join(scriptDir, filepath.Base(hr.Source))
			if err := copyutil.CopyFile(src, dst); err != nil {
				return Result{}, fmt.Errorf("复制 hook 脚本 %s 失败: %w", hr.Source, err)
			}
			files = append(files, dst)
		}

		baseScript := filepath.Join(scriptDir, "telemetry.sh")
		if err := hooks.WriteTelemetryScript(baseScript, workBin, manifest.Name, opts.Scope); err != nil {
			return Result{}, fmt.Errorf("写入 telemetry 脚本失败: %w", err)
		}
		files = append(files, baseScript)

		var entries []hooks.SidecarEntry
		for _, b := range bindings {
			wrapperName := wrapperFileName(b)
			wrapperPath := filepath.Join(scriptDir, wrapperName)
			if err := hooks.WriteWrapperScript(wrapperPath, baseScript, ideName, b.IDEEvent, manifest.Name, opts.Scope); err != nil {
				return Result{}, fmt.Errorf("写入 wrapper 脚本 %s 失败: %w", wrapperName, err)
			}
			cmdPath, err := hooks.CommandPathForIDE(ideName, opts.Scope, manifest.Name, wrapperName)
			if err != nil {
				return Result{}, fmt.Errorf("解析 wrapper 命令路径失败: %w", err)
			}
			entries = append(entries, hooks.SidecarEntry{
				IDEEvent: b.IDEEvent,
				Matcher:  b.Matcher,
				Command:  cmdPath,
				WorkID:   "work-telemetry",
			})
			files = append(files, wrapperPath)
		}

		switch ideName {
		case "cursor":
			if err := hooks.MergeCursorHooks(configPath, entries); err != nil {
				return Result{}, fmt.Errorf("合并 Cursor hooks 失败: %w", err)
			}
		default:
			if err := hooks.MergeSettingsHooks(configPath, entries); err != nil {
				return Result{}, fmt.Errorf("合并 settings hooks 失败: %w", err)
			}
		}
		files = append(files, configPath)

		sidecar.IDEs[ideName] = hooks.SidecarIDE{
			ConfigPath: configPath,
			ScriptDir:  scriptDir,
			Entries:    entries,
		}
		installedIDEs = append(installedIDEs, ideName)
	}

	if len(installedIDEs) == 0 && len(ideList) > 0 {
		return Result{}, fmt.Errorf("未检测到任何目标 IDE")
	}

	if !opts.DryRun {
		if err := hooks.SaveSidecar(sidecar); err != nil {
			return Result{}, fmt.Errorf("保存 sidecar 记录失败: %w", err)
		}
		rec := state.BundleRecord{
			Name:    manifest.Name,
			Kind:    "hooks",
			Version: manifest.Version,
			Scope:   opts.Scope,
			Ref:     refRaw,
			IDEs:    installedIDEs,
			Resources: state.BundleResources{
				Hooks: hookIDs(manifest),
			},
			Telemetry: &state.TelemetryInfo{Events: events},
		}
		statePath, err := platform.WorkStatePath(opts.Scope)
		if err != nil {
			return Result{}, fmt.Errorf("定位状态文件路径失败: %w", err)
		}
		store, err := state.Open(statePath)
		if err != nil {
			return Result{}, fmt.Errorf("打开状态文件失败: %w", err)
		}
		if err := store.Upsert(rec); err != nil {
			return Result{}, fmt.Errorf("写入安装记录失败: %w", err)
		}
	}

	return Result{
		Success:       true,
		Name:          manifest.Name,
		Kind:          "hooks",
		Version:       manifest.Version,
		Scope:         opts.Scope,
		InstalledIDEs: installedIDEs,
		SkippedIDEs:   skipped,
		Warnings:      warnings,
		FilesWritten:  files,
		DryRun:        opts.DryRun,
	}, nil
}

func uninstallHooks(ctx context.Context, rec *state.BundleRecord, dryRun bool) error {
	sc, err := hooks.LoadSidecar(rec.Name)
	if err != nil {
		// fallback: try paths from record
		return uninstallHooksFallback(rec, dryRun)
	}
	if dryRun {
		return nil
	}
	for ide, info := range sc.IDEs {
		switch ide {
		case "cursor":
			if err := hooks.UnmergeCursorHooks(info.ConfigPath); err != nil {
				return fmt.Errorf("移除 Cursor hooks 失败: %w", err)
			}
		default:
			if err := hooks.UnmergeSettingsHooks(info.ConfigPath); err != nil {
				return fmt.Errorf("移除 settings hooks 失败: %w", err)
			}
		}
		// 清理脚本目录：失败不阻断卸载流程，残留文件可在下次卸载时清理
		_ = os.RemoveAll(info.ScriptDir)
	}
	// 移除 sidecar 记录：失败不阻断，记录残留不影响 IDE 行为
	_ = hooks.RemoveSidecar(rec.Name)
	return nil
}

func uninstallHooksFallback(rec *state.BundleRecord, dryRun bool) error {
	if dryRun {
		return nil
	}
	for _, ide := range rec.IDEs {
		configPath, err := hooks.HooksConfigPath(ide, rec.Scope)
		if err != nil {
			continue
		}
		switch ide {
		case "cursor":
			// 尽力移除，失败时配置文件可能已不存在
			_ = hooks.UnmergeCursorHooks(configPath)
		default:
			_ = hooks.UnmergeSettingsHooks(configPath)
		}
		scriptDir, err := hooks.HooksScriptDir(ide, rec.Scope, rec.Name)
		if err == nil {
			// 清理脚本目录：失败可忽略
			_ = os.RemoveAll(scriptDir)
		}
	}
	// 移除 sidecar 记录：失败可忽略
	_ = hooks.RemoveSidecar(rec.Name)
	return nil
}

func hookIDs(m *hooks.Manifest) []string {
	out := make([]string, 0, len(m.Resources.Hooks))
	for _, h := range m.Resources.Hooks {
		out = append(out, h.ID)
	}
	return out
}

func wrapperFileName(b hooks.Binding) string {
	name := strings.ToLower(b.IDEEvent)
	if b.Matcher != "" {
		name += "-" + strings.ReplaceAll(b.Matcher, "|", "-")
	}
	name = strings.NewReplacer(" ", "", "*", "star", ".", "").Replace(name)
	return "run-" + name + ".sh"
}
