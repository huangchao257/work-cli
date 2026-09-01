package graph

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- 共享测试辅助 ---

// skipOnWindows 跳过依赖 POSIX shell（fake 脚本 / bash）的用例。
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("依赖 POSIX shell（fake 脚本 / bash）")
	}
}

// writeFakeScript 在 path 写入 shell 脚本并赋予可执行权限。
func writeFakeScript(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" + content
	if !strings.HasSuffix(content, "\n") {
		script += "\n"
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// installFakeCodegraph 把伪造的 codegraph 命令放进 PATH（排最前），
// 并让 `codegraph status --json` 输出已初始化状态。
func installFakeCodegraph(t *testing.T) {
	t.Helper()
	skipOnWindows(t)
	binDir := t.TempDir()
	writeFakeScript(t, filepath.Join(binDir, "codegraph"), `if [ "$1" = "status" ]; then
	printf '{"initialized": true, "fileCount": 3, "nodeCount": 42}'
fi
exit 0`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// --- resolveRoot ---

func TestResolveRoot(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		path string
		want string
	}{
		// 空串与纯空白都视为“未指定”，回退当前目录
		{"空串回退当前目录", "", cwd},
		{"空白串回退当前目录", "   \t ", cwd},
		{"相对路径转绝对", "sub/../..", filepath.Clean(filepath.Join(cwd, "sub", "..", ".."))},
		{"绝对路径原样返回", "/definitely/not/exist", "/definitely/not/exist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveRoot(tt.path)
			if err != nil {
				t.Fatalf("resolveRoot(%q) 报错: %v", tt.path, err)
			}
			if got != tt.want {
				t.Fatalf("resolveRoot(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// --- findScript ---

// TestFindScriptUserScopeWins 验证用户级脚本优先于项目级：
// 恶意仓库在项目内自带同名脚本时，findScript 必须返回用户级可信脚本。
func TestFindScriptUserScopeWins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := t.TempDir()
	// 项目级“恶意”脚本
	writeFakeScript(t, filepath.Join(root, ".claude", "skills", skillID, "scripts", "generate-agents.sh"),
		"echo malicious")
	// 用户级可信脚本
	userScript := filepath.Join(home, ".cursor", "skills", skillID, "scripts", "generate-agents.sh")
	writeFakeScript(t, userScript, "echo trusted")

	got, err := findScript(root, "generate-agents.sh")
	if err != nil {
		t.Fatalf("findScript 报错: %v", err)
	}
	if got != userScript {
		t.Fatalf("用户级脚本应优先: got %q, want %q", got, userScript)
	}
}

// TestFindScriptUserScopeOrder 验证用户级三家 IDE 的候选顺序（cursor → claude → qoder）。
func TestFindScriptUserScopeOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// claude 与 qoder 用户级都存在时，cursor 不存在 → 应选 claude
	claude := filepath.Join(home, ".claude", "skills", skillID, "scripts", "generate-agents.sh")
	writeFakeScript(t, claude, "echo claude")
	qoder := filepath.Join(home, ".qoder", "skills", skillID, "scripts", "generate-agents.sh")
	writeFakeScript(t, qoder, "echo qoder")

	root := t.TempDir()
	got, err := findScript(root, "generate-agents.sh")
	if err != nil {
		t.Fatalf("findScript 报错: %v", err)
	}
	if got != claude {
		t.Fatalf("用户级顺序应为 claude 先于 qoder: got %q, want %q", got, claude)
	}
}

// TestFindScriptProjectFallback 覆盖三家 IDE 的项目级回退。
func TestFindScriptProjectFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // 无用户级脚本

	for _, tt := range []struct {
		name   string
		dotDir string
	}{
		{"cursor 项目级回退", ".cursor"},
		{"claude 项目级回退", ".claude"},
		{"qoder 项目级回退", ".qoder"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			want := filepath.Join(root, tt.dotDir, "skills", skillID, "scripts", "generate-agents.sh")
			writeFakeScript(t, want, "echo ok")

			got, err := findScript(root, "generate-agents.sh")
			if err != nil {
				t.Fatalf("findScript 报错: %v", err)
			}
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

// TestFindScriptCursorBeforeClaude 验证项目级候选顺序中 .cursor 优先于 .claude。
func TestFindScriptCursorBeforeClaude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := t.TempDir()
	cur := filepath.Join(root, ".cursor", "skills", skillID, "scripts", "generate-agents.sh")
	writeFakeScript(t, cur, "echo cursor")
	cla := filepath.Join(root, ".claude", "skills", skillID, "scripts", "generate-agents.sh")
	writeFakeScript(t, cla, "echo claude")

	got, err := findScript(root, "generate-agents.sh")
	if err != nil {
		t.Fatalf("findScript 报错: %v", err)
	}
	if got != cur {
		t.Fatalf("项目级顺序应为 .cursor 优先: got %q, want %q", got, cur)
	}
}

// TestFindScriptRejectsDirectory 验证候选路径是目录时被跳过（继续探测后续候选）。
func TestFindScriptRejectsDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := t.TempDir()
	// .cursor 候选是目录：应跳过，回退到 .claude 的真实文件
	if err := os.MkdirAll(filepath.Join(root, ".cursor", "skills", skillID, "scripts", "generate-agents.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".claude", "skills", skillID, "scripts", "generate-agents.sh")
	writeFakeScript(t, want, "echo ok")

	got, err := findScript(root, "generate-agents.sh")
	if err != nil {
		t.Fatalf("findScript 报错: %v", err)
	}
	if got != want {
		t.Fatalf("目录候选应被跳过: got %q, want %q", got, want)
	}
}

// TestFindScriptNotFound 无任何候选时返回错误。
func TestFindScriptNotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()

	got, err := findScript(root, "generate-agents.sh")
	if err == nil {
		t.Fatalf("应返回错误，却得到 %q", got)
	}
	if !strings.Contains(err.Error(), "generate-agents.sh") {
		t.Fatalf("错误信息应包含脚本名: %v", err)
	}
}

// --- runBash ---

func TestRunBash(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()

	t.Run("成功", func(t *testing.T) {
		script := filepath.Join(dir, "ok.sh")
		writeFakeScript(t, script, "exit 0")
		if err := runBash(context.Background(), dir, script); err != nil {
			t.Fatalf("runBash 成功路径报错: %v", err)
		}
	})

	t.Run("失败信息包含脚本名", func(t *testing.T) {
		script := filepath.Join(dir, "fail.sh")
		writeFakeScript(t, script, "exit 3")
		err := runBash(context.Background(), dir, script)
		if err == nil {
			t.Fatal("应返回错误")
		}
		if !strings.Contains(err.Error(), "fail.sh") {
			t.Fatalf("错误信息应包含脚本基名: %v", err)
		}
	})
}

// --- RunPostInstall scope 门禁 ---

func TestRunPostInstallScopeGate(t *testing.T) {
	ctx := context.Background()

	t.Run("user scope 直接返回", func(t *testing.T) {
		// PATH 无 codegraph、无脚本：若误执行 Init 必然报错
		t.Setenv("PATH", t.TempDir())
		if err := RunPostInstall(ctx, "user", false); err != nil {
			t.Fatalf("user scope 应为 no-op: %v", err)
		}
	})

	t.Run("dryRun 直接返回", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if err := RunPostInstall(ctx, "project", true); err != nil {
			t.Fatalf("dryRun 应为 no-op: %v", err)
		}
	})

	t.Run("project 且非 dryRun 会执行 Init", func(t *testing.T) {
		skipOnWindows(t)
		installFakeCodegraph(t)
		home := t.TempDir()
		t.Setenv("HOME", home)

		// RunPostInstall 不接收项目路径，固定作用于当前目录：chdir 到临时根。
		root := t.TempDir()
		t.Chdir(root)
		marker := filepath.Join(root, "gen.marker")
		writeFakeScript(t, filepath.Join(root, ".claude", "skills", skillID, "scripts", "generate-agents.sh"),
			"echo ran >> \""+marker+"\"")

		if err := RunPostInstall(ctx, "project", false); err != nil {
			t.Fatalf("RunPostInstall 报错: %v", err)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("project scope 应触发 Init 执行 generate-agents.sh: %v", err)
		}
	})
}

// --- Init / Sync ---

func TestInitSyncDryRun(t *testing.T) {
	// DryRun 在 resolveRoot 之后立即返回，不触碰 PATH / 脚本查找。
	ctx := context.Background()
	t.Setenv("PATH", t.TempDir()) // 无 codegraph：若 DryRun 失效会在此暴露
	if err := Init(ctx, Options{ProjectPath: t.TempDir(), DryRun: true}); err != nil {
		t.Fatalf("Init dry-run 报错: %v", err)
	}
	if err := Sync(ctx, Options{ProjectPath: t.TempDir(), DryRun: true}); err != nil {
		t.Fatalf("Sync dry-run 报错: %v", err)
	}
}

// TestInitSyncExecScript 验证非 dry-run 路径端到端执行项目级 generate-agents.sh。
func TestInitSyncExecScript(t *testing.T) {
	skipOnWindows(t)
	installFakeCodegraph(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := t.TempDir()
	marker := filepath.Join(root, "gen.marker")
	writeFakeScript(t, filepath.Join(root, ".cursor", "skills", skillID, "scripts", "generate-agents.sh"),
		"echo ran >> \""+marker+"\"")

	t.Run("Init 执行脚本", func(t *testing.T) {
		if err := Init(context.Background(), Options{ProjectPath: root, Quiet: true}); err != nil {
			t.Fatalf("Init 报错: %v", err)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("Init 未执行 generate-agents.sh: %v", err)
		}
	})

	t.Run("Sync 执行脚本", func(t *testing.T) {
		if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := Sync(context.Background(), Options{ProjectPath: root}); err != nil {
			t.Fatalf("Sync 报错: %v", err)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("Sync 未执行 generate-agents.sh: %v", err)
		}
	})
}

// TestInitSyncMissingCodegraph codegraph 不在 PATH 时应给出可操作的错误提示。
func TestInitSyncMissingCodegraph(t *testing.T) {
	skipOnWindows(t)
	t.Setenv("PATH", t.TempDir()) // 清空 PATH：无 codegraph
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	// 项目级脚本就位，确保失败只能来自 codegraph 缺失
	writeFakeScript(t, filepath.Join(root, ".claude", "skills", skillID, "scripts", "generate-agents.sh"),
		"echo ran")

	ctx := context.Background()
	err := Init(ctx, Options{ProjectPath: root})
	if err == nil || !strings.Contains(err.Error(), "codegraph-stack") {
		t.Fatalf("Init 应提示安装 codegraph-stack: %v", err)
	}
	err = Sync(ctx, Options{ProjectPath: root})
	if err == nil || !strings.Contains(err.Error(), "codegraph-stack") {
		t.Fatalf("Sync 应提示安装 codegraph-stack: %v", err)
	}
}

// TestInitSyncMissingScript 脚本缺失时应提示先安装 codegraph-kit。
func TestInitSyncMissingScript(t *testing.T) {
	skipOnWindows(t)
	installFakeCodegraph(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir() // 无任何项目级脚本

	ctx := context.Background()
	err := Init(ctx, Options{ProjectPath: root})
	if err == nil || !strings.Contains(err.Error(), "codegraph-kit") {
		t.Fatalf("Init 应提示安装 codegraph-kit: %v", err)
	}
	err = Sync(ctx, Options{ProjectPath: root})
	if err == nil || !strings.Contains(err.Error(), "generate-agents.sh") {
		t.Fatalf("Sync 应提示脚本缺失: %v", err)
	}
}
