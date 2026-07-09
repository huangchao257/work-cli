package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

// PrintStatus 输出 CodeGraph 与 AGENTS 自动同步状态。
func PrintStatus(ctx context.Context, opts Options, w io.Writer) error {
	root, err := resolveRoot(opts.ProjectPath)
	if err != nil {
		return fmt.Errorf("解析项目根目录失败: %w", err)
	}
	st, err := collectStatus(ctx, root)
	if err != nil {
		return fmt.Errorf("收集 CodeGraph 状态失败: %w", err)
	}
	if opts.Quiet {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(st)
	}
	fmt.Fprintf(w, "项目: %s\n", st.ProjectPath)
	if len(st.Codegraph) > 0 {
		var m map[string]any
		if json.Unmarshal(st.Codegraph, &m) == nil {
			if init, _ := m["initialized"].(bool); init {
				fmt.Fprintf(w, "CodeGraph: 已索引（%v 文件, %v 符号）\n", m["fileCount"], m["nodeCount"])
			} else {
				fmt.Fprintln(w, "CodeGraph: 未初始化（运行 work graph init）")
			}
		}
	} else if _, err := exec.LookPath("codegraph"); err != nil {
		fmt.Fprintln(w, "CodeGraph: 未安装（运行 work install codegraph-stack）")
	} else {
		fmt.Fprintln(w, "CodeGraph: 无法读取状态")
	}
	if st.Watching {
		fmt.Fprintln(w, "generate-agents.sh: 已安装（可运行 work graph watch 开启文件监控）")
	} else {
		fmt.Fprintln(w, "generate-agents.sh: 未安装（运行 work install codegraph-kit --scope project）")
	}
	return nil
}

// collectStatus 收集 CodeGraph 与 AGENTS 同步状态。
func collectStatus(ctx context.Context, root string) (Status, error) {
	st := Status{ProjectPath: root}
	_, err := findScript(root, "generate-agents.sh")
	st.Watching = err == nil
	if _, err := exec.LookPath("codegraph"); err != nil {
		return st, nil
	}
	cmd := exec.CommandContext(ctx, "codegraph", "status", "--json", root)
	out, err := cmd.Output()
	if err == nil {
		st.Codegraph = json.RawMessage(out)
	}
	return st, nil
}
