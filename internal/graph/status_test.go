package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// --- collectStatus ---

func TestCollectStatusScriptInstalled(t *testing.T) {
	skipOnWindows(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	writeFakeScript(t, filepath.Join(root, ".claude", "skills", skillID, "scripts", "generate-agents.sh"),
		"echo ok")

	st, err := collectStatus(context.Background(), root)
	if err != nil {
		t.Fatalf("collectStatus 报错: %v", err)
	}
	if !st.Watching {
		t.Fatal("脚本已安装时 st.Watching 应为 true")
	}
	if st.ProjectPath != root {
		t.Fatalf("ProjectPath = %q, want %q", st.ProjectPath, root)
	}
}

func TestCollectStatusScriptMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "") // 隔离真实 codegraph，保证确定性
	root := t.TempDir()

	st, err := collectStatus(context.Background(), root)
	if err != nil {
		t.Fatalf("collectStatus 报错: %v", err)
	}
	if st.Watching {
		t.Fatal("脚本缺失时 st.Watching 应为 false")
	}
	// codegraph 不在 PATH：Codegraph 字段应保持为空（LookPath 失败即返回）
	if len(st.Codegraph) != 0 {
		t.Fatalf("无 codegraph 时 Codegraph 应为空, got %s", st.Codegraph)
	}
}

// TestCollectStatusWithCodegraph fake codegraph status --json 输出被透传进 st.Codegraph。
func TestCollectStatusWithCodegraph(t *testing.T) {
	skipOnWindows(t)
	installFakeCodegraph(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	writeFakeScript(t, filepath.Join(root, ".cursor", "skills", skillID, "scripts", "generate-agents.sh"),
		"echo ok")

	st, err := collectStatus(context.Background(), root)
	if err != nil {
		t.Fatalf("collectStatus 报错: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(st.Codegraph, &m); err != nil {
		t.Fatalf("Codegraph 应为合法 JSON: %v (raw=%s)", err, st.Codegraph)
	}
	if init, ok := m["initialized"].(bool); !ok || !init {
		t.Fatalf("initialized 应为 true, got %v", m["initialized"])
	}
	if !st.Watching {
		t.Fatal("脚本已安装时 st.Watching 应为 true")
	}
}

// --- PrintStatus ---

func TestPrintStatusQuietJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	var buf bytes.Buffer
	if err := PrintStatus(context.Background(), Options{ProjectPath: root, Quiet: true}, &buf); err != nil {
		t.Fatalf("PrintStatus 报错: %v", err)
	}
	var st Status
	if err := json.Unmarshal(buf.Bytes(), &st); err != nil {
		t.Fatalf("Quiet 模式应输出合法 JSON: %v (raw=%q)", err, buf.String())
	}
	if st.ProjectPath != root {
		t.Fatalf("ProjectPath = %q, want %q", st.ProjectPath, root)
	}
	if st.Watching {
		t.Fatal("脚本缺失时 Watching 应为 false")
	}
}

func TestPrintStatusText(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	var buf bytes.Buffer
	if err := PrintStatus(context.Background(), Options{ProjectPath: root}, &buf); err != nil {
		t.Fatalf("PrintStatus 报错: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, root) {
		t.Fatalf("文本输出应包含项目路径: %q", out)
	}
	// 无 codegraph 且无脚本：两条能力都应提示未就绪
	if !strings.Contains(out, "未安装") {
		t.Fatalf("无 codegraph 时应提示未安装: %q", out)
	}
	if !strings.Contains(out, "未安装") || !strings.Contains(out, "codegraph-kit") {
		t.Fatalf("无脚本时应提示安装 codegraph-kit: %q", out)
	}
}

// TestPrintStatusTextWithCodegraph fake codegraph 已初始化时文本输出应报告已索引。
func TestPrintStatusTextWithCodegraph(t *testing.T) {
	skipOnWindows(t)
	installFakeCodegraph(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	writeFakeScript(t, filepath.Join(root, ".qoder", "skills", skillID, "scripts", "generate-agents.sh"),
		"echo ok")

	var buf bytes.Buffer
	if err := PrintStatus(context.Background(), Options{ProjectPath: root}, &buf); err != nil {
		t.Fatalf("PrintStatus 报错: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "已索引") {
		t.Fatalf("fake codegraph 已初始化时应输出已索引: %q", out)
	}
	if !strings.Contains(out, "已安装") {
		t.Fatalf("脚本已安装时应输出已安装: %q", out)
	}
}
