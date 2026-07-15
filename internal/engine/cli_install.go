package engine

import (
	"context"
	"fmt"

	"github.com/huangchao257/work-cli/internal/installer"
	"github.com/huangchao257/work-cli/internal/state"
)

func installCLI(ctx context.Context, pkgDir string, opts Options, refRaw string) (Result, error) {
	manifest, err := installer.ParseDir(pkgDir)
	if err != nil {
		return Result{}, fmt.Errorf("解析 installer.yaml 失败: %w", err)
	}
	if err := checkMissingEnv(installer.RequiredEnvNames(manifest.Env)); err != nil {
		return Result{}, err
	}

	cmd, err := installer.ResolveCommand(manifest.Install)
	if err != nil {
		return Result{}, fmt.Errorf("解析安装命令失败: %w", err)
	}

	warnings := []string{}
	if opts.Scope == "project" {
		warnings = append(warnings, "cli 类型忽略 project scope，将按用户级全局 CLI 安装")
	}

	if opts.DryRun {
		return Result{
			Success:  true,
			Name:     manifest.Name,
			Kind:     "cli",
			Version:  manifest.Version,
			Scope:    "user",
			Commands: []string{cmd},
			DryRun:   true,
			Warnings: warnings,
		}, nil
	}

	if err := installer.RunInDir(ctx, pkgDir, cmd); err != nil {
		return Result{}, fmt.Errorf("执行安装命令失败: %w", err)
	}
	if manifest.Verify != nil && len(manifest.Verify.Command) > 0 {
		if err := installer.RunCommand(ctx, manifest.Verify.Command); err != nil {
			warnings = append(warnings, "安装完成，但验证命令失败: "+err.Error())
		}
	}

	rec := state.BundleRecord{
		Name:           manifest.Name,
		Kind:           "cli",
		Version:        manifest.Version,
		Scope:          "user",
		Ref:            refRaw,
		InstallCommand: cmd,
	}
	if err := saveStateRecord(rec, "user"); err != nil {
		return Result{}, err
	}

	return Result{
		Success:  true,
		Name:     manifest.Name,
		Kind:     "cli",
		Version:  manifest.Version,
		Scope:    "user",
		Commands: []string{cmd},
		Warnings: warnings,
	}, nil
}
