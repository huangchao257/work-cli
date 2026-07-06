// Package output 提供 human（默认）与 --json 两种输出渲染器。

package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/huangchao257/work-cli/internal/engine"
	"github.com/huangchao257/work-cli/internal/hooks"
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

func PrintHumanUninstall(w io.Writer, res engine.Result) error {
	if res.DryRun {
		fmt.Fprintf(w, "（预览模式，未实际执行）\n")
	}
	for _, warn := range res.Warnings {
		fmt.Fprintf(w, "⚠ %s\n", warn)
	}
	if res.Kind == "cli" {
		if len(res.Commands) > 0 {
			if res.DryRun {
				fmt.Fprintf(w, "将执行卸载命令：%s\n", res.Commands[0])
			} else {
				fmt.Fprintf(w, "✓ 已卸载 %s（已执行：%s）\n", res.Name, res.Commands[0])
			}
		} else {
			fmt.Fprintf(w, "✓ 已卸载 %s\n", res.Name)
		}
		return nil
	}
	if res.Success {
		ides := strings.Join(res.InstalledIDEs, ", ")
		if ides != "" {
			fmt.Fprintf(w, "✓ 已卸载 %s（范围：%s，目标 IDE：%s）\n", res.Name, scopeLabel(res.Scope), ides)
		} else {
			fmt.Fprintf(w, "✓ 已卸载 %s（范围：%s）\n", res.Name, scopeLabel(res.Scope))
		}
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

func PrintHooksStatusHuman(w io.Writer, st hooks.Status) error {
	syncAge := "从未同步"
	if st.LastSync != nil {
		syncAge = formatAge(time.Since(*st.LastSync)) + "前"
	}
	url := st.TelemetryURL
	if url == "" {
		url = "（未配置）"
	}
	on := "开启"
	if !st.TelemetryOn {
		on = "关闭"
	}
	fmt.Fprintf(w, "待上报 %d 条 · 上次同步 %s · telemetry %s\n", st.PendingCount, syncAge, on)
	fmt.Fprintf(w, "上报地址: %s\n", url)
	if st.LastError != "" {
		fmt.Fprintf(w, "上次错误: %s\n", st.LastError)
	}
	return nil
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return "不到 1 分钟"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d 分钟", int(d.Minutes()))
	}
	return fmt.Sprintf("%d 小时", int(d.Hours()))
}
