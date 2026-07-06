package adapter

import (
	"path/filepath"

	"github.com/huangchao257/work-cli/internal/platform"
)

// NewQoder 创建 Qoder IDE 适配器。
// Qoder 使用 HTML 注释格式的 rule 元数据（<!-- qoder-rule ... -->），
// rule 文件扩展名为 .md。
func NewQoder() Adapter {
	return &baseAdapter{
		ide:           platform.IDEQoder,
		name:          "qoder",
		detectFn:      detectQoder,
		ruleFormatter: qoderRuleFrontMatter,
		rulePathFn:    genericRulePath(platform.IDEQoder),
	}
}

// detectQoder 检测当前系统是否安装了 Qoder IDE。
func detectQoder() bool {
	home, err := platform.UserHome()
	if err != nil {
		return false
	}
	return dirExists(filepath.Join(home, ".qoder"))
}
