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
		if !dryRun {
			if _, err := Uninstall(ctx, rec.Name, rec.Scope, false); err != nil {
				return nil, err
			}
		}
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
// Prefer the canonical resource name over legacy stored paths.
func resolveInstalledRef(rec state.BundleRecord) (source.Ref, error) {
	if ref, err := source.ParseInstallName(rec.Name); err == nil {
		if err := source.ValidateInstallName(rec.Name); err == nil {
			return ref, nil
		}
	}
	return source.ParseRef(rec.Ref)
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
	if err := runInDir(ctx, pkgDir, cmd); err != nil {
		return Result{}, err
	}
	rec.Version = manifest.Version
	rec.InstallCommand = cmd
	if err := saveStateRecord(rec, "user"); err != nil {
		return Result{}, err
	}
	return Result{Success: true, Name: rec.Name, Kind: "cli", Version: manifest.Version, Scope: "user", Commands: []string{cmd}, DryRun: false}, nil
}
