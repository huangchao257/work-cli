package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/huangchao257/work-cli/internal/engine"
)

func PrintHuman(w io.Writer, res engine.Result) error {
	if res.DryRun {
		fmt.Fprintf(w, "（预览模式，未实际执行）\n")
	}
	for _, warn := range res.Warnings {
		fmt.Fprintf(w, "⚠ %s\n", warn)
	}
	if res.Kind == "cli" {
		if len(res.Commands) > 0 {
			if res.DryRun {
				fmt.Fprintf(w, "将执行：%s\n", res.Commands[0])
			} else {
				fmt.Fprintf(w, "✓ 已安装 %s v%s（已执行：%s）\n", res.Name, res.Version, res.Commands[0])
			}
		}
		return nil
	}
	if res.Success {
		ides := strings.Join(res.InstalledIDEs, ", ")
		if res.DryRun {
			fmt.Fprintf(w, "将写入 %d 个路径\n", len(res.FilesWritten))
			for _, f := range res.FilesWritten {
				fmt.Fprintf(w, "  - %s\n", f)
			}
			return nil
		}
		fmt.Fprintf(w, "✓ 已安装 %s v%s → %s（范围：%s）\n", res.Name, res.Version, ides, scopeLabel(res.Scope))
	}
	return nil
}

func PrintHumanList(w io.Writer, res engine.ListResult) error {
	if len(res.Items) == 0 {
		fmt.Fprintln(w, "暂无已安装项")
		return nil
	}
	for _, item := range res.Items {
		if item.Kind == "cli" {
			fmt.Fprintf(w, "- %s v%s [%s] scope=%s\n", item.Name, item.Version, item.Kind, item.Scope)
			if item.InstallCommand != "" {
				fmt.Fprintf(w, "    命令: %s\n", item.InstallCommand)
			}
			continue
		}
		fmt.Fprintf(w, "- %s v%s [%s] scope=%s ides=%s\n", item.Name, item.Version, item.Kind, item.Scope, strings.Join(item.IDEs, ","))
	}
	return nil
}

func scopeLabel(scope string) string {
	if scope == "project" {
		return "项目级"
	}
	return "用户级"
}
