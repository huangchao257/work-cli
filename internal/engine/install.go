package engine

import (
	"context"

	pkgmanifest "github.com/huangchao257/work-cli/internal/pkg/manifest"
	"github.com/huangchao257/work-cli/internal/source"
)

func Install(ctx context.Context, opts Options) (Result, error) {
	pkgDir, err := source.Resolve(opts.Ref)
	if err != nil {
		return Result{}, err
	}
	kind, err := pkgmanifest.DetectKind(pkgDir)
	if err != nil {
		return Result{}, err
	}
	switch kind {
	case pkgmanifest.KindCLI:
		return installCLI(ctx, pkgDir, opts, opts.Ref.Raw)
	case pkgmanifest.KindHooks:
		return installHooks(ctx, pkgDir, opts, opts.Ref.Raw)
	default:
		return installBundle(ctx, pkgDir, opts, opts.Ref.Raw)
	}
}
