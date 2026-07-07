package adapter

import (
	"context"
	"fmt"
	"os"

	"github.com/huangchao257/work-cli/internal/bundle"
	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/state"
)

// RuleFormatter 为 rule 资源生成前置元数据（YAML front-matter 或 HTML 注释），
// 各 IDE 适配器可选择不同的格式化策略。
type RuleFormatter func(rule bundle.RuleResource) string

// baseAdapter 提供 InstallSkill/InstallRule/InstallMCP/Uninstall 的默认实现。
// 各 IDE 适配器通过组合 baseAdapter 并定制 detectFn、ruleFormatter、rulePathFn
// 即可完成适配，无需重复编写样板代码。
type baseAdapter struct {
	ide           platform.IDE
	name          string
	detectFn      func() bool
	ruleFormatter RuleFormatter
	rulePathFn    func(scope, ruleID string) (string, error)
}

func (a *baseAdapter) Name() string { return a.name }

func (a *baseAdapter) Detect() bool { return a.detectFn() }

// InstallSkill 将 bundle 中的 skill 资源复制到 IDE 对应的 skills 目录。
func (a *baseAdapter) InstallSkill(ctx context.Context, bundleRoot string, skill bundle.SkillResource, scope Scope) (string, error) {
	dest, err := platform.SkillDir(a.ide, string(scope), skill.ID)
	if err != nil {
		return "", err
	}
	return installSkillAt(bundleRoot, skill, dest)
}

// InstallRule 将 bundle 中的 rule 资源写入 IDE 对应目录，并追加适配器特定的前置元数据。
func (a *baseAdapter) InstallRule(ctx context.Context, bundleRoot string, rule bundle.RuleResource, scope Scope) (string, error) {
	dest, err := a.rulePathFn(string(scope), rule.ID)
	if err != nil {
		return "", err
	}
	return installRuleFile(bundleRoot, rule, dest, a.ruleFormatter(rule))
}

// InstallMCP 将 bundle 中的 MCP 配置合并到 IDE 的 mcp.json。
func (a *baseAdapter) InstallMCP(ctx context.Context, bundleRoot string, mcp bundle.MCPResource, scope Scope) (string, error) {
	configPath, err := platform.MCPConfigPath(a.ide, string(scope))
	if err != nil {
		return "", err
	}
	return installMCPAt(bundleRoot, mcp, configPath)
}

// Uninstall 清理已安装的 skills/rules/MCP 资源。
// 删除单个文件失败时不阻断后续清理，残留文件可由下次安装覆盖。
func (a *baseAdapter) Uninstall(ctx context.Context, rec state.BundleRecord, scope Scope) error {
	// 清理 skills
	for _, id := range rec.Resources.Skills {
		dir, err := platform.SkillDir(a.ide, string(scope), id)
		if err != nil {
			return fmt.Errorf("定位 skill 目录失败: %w", err)
		}
		_ = os.RemoveAll(dir)
	}

	// 清理 rules：使用适配器的 rulePathFn 确定文件路径
	for _, id := range rec.Resources.Rules {
		path, err := a.rulePathFn(string(scope), id)
		if err != nil {
			return fmt.Errorf("定位 rule 文件失败: %w", err)
		}
		_ = os.Remove(path)
	}

	// 清理 MCP servers
	if len(rec.Resources.MCP) > 0 {
		mcpPath, err := platform.MCPConfigPath(a.ide, string(scope))
		if err != nil {
			return fmt.Errorf("定位 MCP 配置失败: %w", err)
		}
		err = withMCPLock(mcpPath, func(existing []byte) ([]byte, error) {
			out := existing
			for _, id := range rec.Resources.MCP {
				var rmErr error
				out, rmErr = RemoveMCPServer(out, id)
				if rmErr != nil {
					return nil, fmt.Errorf("移除 MCP server %s 失败: %w", id, rmErr)
				}
			}
			return out, nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// genericRulePath 返回 Qoder/Cursor 等通用 IDE 的 rule 文件路径。
// 路径格式为 <ideBase>/rules/<ruleID>.<ext>，其中 ext 由 platform.RuleFile 自动确定。
func genericRulePath(ide platform.IDE) func(scope, ruleID string) (string, error) {
	return func(scope, ruleID string) (string, error) {
		return platform.RuleFile(ide, scope, ruleID)
	}
}
