package adapter

import (
	"github.com/huangchao257/work-cli/internal/platform"
)

// NewCursor 创建 Cursor IDE 适配器。
// Cursor 使用 YAML front-matter 格式的 rule 元数据，rule 文件扩展名为 .mdc。
func NewCursor() Adapter {
	return &baseAdapter{
		ide:           platform.IDECursor,
		name:          "cursor",
		detectFn:      platform.DetectCursor,
		ruleFormatter: cursorRuleFrontMatter,
		rulePathFn:    genericRulePath(platform.IDECursor),
	}
}
