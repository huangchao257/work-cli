package engine

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/huangchao257/work-cli/internal/bundle"
	"github.com/huangchao257/work-cli/internal/installer"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/state"
)

func installCLI(ctx context.Context, pkgDir string, opts Options, refRaw string) (Result, error) {
	manifest, err := installer.ParseDir(pkgDir)
	if err != nil {
		return Result{}, fmt.Errorf("解析 installer.yaml 失败: %w", err)
	}
	if missing := bundle.CheckRequiredEnvVars(manifest.Env); len(missing) > 0 {
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

	if err := runInDir(ctx, pkgDir, cmd); err != nil {
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
	statePath, err := platform.WorkStatePath("user")
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

func runInDir(ctx context.Context, dir, command string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前工作目录失败: %w", err)
	}
	// 仅在 Chdir 成功后才需要恢复；失败时进程仍处于原 CWD，无需恢复。
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("切换工作目录到 %s 失败: %w", dir, err)
	}
	defer func() {
		// 恢复原 CWD；失败时输出到 stderr，避免静默丢弃。
		// 注意：os.Chdir 是进程全局操作，调用方应确保无并发 goroutine 依赖 CWD。
		if rerr := os.Chdir(cwd); rerr != nil {
			fmt.Fprintf(os.Stderr, "警告: 恢复工作目录失败: %v\n", rerr)
		}
	}()
	return installer.Run(ctx, command)
}
