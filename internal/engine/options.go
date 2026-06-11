package engine

import "github.com/huangchao257/work-cli/internal/source"

type Options struct {
	Scope  string
	IDEs   []string
	DryRun bool
	Ref    source.Ref
}
