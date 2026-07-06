// Package engine 提供共享的状态记录保存辅助。
// 消除 bundle.go / cli_install.go / hooks.go 中重复的 statePath + store.Open + Upsert 模式。

package engine

import (
	"fmt"

	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/state"
)

// saveStateRecord 将安装/更新记录写入状态文件。返回写入前已打开的 Store（调用方无需再关闭）。
// scope 用于确定状态文件路径（user 或 project）。
func saveStateRecord(rec state.BundleRecord, scope string) error {
	statePath, err := platform.WorkStatePath(scope)
	if err != nil {
		return fmt.Errorf("定位状态文件路径失败: %w", err)
	}
	store, err := state.Open(statePath)
	if err != nil {
		return fmt.Errorf("打开状态文件失败: %w", err)
	}
	if err := store.Upsert(rec); err != nil {
		return fmt.Errorf("写入安装记录失败: %w", err)
	}
	return nil
}
