// Package output 输出渲染基准测试 — 衡量 JSON 与 human 格式输出的性能。
package output

import (
	"bytes"
	"testing"

	"github.com/huangchao257/work-cli/internal/engine"
)

func makeTestResult() engine.Result {
	return engine.Result{
		Success:       true,
		Name:          "dev-kit",
		Kind:          "bundle",
		Version:       "1.0.0",
		Scope:         "user",
		InstalledIDEs: []string{"cursor", "claude", "qoder"},
		Warnings:      []string{"未检测到 qoder，已跳过"},
		FilesWritten:  []string{"/home/user/.cursor/skills/dev-kit/skill.md", "/home/user/.claude/skills/dev-kit/skill.md"},
		DryRun:        false,
	}
}

func BenchmarkPrintJSON(b *testing.B) {
	res := makeTestResult()
	buf := &bytes.Buffer{}

	b.ResetTimer()
	for range b.N {
		buf.Reset()
		if err := PrintJSON(buf, res); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrintInstallJSON(b *testing.B) {
	res := makeTestResult()
	buf := &bytes.Buffer{}

	b.ResetTimer()
	for range b.N {
		buf.Reset()
		if err := PrintInstallJSON(buf, res); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrintHumanInstall(b *testing.B) {
	res := makeTestResult()
	buf := &bytes.Buffer{}

	b.ResetTimer()
	for range b.N {
		buf.Reset()
		if err := PrintHuman(buf, res); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrintHumanUninstall(b *testing.B) {
	res := makeTestResult()
	res.Kind = "bundle"
	buf := &bytes.Buffer{}

	b.ResetTimer()
	for range b.N {
		buf.Reset()
		if err := PrintHumanUninstall(buf, res); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrintJSONLargeResult(b *testing.B) {
	// 模拟大量文件写入的场景（如大型 bundle）
	ideList := []string{}
	filesList := []string{}
	warningsList := []string{}
	for i := range 100 {
		ideList = append(ideList, "cursor")
		filesList = append(filesList, ".cursor/skills/skill-"+string([]byte{byte('a' + i%26)})+"/file.md")
		warningsList = append(warningsList, "这是第"+string([]byte{byte('0' + i%10)})+"条警告信息")
	}
	res := engine.Result{
		Success:       true,
		Name:          "large-bundle",
		Kind:          "bundle",
		Version:       "1.0.0",
		Scope:         "user",
		InstalledIDEs: ideList,
		Warnings:      warningsList,
		FilesWritten:  filesList,
		DryRun:        false,
	}
	buf := &bytes.Buffer{}

	b.ResetTimer()
	for range b.N {
		buf.Reset()
		if err := PrintJSON(buf, res); err != nil {
			b.Fatal(err)
		}
	}
}
