package adapter

import (
	"context"
	"os"
	"path/filepath"

	"github.com/huangchao257/work-cli/internal/bundle"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/state"
)

type claudeAdapter struct{}

func NewClaude() Adapter { return &claudeAdapter{} }

func (c *claudeAdapter) Name() string { return "claude" }

func (c *claudeAdapter) Detect() bool {
	home, err := platform.UserHome()
	if err != nil {
		return false
	}
	if dirExists(filepath.Join(home, ".claude")) {
		return true
	}
	_, err = os.Stat(filepath.Join(home, ".claude.json"))
	return err == nil
}

func (c *claudeAdapter) InstallSkill(ctx context.Context, bundleRoot string, skill bundle.SkillResource, scope Scope) (string, error) {
	dest, err := platform.SkillDir(platform.IDEClaude, string(scope), skill.ID)
	if err != nil {
		return "", err
	}
	return installSkillAt(bundleRoot, skill, dest)
}

func (c *claudeAdapter) InstallRule(ctx context.Context, bundleRoot string, rule bundle.RuleResource, scope Scope) (string, error) {
	base, err := platform.RuleDir(platform.IDEClaude, string(scope))
	if err != nil {
		return "", err
	}
	rulePath := filepath.Join(base, "rules", rule.ID+".md")
	if err := os.MkdirAll(filepath.Dir(rulePath), 0o755); err != nil {
		return "", err
	}
	return installRuleFile(bundleRoot, rule, rulePath, qoderRuleFrontMatter(rule))
}

func (c *claudeAdapter) InstallMCP(ctx context.Context, bundleRoot string, mcp bundle.MCPResource, scope Scope) (string, error) {
	path, err := platform.MCPConfigPath(platform.IDEClaude, string(scope))
	if err != nil {
		return "", err
	}
	return installMCPAt(bundleRoot, mcp, path)
}

func (c *claudeAdapter) Uninstall(ctx context.Context, rec state.BundleRecord, scope Scope) error {
	for _, id := range rec.Resources.Skills {
		dir, err := platform.SkillDir(platform.IDEClaude, string(scope), id)
		if err != nil {
			return err
		}
		_ = os.RemoveAll(dir)
	}
	base, err := platform.RuleDir(platform.IDEClaude, string(scope))
	if err != nil {
		return err
	}
	for _, id := range rec.Resources.Rules {
		_ = os.Remove(filepath.Join(base, "rules", id+".md"))
	}
	if len(rec.Resources.MCP) > 0 {
		path, err := platform.MCPConfigPath(platform.IDEClaude, string(scope))
		if err != nil {
			return err
		}
		data, _ := os.ReadFile(path)
		out := data
		for _, id := range rec.Resources.MCP {
			var err2 error
			out, err2 = RemoveMCPServer(out, id)
			if err2 != nil {
				return err2
			}
		}
		_ = os.WriteFile(path, out, 0o644)
	}
	return nil
}
