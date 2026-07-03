package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const skillID = "codegraph-agents"

// Options for graph commands.
type Options struct {
	ProjectPath string
	Quiet       bool
	DryRun      bool
}

// Status summarizes CodeGraph and AGENTS auto-sync state.
type Status struct {
	ProjectPath    string          `json:"projectPath"`
	Codegraph      json.RawMessage `json:"codegraph,omitempty"`
	AgentsHook     bool            `json:"agentsHook"`
	AgentsLog      string          `json:"agentsLog,omitempty"`
	SkillInstalled bool            `json:"skillInstalled"`
}

func Init(ctx context.Context, opts Options) error {
	root, err := resolveRoot(opts.ProjectPath)
	if err != nil {
		return fmt.Errorf("解析项目根目录失败: %w", err)
	}
	if opts.DryRun {
		fmt.Printf("（预览）将在 %s 执行 graph init\n", root)
		fmt.Println("  - codegraph init -i")
		fmt.Println("  - 配置 .cursor/hooks.json 自动同步")
		fmt.Println("  - 生成各目录 AGENTS.md")
		return nil
	}
	if err := ensureCodegraph(root, true, opts.Quiet); err != nil {
		return err
	}
	script, err := findScript(root, "on-file-edit.sh")
	if err != nil {
		return fmt.Errorf("未找到 codegraph-agents 技能，请先执行: work install codegraph-kit --scope project\n%w", err)
	}
	if err := setupCursorHook(root, script); err != nil {
		return fmt.Errorf("配置 Cursor hooks 失败: %w", err)
	}
	gen, err := findScript(root, "generate-agents.sh")
	if err != nil {
		return fmt.Errorf("未找到 generate-agents.sh 脚本: %w", err)
	}
	if !opts.Quiet {
		fmt.Println("正在生成 AGENTS.md ...")
	}
	args := []string{"--skip-sync", "-p", root}
	if opts.Quiet {
		args = append([]string{"--quiet"}, args...)
	}
	if err := runBash(ctx, root, gen, args...); err != nil {
		return fmt.Errorf("生成 AGENTS.md 失败: %w", err)
	}
	if !opts.Quiet {
		fmt.Println("✓ 知识图谱与 AGENTS.md 已就绪；保存代码后将自动更新（约 2 秒）")
	}
	return nil
}

func Sync(ctx context.Context, opts Options) error {
	root, err := resolveRoot(opts.ProjectPath)
	if err != nil {
		return fmt.Errorf("解析项目根目录失败: %w", err)
	}
	if opts.DryRun {
		fmt.Printf("（预览）将在 %s 执行 graph sync\n", root)
		return nil
	}
	if err := ensureCodegraph(root, false, opts.Quiet); err != nil {
		return err
	}
	gen, err := findScript(root, "generate-agents.sh")
	if err != nil {
		return fmt.Errorf("未找到 generate-agents.sh 脚本: %w", err)
	}
	args := []string{"--quiet", "--skip-sync", "-p", root}
	return runBash(ctx, root, gen, args...)
}

func PrintStatus(ctx context.Context, opts Options, w ioWriter) error {
	root, err := resolveRoot(opts.ProjectPath)
	if err != nil {
		return fmt.Errorf("解析项目根目录失败: %w", err)
	}
	st, err := CollectStatus(ctx, root)
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

func CollectStatus(ctx context.Context, root string) (Status, error) {
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

type ioWriter interface {
	Write([]byte) (int, error)
}

func resolveRoot(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return cwd, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func ensureCodegraph(root string, init bool, quiet bool) error {
	if _, err := exec.LookPath("codegraph"); err != nil {
		return fmt.Errorf("未找到 codegraph，请先执行: work install codegraph-stack")
	}
	status, _ := codegraphStatus(root)
	initialized := false
	if status != nil {
		if v, ok := status["initialized"].(bool); ok {
			initialized = v
		}
	}
	if !initialized && init {
		if !quiet {
			fmt.Println("正在初始化 CodeGraph 索引...")
		}
		cmd := exec.Command("codegraph", "init", "-i", "-p", root)
		cmd.Dir = root
		if quiet {
			cmd.Stdout = nil
			cmd.Stderr = nil
		} else {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("codegraph init 失败: %w", err)
		}
		return nil
	}
	cmd := exec.Command("codegraph", "sync", "-p", root)
	cmd.Dir = root
	// sync 是幂等的预同步，失败时不应阻断后续流程；记录到 stderr 供排查
	if err := cmd.Run(); err != nil && !quiet {
		fmt.Fprintf(os.Stderr, "警告: codegraph sync 预同步失败（已忽略）: %v\n", err)
	}
	return nil
}

func codegraphStatus(root string) (map[string]any, error) {
	cmd := exec.Command("codegraph", "status", "--json", "-p", root)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func findScript(projectRoot, name string) (string, error) {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(projectRoot, ".cursor", "skills", skillID, "scripts", name),
		filepath.Join(home, ".cursor", "skills", skillID, "scripts", name),
		filepath.Join(home, ".claude", "skills", skillID, "scripts", name),
		filepath.Join(home, ".qoder", "skills", skillID, "scripts", name),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("未找到脚本 %s", name)
}

func skillInstalled(projectRoot string) bool {
	_, err := findScript(projectRoot, "generate-agents.sh")
	return err == nil
}

func hookConfigured(projectRoot string) bool {
	data, err := os.ReadFile(filepath.Join(projectRoot, ".cursor", "hooks.json"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "codegraph-agents") || strings.Contains(string(data), "on-file-edit.sh")
}

type cursorHooksFile struct {
	Version int                          `json:"version"`
	Hooks   map[string][]cursorHookEntry `json:"hooks"`
}

type cursorHookEntry struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

func setupCursorHook(projectRoot, hookScript string) error {
	hooksPath := filepath.Join(projectRoot, ".cursor", "hooks.json")
	marker := "codegraph-agents/on-file-edit.sh"

	var cfg cursorHooksFile
	if data, err := os.ReadFile(hooksPath); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("解析 hooks.json 失败: %w", err)
		}
	} else {
		cfg = cursorHooksFile{Version: 1, Hooks: map[string][]cursorHookEntry{}}
	}
	if cfg.Hooks == nil {
		cfg.Hooks = map[string][]cursorHookEntry{}
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}

	filtered := make([]cursorHookEntry, 0)
	for _, e := range cfg.Hooks["afterFileEdit"] {
		if strings.Contains(e.Command, marker) || strings.Contains(e.Command, "on-file-edit.sh") {
			continue
		}
		filtered = append(filtered, e)
	}
	filtered = append(filtered, cursorHookEntry{Command: hookScript, Timeout: 15})
	cfg.Hooks["afterFileEdit"] = filtered

	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		return fmt.Errorf("创建 hooks 目录失败: %w", err)
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 hooks.json 失败: %w", err)
	}
	if err := os.WriteFile(hooksPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入 hooks.json 失败: %w", err)
	}
	return nil
}

func runBash(ctx context.Context, dir, script string, args ...string) error {
	cmd := exec.CommandContext(ctx, "bash", append([]string{script}, args...)...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行 %s 失败: %w", filepath.Base(script), err)
	}
	return nil
}

// RunPostInstall is called after bundle install when post_install is configured.
func RunPostInstall(ctx context.Context, scope string, dryRun bool) error {
	if scope != "project" || dryRun {
		return nil
	}
	time.Sleep(200 * time.Millisecond)
	return Init(ctx, Options{Quiet: true})
}
