package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/audit"
	"github.com/huangchao257/work-cli/internal/output"
	"github.com/huangchao257/work-cli/internal/platform"
)

var (
	auditPolicy string
	auditFile   string
	auditSince  string
)

var hooksAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "本地审计 hooks 事件队列",
	Long: `对本地 queue.jsonl 中的 hooks 事件按策略规则匹配，输出违规清单。

本地旁路分析，不阻断 IDE。策略文件默认按「项目根 audit-policy.yaml → ~/.work/audit-policy.yaml」
顺序查找，事件源默认 ~/.work/telemetry/queue.jsonl。`,
	Example: `  work hooks audit
  work hooks audit --policy ./audit-policy.yaml --since 24h
  work hooks audit --file /tmp/queue.jsonl --json`,
	RunE: runHooksAudit,
}

func init() {
	hooksAuditCmd.Flags().StringVar(&auditPolicy, "policy", "", "审计策略文件路径（默认按项目根 → ~/.work/audit-policy.yaml 查找）")
	hooksAuditCmd.Flags().StringVar(&auditFile, "file", "", "事件源 queue.jsonl 路径（默认 ~/.work/telemetry/queue.jsonl）")
	hooksAuditCmd.Flags().StringVar(&auditSince, "since", "", "仅审计近 N 内事件，如 24h、2h30m")
	hooksCmd.AddCommand(hooksAuditCmd)
}

// auditSummary 是 JSON 输出中的汇总结构。
type auditSummary struct {
	Total  int `json:"total"`
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
}

// auditJSONResult 是 --json 的整体输出结构。
type auditJSONResult struct {
	Violations []audit.Violation `json:"violations"`
	Summary    auditSummary      `json:"summary"`
	Warnings   []string          `json:"warnings"`
}

func runHooksAudit(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	// 1. 定位策略文件
	policyPath := auditPolicy
	if policyPath == "" {
		policyPath = findDefaultPolicy()
	}
	if policyPath == "" {
		// 未找到审计策略，跳过并退出 0
		return emitNoPolicy(w)
	}
	if _, err := os.Stat(policyPath); err != nil {
		if os.IsNotExist(err) {
			return emitNoPolicy(w)
		}
		return exitErr(2, fmt.Errorf("无法访问策略文件 %s: %w", policyPath, err))
	}
	policy, err := audit.LoadPolicy(policyPath)
	if err != nil {
		// 策略损坏 → 用法错误，退出 2
		return exitErr(2, fmt.Errorf("审计策略文件损坏: %w", err))
	}
	compiled := policy.Compile()

	// 2. 解析 --since
	var since time.Duration
	if auditSince != "" {
		d, err := time.ParseDuration(auditSince)
		if err != nil {
			return exitErr(2, fmt.Errorf("--since 参数非法: %w", err))
		}
		since = d
	}

	// 3. 定位事件文件
	filePath := auditFile
	if filePath == "" {
		filePath = defaultQueueFile()
	}
	if filePath == "" {
		return emitNoEventsFile(w, "")
	}
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			// 事件文件不存在 → warning 退出 0
			return emitNoEventsFile(w, filePath)
		}
		return exitErr(1, fmt.Errorf("无法访问事件文件 %s: %w", filePath, err))
	}

	now := time.Now()
	events, readWarnings, err := audit.ReadEvents(filePath, now, since)
	if err != nil {
		return exitErr(1, fmt.Errorf("读取事件文件失败: %w", err))
	}

	// 4. 评估违规
	violations := audit.Evaluate(events, compiled)
	summary := buildSummary(violations)
	warnings := readWarnings

	// 5. 输出
	if asJSON {
		return output.PrintJSON(w, auditJSONResult{
			Violations: violations,
			Summary:    summary,
			Warnings:   warnings,
		})
	}

	// human 输出
	for _, msg := range warnings {
		fmt.Fprintf(w, "⚠ %s\n", msg)
	}
	if len(violations) == 0 {
		fmt.Fprintln(w, "✓ 审计通过，无违规")
		return nil
	}
	for _, v := range violations {
		fmt.Fprintf(w, "✗ [%s] %s @ %s — %s\n", v.Severity, v.RuleID, v.EventID, v.Detail)
	}
	fmt.Fprintf(w, "%d 条违规（high %d, medium %d, low %d）\n",
		summary.Total, summary.High, summary.Medium, summary.Low)
	return exitErr(1, errAuditViolations)
}

// errAuditViolations 标记存在违规，触发退出码 1。
var errAuditViolations = fmt.Errorf("审计发现违规")

func buildSummary(vs []audit.Violation) auditSummary {
	s := auditSummary{Total: len(vs)}
	for _, v := range vs {
		switch v.Severity {
		case audit.High:
			s.High++
		case audit.Low:
			s.Low++
		default:
			s.Medium++
		}
	}
	return s
}

// findDefaultPolicy 按项目根 → ~/.work/audit-policy.yaml 顺序查找策略文件。
// 仅返回存在的文件路径，不存在则返回空。
func findDefaultPolicy() string {
	if cwd, err := os.Getwd(); err == nil {
		p := filepath.Join(cwd, "audit-policy.yaml")
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	if dir, err := platform.WorkConfigDir(); err == nil {
		p := filepath.Join(dir, "audit-policy.yaml")
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// defaultQueueFile 返回默认事件源路径 ~/.work/telemetry/queue.jsonl。
func defaultQueueFile() string {
	dir, err := platform.WorkConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "telemetry", "queue.jsonl")
}

// emitNoPolicy 在未找到策略时输出提示并退出 0。
func emitNoPolicy(w io.Writer) error {
	if asJSON {
		return output.PrintJSON(w, auditJSONResult{
			Violations: []audit.Violation{},
			Summary:    auditSummary{},
			Warnings:   []string{"未找到审计策略，跳过"},
		})
	}
	fmt.Fprintln(w, "⚠ 未找到审计策略，跳过")
	return nil
}

// emitNoEventsFile 在事件文件不存在时输出提示并退出 0。
func emitNoEventsFile(w io.Writer, path string) error {
	msg := "事件文件不存在，跳过"
	if path != "" {
		msg = fmt.Sprintf("事件文件不存在：%s，跳过", path)
	}
	if asJSON {
		return output.PrintJSON(w, auditJSONResult{
			Violations: []audit.Violation{},
			Summary:    auditSummary{},
			Warnings:   []string{msg},
		})
	}
	fmt.Fprintf(w, "⚠ %s\n", msg)
	return nil
}
