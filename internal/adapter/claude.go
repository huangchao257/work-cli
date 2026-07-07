package adapter

import (
	"os"
	"path/filepath"

	"github.com/huangchao257/work-cli/internal/platform"
)

// NewClaude 创建 Claude Code IDE 适配器。
// Claude Code 的 rules 位于 <ideBase>/rules/<id>.md，使用 Qoder 风格的 HTML 注释元数据。
func NewClaude() Adapter {
	return &baseAdapter{
		ide:           platform.IDEClaude,
		name:          "claude",
		detectFn:      detectClaude,
		ruleFormatter: qoderRuleFrontMatter,
		rulePathFn:    claudeRulePath,
	}
}

// detectClaude 检测当前系统是否安装了 Claude Code。
// 同时检查 ~/.claude/（标准安装路径）和 XDG_CONFIG_HOME/claude/。
func detectClaude() bool {
	home, err := platform.UserHome()
	if err != nil {
		return false
	}
	// 标准安装路径
	if dirExists(filepath.Join(home, ".claude")) {
		return true
	}
	// 也可能以文件形式存在（~/.claude.json）
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); err == nil {
		return true
	}
	// XDG 配置路径
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	if dirExists(filepath.Join(configHome, "claude")) {
		return true
	}
	return false
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
