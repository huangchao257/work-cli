package engine

import (
	"context"

	"github.com/huangchao257/work-cli/internal/installer"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/source"
	"github.com/huangchao257/work-cli/internal/state"
)

func Update(ctx context.Context, name, scope string, dryRun bool) ([]Result, error) {
	if scope == "" {
		scope = "user"
	}
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
	var targets []state.BundleRecord
	for _, r := range records {
		if name != "" && r.Name != name {
			continue
		}
		targets = append(targets, r)
	}
	if name != "" && len(targets) == 0 {
		return nil, findRecordError(name, scope)
	}

	var results []Result
	for _, rec := range targets {
		ref, err := source.ParseRef(rec.Ref)
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
		res, err := Install(ctx, Options{Scope: rec.Scope, DryRun: dryRun, Ref: ref})
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
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
		return Result{Success: true, Name: rec.Name, Kind: "cli", Commands: []string{cmd}, DryRun: true}, nil
	}
	if err := runInDir(ctx, pkgDir, cmd); err != nil {
		return Result{}, err
	}
	rec.Version = manifest.Version
	rec.InstallCommand = cmd
	statePath, _ := platform.WorkStatePath("user")
	store, _ := state.Open(statePath)
	_ = store.Upsert(rec)
	return Result{Success: true, Name: rec.Name, Kind: "cli", Version: manifest.Version, Commands: []string{cmd}}, nil
}

func findRecordError(name, scope string) error {
	_, _, err := findRecord(name, scope)
	return err
}
