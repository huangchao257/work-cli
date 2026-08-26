package engine

import (
	"context"
	"fmt"

	"github.com/huangchao257/work-cli/internal/adapter"
	"github.com/huangchao257/work-cli/internal/installer"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/source"
	"github.com/huangchao257/work-cli/internal/state"
)

func Uninstall(ctx context.Context, name, scope string, dryRun bool) (Result, error) {
	if scope == "" {
		scope = "user"
	}
	rec, store, err := findRecord(name, scope)
	if err != nil {
		return Result{}, err
	}

	var commands []string
	warnings := []string{}
	installedIDEs := rec.IDEs

	switch rec.Kind {
	case "hooks":
		if !dryRun {
			if err := uninstallHooks(ctx, rec, false); err != nil {
				return Result{}, fmt.Errorf("卸载 hooks 失败: %w", err)
			}
		}
	case "cli":
		{
			var resolveErrors []string
			// 仅接受受信资源名；项目级 installed.json 可能来自不可信仓库，
			// 禁止 ./本地目录 与 git: 引用（防止卸载时执行任意命令）。
			ref, err := source.ParseTrustedRef(rec.Name)
			if err != nil {
				// 兼容旧记录：Ref 名称可能非规范；回退到受信的 rec.Ref，仍拒绝本地/git。
				ref, err = source.ParseTrustedRef(rec.Ref)
			}
			if err != nil {
				resolveErrors = append(resolveErrors, fmt.Sprintf("解析引用失败: %v", err))
			} else {
				pkgDir, err := source.Resolve(ref)
				if err != nil {
					resolveErrors = append(resolveErrors, fmt.Sprintf("解析包目录失败: %v", err))
				} else {
					manifest, err := installer.ParseDir(pkgDir)
					if err != nil {
						resolveErrors = append(resolveErrors, fmt.Sprintf("解析 installer.yaml 失败: %v", err))
					} else if manifest.Uninstall != nil {
						cmd, err := installer.ResolveCommand(*manifest.Uninstall)
						if err != nil {
							resolveErrors = append(resolveErrors, fmt.Sprintf("解析卸载命令失败: %v", err))
						} else {
							commands = append(commands, cmd)
							if !dryRun {
								if err := installer.RunInDir(ctx, pkgDir, cmd); err != nil {
									return Result{}, fmt.Errorf("执行卸载命令失败: %w", err)
								}
							}
						}
					}
				}
			}
			if len(commands) == 0 {
				msg := fmt.Sprintf("未找到 %s 的卸载命令，请手动卸载（例如 npm uninstall -g @fission-ai/openspec）", name)
				if len(resolveErrors) > 0 {
					msg += "。原因: " + resolveErrors[len(resolveErrors)-1]
				}
				warnings = append(warnings, msg)
			}
		}
	default:
		if !dryRun {
			scopeVal := adapter.Scope(rec.Scope)
			for _, ide := range rec.IDEs {
				a, ok := adapter.ByName(ide)
				if !ok {
					continue
				}
				if err := a.Uninstall(ctx, *rec, scopeVal); err != nil {
					return Result{}, fmt.Errorf("从 %s 卸载失败: %w", ide, err)
				}
			}
		}
	}

	if !dryRun {
		if err := store.Remove(rec.Name, rec.Scope); err != nil {
			return Result{}, fmt.Errorf("移除安装记录失败: %w", err)
		}
	}

	return Result{
		Success:       true,
		Name:          rec.Name,
		Kind:          rec.Kind,
		Version:       rec.Version,
		Scope:         rec.Scope,
		InstalledIDEs: installedIDEs,
		Commands:      commands,
		Warnings:      warnings,
		DryRun:        dryRun,
	}, nil
}

func findRecord(name, scope string) (*state.BundleRecord, *state.Store, error) {
	statePath, err := platform.WorkStatePath(scope)
	if err != nil {
		return nil, nil, fmt.Errorf("定位状态文件路径失败: %w", err)
	}
	store, err := state.Open(statePath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开状态文件失败: %w", err)
	}
	rec, firstErr := store.Find(name, scope)
	if firstErr == nil {
		return rec, store, nil
	}
	if scope != "user" {
		statePath, err = platform.WorkStatePath("user")
		if err != nil {
			return nil, nil, fmt.Errorf("定位用户状态文件路径失败（原作用域 %s 错误: %v）: %w", scope, firstErr, err)
		}
		store, err = state.Open(statePath)
		if err != nil {
			return nil, nil, fmt.Errorf("打开用户状态文件失败（原作用域 %s 错误: %v）: %w", scope, firstErr, err)
		}
		rec, err = store.Find(name, "user")
		if err == nil {
			return rec, store, nil
		}
		return nil, nil, fmt.Errorf("作用域 %s 查找失败: %v；用户作用域查找失败: %w", scope, firstErr, err)
	}
	return nil, nil, firstErr
}
