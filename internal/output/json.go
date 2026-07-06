package output

import (
	"encoding/json"
	"io"

	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/hooks"
)

// PrintJSON 将 v 格式化为带缩进的 JSON 写入 w。使用 json.MarshalIndent
// 避免每次调用都创建新的 json.Encoder 实例，减少热路径上的分配。
func PrintJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func PrintInstallJSON(w io.Writer, res engine.Result) error {
	return PrintJSON(w, res)
}

func PrintListJSON(w io.Writer, res engine.ListResult) error {
	return PrintJSON(w, res)
}

func PrintHooksStatusJSON(w io.Writer, st hooks.Status) error {
	return PrintJSON(w, st)
}
