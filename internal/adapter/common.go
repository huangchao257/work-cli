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
	existing, _ := os.ReadFile(configPath)
	merged, err := MergeMCPServers(existing, mcp.ID, server)
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
