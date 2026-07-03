// Package adapter 提供各 IDE（Cursor/Qoder/Claude Code）资源文件的写入适配层。

package adapter

import (
	"context"

	"github.com/huangchao257/work-cli/internal/bundle"
	"github.com/huangchao257/work-cli/internal/state"
)

type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

type Adapter interface {
	Name() string
	Detect() bool
	InstallSkill(ctx context.Context, bundleRoot string, skill bundle.SkillResource, scope Scope) (string, error)
	InstallRule(ctx context.Context, bundleRoot string, rule bundle.RuleResource, scope Scope) (string, error)
	InstallMCP(ctx context.Context, bundleRoot string, mcp bundle.MCPResource, scope Scope) (string, error)
	Uninstall(ctx context.Context, rec state.BundleRecord, scope Scope) error
}

func All() []Adapter {
	return []Adapter{
		NewCursor(),
		NewQoder(),
		NewClaude(),
	}
}

func ByName(name string) (Adapter, bool) {
	for _, a := range All() {
		if a.Name() == name {
			return a, true
		}
	}
	return nil, false
}
