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

	switch rec.Kind {
	case "hooks":
		if !dryRun {
			if err := uninstallHooks(ctx, rec, false); err != nil {
				return Result{}, err
			}
		}
	case "cli":
		ref, err := source.ParseRef(rec.Ref)
		if err == nil {
			pkgDir, err := source.Resolve(ref)
			if err == nil {
				manifest, err := installer.ParseDir(pkgDir)
				if err == nil && manifest.Uninstall != nil {
					cmd, err := installer.ResolveCommand(*manifest.Uninstall)
					if err == nil {
						commands = append(commands, cmd)
						if !dryRun {
							if err := runInDir(ctx, pkgDir, cmd); err != nil {
								return Result{}, err
							}
						}
					}
				}
			}
		}
		if len(commands) == 0 {
			warnings = append(warnings, fmt.Sprintf("未找到 %s 的卸载命令，请手动卸载（例如 npm uninstall -g @fission-ai/openspec）", name))
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
					return Result{}, err
				}
			}
		}
	}

	if !dryRun {
		if err := store.Remove(rec.Name, rec.Scope); err != nil {
			return Result{}, err
		}
	}

	return Result{
		Success:  true,
		Name:     rec.Name,
		Kind:     rec.Kind,
		Version:  rec.Version,
		Scope:    rec.Scope,
		Commands: commands,
		Warnings: warnings,
		DryRun:   dryRun,
	}, nil
}

func findRecord(name, scope string) (*state.BundleRecord, *state.Store, error) {
	statePath, err := platform.WorkStatePath(scope)
	if err != nil {
		return nil, nil, err
	}
	store, err := state.Open(statePath)
	if err != nil {
		return nil, nil, err
	}
	rec, err := store.Find(name, scope)
	if err == nil {
		return rec, store, nil
	}
	if scope != "user" {
		statePath, err = platform.WorkStatePath("user")
		if err != nil {
			return nil, nil, err
		}
		store, err = state.Open(statePath)
		if err != nil {
			return nil, nil, err
		}
		rec, err = store.Find(name, "user")
	}
	if err != nil {
		return nil, nil, err
	}
	return rec, store, nil
}
