package adapter

import (
	"context"
	"fmt"
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
			return fmt.Errorf("定位 skill 目录失败: %w", err)
		}
		// 清理 skill 目录：失败不阻断卸载，残留文件可下次清理
		_ = os.RemoveAll(dir)
	}
	for _, id := range rec.Resources.Rules {
		path, err := platform.RuleFile(platform.IDECursor, string(scope), id)
		if err != nil {
			return fmt.Errorf("定位 rule 文件失败: %w", err)
		}
		// 移除 rule 文件：失败不阻断卸载
		_ = os.Remove(path)
	}
	if len(rec.Resources.MCP) > 0 {
		path, err := platform.MCPConfigPath(platform.IDECursor, string(scope))
		if err != nil {
			return fmt.Errorf("定位 MCP 配置失败: %w", err)
		}
		data, _ := os.ReadFile(path)
		out := data
		var err2 error
		for _, id := range rec.Resources.MCP {
			out, err2 = RemoveMCPServer(out, id)
			if err2 != nil {
				return fmt.Errorf("移除 MCP server %s 失败: %w", id, err2)
			}
		}
		// 写回 MCP 配置：失败不阻断卸载，配置可能已不存在
		_ = os.WriteFile(path, out, 0o644)
	}
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
