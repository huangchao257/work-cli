package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/huangchao257/work-cli/internal/bundle"
	"github.com/huangchao257/work-cli/internal/pkg/copyutil"
	"github.com/huangchao257/work-cli/internal/platform"
)

func installSkillAt(bundleRoot string, skill bundle.SkillResource, dest string) (string, error) {
	src := filepath.Join(bundleRoot, filepath.FromSlash(strings.TrimPrefix(skill.Source, "./")))
	if err := validateSourceInBundle(bundleRoot, src); err != nil {
		return "", err
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := copyutil.CopyDir(src, dest); err != nil {
		return "", fmt.Errorf("复制 skill %s 失败: %w", skill.ID, err)
	}
	return dest, nil
}

func installRuleFile(bundleRoot string, rule bundle.RuleResource, dest string, frontMatter string) (string, error) {
	src := filepath.Join(bundleRoot, filepath.FromSlash(strings.TrimPrefix(rule.Source, "./")))
	if err := validateSourceInBundle(bundleRoot, src); err != nil {
		return "", err
	}
	content, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	buf.WriteString(frontMatter)
	buf.Write(content)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".rule-*.md")
	if err != nil {
		return "", fmt.Errorf("创建临时规则文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	data := buf.Bytes()
	if _, err := tmp.Write(data); err != nil {
		return "", fmt.Errorf("写入临时规则文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("关闭临时规则文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return "", fmt.Errorf("原子替换规则文件失败: %w", err)
	}
	cleanup = false
	return dest, nil
}

func installMCPAt(bundleRoot string, mcp bundle.MCPResource, configPath string) (string, error) {
	src := filepath.Join(bundleRoot, filepath.FromSlash(strings.TrimPrefix(mcp.Source, "./")))
	if err := validateSourceInBundle(bundleRoot, src); err != nil {
		return "", err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	var server json.RawMessage
	if err := json.Unmarshal(data, &server); err != nil {
		return "", fmt.Errorf("解析 MCP %s 失败: %w", mcp.ID, err)
	}
	// 使用文件锁防止多个 work 进程同时修改同一 MCP 配置文件导致数据损坏
	// withMCPLock 内部完成 read-merge-write，全程持有锁
	if err := withMCPLock(configPath, func(existing []byte) ([]byte, error) {
		return MergeMCPServers(existing, mcp.ID, server)
	}); err != nil {
		return "", err
	}
	return configPath, nil
}

func validateSourceInBundle(bundleRoot, src string) error {
	cleaned := filepath.Clean(src)
	root := filepath.Clean(bundleRoot)
	sep := string(os.PathSeparator)
	if !strings.HasPrefix(cleaned, root+sep) && cleaned != root {
		return fmt.Errorf("源路径 %q 试图逃逸 bundle 目录", src)
	}
	return nil
}

func cursorRuleFrontMatter(rule bundle.RuleResource) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: ")
	b.WriteString(rule.ID)
	b.WriteString("\n")
	switch rule.Apply {
	case "always":
		b.WriteString("alwaysApply: true\n")
	case "manual":
		b.WriteString("alwaysApply: false\n")
	case "files":
		b.WriteString("globs:\n")
		for _, g := range rule.Globs {
			b.WriteString("  - ")
			b.WriteString(g)
			b.WriteString("\n")
		}
	}
	b.WriteString("---\n\n")
	return b.String()
}

func qoderRuleFrontMatter(rule bundle.RuleResource) string {
	var b strings.Builder
	b.WriteString("<!-- qoder-rule ")
	b.WriteString(rule.ID)
	b.WriteString(" apply=")
	b.WriteString(rule.Apply)
	if len(rule.Globs) > 0 {
		b.WriteString(" globs=")
		b.WriteString(strings.Join(rule.Globs, ","))
	}
	b.WriteString(" -->\n\n")
	return b.String()
}

// withMCPLock 对指定路径的 MCP 配置文件加独占锁，读取、合并并写入内容。
// 全程持有锁，防止多个 work 进程同时修改同一 MCP 配置文件导致数据损坏。
func withMCPLock(configPath string, fn func(existing []byte) ([]byte, error)) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("创建 MCP 配置目录失败: %w", err)
	}
	f, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("打开 MCP 配置文件失败: %w", err)
	}
	defer f.Close()

	if err := platform.FlockLock(f, configPath, platform.FlockEX); err != nil {
		return fmt.Errorf("获取 MCP 配置文件独占锁失败: %w", err)
	}
	defer func() { _ = platform.FlockUnlock(f) }()

	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取 MCP 配置文件失败: %w", err)
	}

	merged, err := fn(existing)
	if err != nil {
		return err
	}

	// 原子写入：先写临时文件，再 rename，避免 truncate 中途崩溃导致文件为空
	tmp, werr := os.CreateTemp(filepath.Dir(configPath), ".mcp-*.json")
	if werr != nil {
		return fmt.Errorf("创建临时 MCP 配置文件失败: %w", werr)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, werr := tmp.Write(merged); werr != nil {
		return fmt.Errorf("写入临时 MCP 配置文件失败: %w", werr)
	}
	if werr := tmp.Close(); werr != nil {
		return fmt.Errorf("关闭临时 MCP 配置文件失败: %w", werr)
	}
	if werr := os.Rename(tmpPath, configPath); werr != nil {
		return fmt.Errorf("原子替换 MCP 配置文件失败: %w", werr)
	}
	cleanup = false
	return nil
}
