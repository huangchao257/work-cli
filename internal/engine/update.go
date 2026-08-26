package engine

import (
	"context"
	"fmt"

	"github.com/huangchao257/work-cli/internal/installer"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/source"
	"github.com/huangchao257/work-cli/internal/state"
)

func Update(ctx context.Context, name, scope string, dryRun bool) ([]Result, error) {
	if scope == "" {
		scope = "user"
	}

	var targets []state.BundleRecord
	if name != "" {
		rec, _, err := findRecord(name, scope)
		if err != nil {
			return nil, err
		}
		targets = []state.BundleRecord{*rec}
	} else {
		statePath, err := platform.WorkStatePath(scope)
		if err != nil {
			return nil, err
		}
		store, err := state.Open(statePath)
		if err != nil {
			return nil, err
		}
		records, err := store.List("")
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			return nil, fmt.Errorf("当前范围 (%s) 没有已安装的资源，可先运行 work list 查看", scope)
		}
		targets = records
	}

	var results []Result
	for _, rec := range targets {
		ref, err := resolveInstalledRef(rec)
		if err != nil {
			return nil, err
		}
		if rec.Kind == "cli" {
			res, err := updateCLI(ctx, ref, rec, dryRun)
			if err != nil {
				return nil, err
			}
			results = append(results, res)
			continue
		}
		// bundle/hooks 的 Install 本身即覆盖式更新：文件按 ID 覆盖（安装前先 RemoveAll
		// 旧目标）、状态记录按 name+scope Upsert、hooks 合并前先清除旧 work 托管条目。
		// 因此"更新" == 重新 Install。切不可再对同一 name/scope 调 Uninstall——那会把
		// 刚装好的资源与记录一并删除（findRecord 命中的正是新写入的记录）。
		// 若安装失败，旧版本仍在，天然具备原子性。
		res, err := Install(ctx, Options{
			Scope:  rec.Scope,
			IDEs:   rec.IDEs,
			DryRun: dryRun,
			Ref:    ref,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

// resolveInstalledRef resolves the package source for an installed record.
// 仅接受受信资源名（内置/Registry）；拒绝本地路径与 git: 引用，防止项目级
// 状态文件被恶意仓库注入任意命令（见 source.ParseTrustedRef）。
func resolveInstalledRef(rec state.BundleRecord) (source.Ref, error) {
	if ref, err := source.ParseTrustedRef(rec.Name); err == nil {
		return ref, nil
	}
	return source.ParseTrustedRef(rec.Ref)
}

func updateCLI(ctx context.Context, ref source.Ref, rec state.BundleRecord, dryRun bool) (Result, error) {
	pkgDir, err := source.Resolve(ref)
	if err != nil {
		return Result{}, err
	}
	manifest, err := installer.ParseDir(pkgDir)
	if err != nil {
		return Result{}, err
	}
	var cmd string
	if manifest.Update != nil {
		cmd, err = installer.ResolveCommand(*manifest.Update)
	} else {
		cmd, err = installer.ResolveCommand(manifest.Install)
	}
	if err != nil {
		return Result{}, err
	}
	if dryRun {
		return Result{Success: true, Name: rec.Name, Kind: "cli", Version: rec.Version, Scope: "user", Commands: []string{cmd}, DryRun: true}, nil
	}
	if err := installer.RunInDir(ctx, pkgDir, cmd); err != nil {
		return Result{}, err
	}
	rec.Version = manifest.Version
	rec.InstallCommand = cmd
	if err := saveStateRecord(rec, "user"); err != nil {
		return Result{
			Success:  true,
			Name:     rec.Name,
			Kind:     "cli",
			Version:  manifest.Version,
			Scope:    "user",
			Commands: []string{cmd},
			Warnings: []string{fmt.Sprintf("CLI 更新成功，但保存状态失败: %v", err)},
		}, nil
	}
	return Result{Success: true, Name: rec.Name, Kind: "cli", Version: manifest.Version, Scope: "user", Commands: []string{cmd}, DryRun: false}, nil
}
