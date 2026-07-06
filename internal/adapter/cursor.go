package adapter

import (
	"os"
	"path/filepath"

	"github.com/huangchao257/work-cli/internal/platform"
)

// NewCursor 创建 Cursor IDE 适配器。
// Cursor 使用 YAML front-matter 格式的 rule 元数据，rule 文件扩展名为 .mdc。
func NewCursor() Adapter {
	return &baseAdapter{
		ide:           platform.IDECursor,
		name:          "cursor",
		detectFn:      detectCursor,
		ruleFormatter: cursorRuleFrontMatter,
		rulePathFn:    genericRulePath(platform.IDECursor),
	}
}

// detectCursor 检测当前系统是否安装了 Cursor IDE。
func detectCursor() bool {
	home, err := platform.UserHome()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, ".cursor"))
	return err == nil
}
