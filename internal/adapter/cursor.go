package adapter

import (
	"context"
	"os"
	"path/filepath"

	"github.com/huangchao257/work-cli/internal/bundle"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/state"
)

type cursorAdapter struct{}

func NewCursor() Adapter { return &cursorAdapter{} }

func (c *cursorAdapter) Name() string { return "cursor" }

func (c *cursorAdapter) Detect() bool {
	home, err := platform.UserHome()
	if err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(home, ".cursor")); err == nil {
		return true
	}
	return dirExists(filepath.Join(home, ".cursor"))
}

func (c *cursorAdapter) InstallSkill(ctx context.Context, bundleRoot string, skill bundle.SkillResource, scope Scope) (string, error) {
	dest, err := platform.SkillDir(platform.IDECursor, string(scope), skill.ID)
	if err != nil {
		return "", err
	}
	return installSkillAt(bundleRoot, skill, dest)
}

func (c *cursorAdapter) InstallRule(ctx context.Context, bundleRoot string, rule bundle.RuleResource, scope Scope) (string, error) {
	dest, err := platform.RuleFile(platform.IDECursor, string(scope), rule.ID)
	if err != nil {
		return "", err
	}
	return installRuleFile(bundleRoot, rule, dest, cursorRuleFrontMatter(rule))
}

func (c *cursorAdapter) InstallMCP(ctx context.Context, bundleRoot string, mcp bundle.MCPResource, scope Scope) (string, error) {
	path, err := platform.MCPConfigPath(platform.IDECursor, string(scope))
	if err != nil {
		return "", err
	}
	return installMCPAt(bundleRoot, mcp, path)
}

func (c *cursorAdapter) Uninstall(ctx context.Context, rec state.BundleRecord, scope Scope) error {
	for _, id := range rec.Resources.Skills {
		dir, err := platform.SkillDir(platform.IDECursor, string(scope), id)
		if err != nil {
			return err
		}
		_ = os.RemoveAll(dir)
	}
	for _, id := range rec.Resources.Rules {
		path, err := platform.RuleFile(platform.IDECursor, string(scope), id)
		if err != nil {
			return err
		}
		_ = os.Remove(path)
	}
	if len(rec.Resources.MCP) > 0 {
		path, err := platform.MCPConfigPath(platform.IDECursor, string(scope))
		if err != nil {
			return err
		}
		data, _ := os.ReadFile(path)
		out := data
		var err2 error
		for _, id := range rec.Resources.MCP {
			out, err2 = RemoveMCPServer(out, id)
			if err2 != nil {
				return err2
			}
		}
		_ = os.WriteFile(path, out, 0o644)
	}
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
