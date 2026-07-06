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

	"github.com/huangchao257/work-cli/internal/log"
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

type ioWriter interface {
	Write([]byte) (int, error)
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
		log.Warnf("[work graph]", "codegraph sync 预同步失败（已忽略）: %v", err)
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
