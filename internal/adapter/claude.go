package adapter

import (
	"path/filepath"

	"github.com/huangchao257/work-cli/internal/platform"
)

// NewClaude 创建 Claude Code IDE 适配器。
// Claude Code 的 rules 位于 <ideBase>/rules/<id>.md，使用 Qoder 风格的 HTML 注释元数据。
func NewClaude() Adapter {
	return &baseAdapter{
		ide:           platform.IDEClaude,
		name:          "claude",
		detectFn:      platform.DetectClaude,
		ruleFormatter: qoderRuleFrontMatter,
		rulePathFn:    claudeRulePath,
	}
}

// claudeRulePath 返回 Claude Code rule 文件的完整路径（<base>/rules/<id>.md）。
// 与 Qoder/Cursor 不同，Claude Code 的 rules 目录在 ideBase 下直接嵌套一层 rules/。
func claudeRulePath(scope, ruleID string) (string, error) {
	base, err := platform.RuleDir(platform.IDEClaude, scope)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "rules", ruleID+".md"), nil
}
