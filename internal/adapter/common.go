package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/huangchao257/work-cli/internal/bundle"
	"github.com/huangchao257/work-cli/internal/pkg/copyutil"
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
	merged, err := withMCPLock(configPath, func(existing []byte) ([]byte, error) {
		return MergeMCPServers(existing, mcp.ID, server)
	})
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, merged, 0o644); err != nil {
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

// withMCPLock 对指定路径的 MCP 配置文件加独占锁，读取并调用 fn 合并内容，
// 返回合并结果。用于防止多个 work 进程同时修改同一 MCP 配置文件导致数据损坏。
func withMCPLock(configPath string, fn func(existing []byte) ([]byte, error)) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return nil, fmt.Errorf("创建 MCP 配置目录失败: %w", err)
	}
	f, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开 MCP 配置文件失败: %w", err)
	}
	defer f.Close()

	deadline := time.Now().Add(5 * time.Second)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("获取 MCP 配置文件独占锁失败: %w", err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("获取 MCP 配置文件独占锁超时，可能有其他 work 进程正在操作")
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取 MCP 配置文件失败: %w", err)
	}

	return fn(existing)
}
