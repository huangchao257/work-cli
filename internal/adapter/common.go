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
	if err := os.WriteFile(dest, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func installMCPAt(bundleRoot string, mcp bundle.MCPResource, configPath string) (string, error) {
	src := filepath.Join(bundleRoot, filepath.FromSlash(strings.TrimPrefix(mcp.Source, "./")))
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

	// 写入在锁内执行，保证整个 read-modify-write 是原子的
	if err := os.WriteFile(configPath, merged, 0o644); err != nil {
		return fmt.Errorf("写入 MCP 配置文件失败: %w", err)
	}
	return nil
}
