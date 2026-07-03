package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PrintStatus 输出 CodeGraph 与 AGENTS 自动同步状态。
func PrintStatus(ctx context.Context, opts Options, w ioWriter) error {
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
	if st.SkillInstalled {
		fmt.Fprintln(w, "技能包: codegraph-agents 已安装")
	} else {
		fmt.Fprintln(w, "技能包: 未安装（运行 work install codegraph-kit --scope project）")
	}
	if st.AgentsHook {
		fmt.Fprintln(w, "AGENTS 自动同步: 已开启（保存代码后约 2s 更新）")
	} else {
		fmt.Fprintln(w, "AGENTS 自动同步: 未开启（运行 work graph init）")
	}
	if st.AgentsLog != "" {
		fmt.Fprintf(w, "最近同步日志: %s\n", st.AgentsLog)
	}
	return nil
}

// collectStatus 收集 CodeGraph 与 AGENTS 同步状态。
func collectStatus(ctx context.Context, root string) (Status, error) {
	st := Status{ProjectPath: root}
	st.SkillInstalled = skillInstalled(root)
	st.AgentsHook = hookConfigured(root)
	logPath := filepath.Join(root, ".codegraph", "agents-sync", "sync.log")
	if data, err := os.ReadFile(logPath); err == nil && len(data) > 0 {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) > 0 {
			st.AgentsLog = lines[len(lines)-1]
		}
	}
	if _, err := exec.LookPath("codegraph"); err != nil {
		return st, nil
	}
	cmd := exec.CommandContext(ctx, "codegraph", "status", "--json", "-p", root)
	out, err := cmd.Output()
	if err == nil {
		st.Codegraph = json.RawMessage(out)
	}
	return st, nil
}

func skillInstalled(projectRoot string) bool {
	_, err := findScript(projectRoot, "generate-agents.sh")
	return err == nil
}
