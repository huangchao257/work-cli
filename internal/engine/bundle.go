package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/huangchao257/work-cli/internal/adapter"
	"github.com/huangchao257/work-cli/internal/bundle"
	"github.com/huangchao257/work-cli/internal/graph"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/state"
)

func installBundle(ctx context.Context, pkgDir string, opts Options, refRaw string) (Result, error) {
	manifest, err := bundle.ParseDir(pkgDir)
	if err != nil {
		return Result{}, err
	}
	if missing := bundle.CheckRequiredEnv(manifest); len(missing) > 0 {
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

	targetIDEs := manifest.Targets
	if len(opts.IDEs) > 0 {
		targetIDEs = opts.IDEs
	}

	var adapters []adapter.Adapter
	var skipped []string
	var warnings []string
	for _, name := range targetIDEs {
		a, ok := adapter.ByName(name)
		if !ok {
			return Result{}, fmt.Errorf("未知 IDE: %s", name)
		}
		if !a.Detect() {
			if len(opts.IDEs) > 0 {
				return Result{}, fmt.Errorf("未检测到 IDE: %s", name)
			}
			skipped = append(skipped, name)
			warnings = append(warnings, fmt.Sprintf("未检测到 %s，已跳过", name))
			continue
		}
		adapters = append(adapters, a)
	}
	if len(adapters) == 0 && len(targetIDEs) > 0 && len(skipped) == len(targetIDEs) {
		return Result{}, fmt.Errorf("未检测到任何目标 IDE")
	}
	if len(adapters) == 0 {
		for _, a := range adapter.All() {
			if a.Detect() {
				adapters = append(adapters, a)
			}
		}
	}

	scope := adapter.Scope(opts.Scope)
	var files []string
	var installedIDEs []string

	for _, a := range adapters {
		installedIDEs = append(installedIDEs, a.Name())
		for _, skill := range manifest.Resources.Skills {
			if opts.DryRun {
				dest, err := platform.SkillDir(platform.IDE(a.Name()), opts.Scope, skill.ID)
				if err != nil {
					return Result{}, err
				}
				files = append(files, dest)
				continue
			}
			path, err := a.InstallSkill(ctx, pkgDir, skill, scope)
			if err != nil {
				return Result{}, err
			}
			files = append(files, path)
		}
		for _, rule := range manifest.Resources.Rules {
			if opts.DryRun {
				dest, err := platform.RuleFile(platform.IDE(a.Name()), opts.Scope, rule.ID)
				if err != nil {
					return Result{}, err
				}
				files = append(files, dest)
				continue
			}
			path, err := a.InstallRule(ctx, pkgDir, rule, scope)
			if err != nil {
				return Result{}, err
			}
			files = append(files, path)
		}
		for _, mcp := range manifest.Resources.MCP {
			if opts.DryRun {
				dest, err := platform.MCPConfigPath(platform.IDE(a.Name()), opts.Scope)
				if err != nil {
					return Result{}, err
				}
				files = append(files, dest)
				continue
			}
			path, err := a.InstallMCP(ctx, pkgDir, mcp, scope)
			if err != nil {
				return Result{}, err
			}
			files = append(files, path)
		}
	}

	rec := state.BundleRecord{
		Name:    manifest.Name,
		Kind:    "bundle",
		Version: manifest.Version,
		Scope:   opts.Scope,
		Ref:     refRaw,
		IDEs:    installedIDEs,
		Resources: state.BundleResources{
			Skills: ids(manifest.Resources.Skills),
			Rules:  ruleIDs(manifest.Resources.Rules),
			MCP:    mcpIDs(manifest.Resources.MCP),
		},
	}
	if !opts.DryRun {
		statePath, err := platform.WorkStatePath(opts.Scope)
		if err != nil {
			return Result{}, err
		}
		store, err := state.Open(statePath)
		if err != nil {
			return Result{}, err
		}
		if err := store.Upsert(rec); err != nil {
			return Result{}, err
		}
		if err := runBundlePostInstall(ctx, manifest, opts); err != nil {
			warnings = append(warnings, fmt.Sprintf("安装后初始化未完成: %v（可手动执行 work graph init）", err))
		}
	}

	return Result{
		Success:       true,
		Name:          manifest.Name,
		Kind:          "bundle",
		Version:       manifest.Version,
		Scope:         opts.Scope,
		InstalledIDEs: installedIDEs,
		SkippedIDEs:   skipped,
		Warnings:      warnings,
		FilesWritten:  files,
		DryRun:        opts.DryRun,
	}, nil
}

func ids(skills []bundle.SkillResource) []string {
	out := make([]string, 0, len(skills))
	for _, s := range skills {
		out = append(out, s.ID)
	}
	return out
}

func ruleIDs(rules []bundle.RuleResource) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.ID)
	}
	return out
}

func mcpIDs(mcps []bundle.MCPResource) []string {
	out := make([]string, 0, len(mcps))
	for _, m := range mcps {
		out = append(out, m.ID)
	}
	return out
}

func runBundlePostInstall(ctx context.Context, manifest *bundle.Manifest, opts Options) error {
	if manifest.PostInstall == nil {
		return nil
	}
	when := manifest.PostInstall.WhenScope
	if when == "" {
		when = "project"
	}
	if when != "any" && when != opts.Scope {
		return nil
	}
	switch manifest.PostInstall.Action {
	case "graph_init", "":
		return graph.RunPostInstall(ctx, opts.Scope, opts.DryRun)
	default:
		return fmt.Errorf("未知 post_install.action: %s", manifest.PostInstall.Action)
	}
}
