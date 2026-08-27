package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// --- R5-M1: --yes 残留不得绕过确认门禁（同进程多次 Execute） ---

func TestYesDoesNotLeakAcrossExecutes(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	// 第 1 次：显式 --yes 的 dangerous 操作
	cmd.SetArgs([]string{"api", "call", "demo", "deletePet", "--set", "path.id=p-1", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec1 should pass with --yes: %v (out=%s)", err, buf.String())
	}
	buf.Reset()
	// 第 2 次：不带 --yes 的同型操作——残留的 apiCallYes 不得让它免确认
	cmd.SetArgs([]string{"api", "call", "demo", "deletePet", "--set", "path.id=p-2"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("exec2 without --yes should fail closed (--yes leaked from exec1?)")
	}
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(buf.String(), "非交互环境且未提供 --yes") {
		t.Fatalf("output = %s", buf.String())
	}
}

// --- R5-M1b: --set/--params 残留不得跨 Execute 追加 ---

func TestSetDoesNotAccumulateAcrossExecutes(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "call", "demo", "listPets", "--set", "query.limit=1", "--dry-run", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	// 第二次只传 limit=2：若 stringArray 追加，URL 会含 limit=2（Set 覆盖）或旧值 1
	cmd.SetArgs([]string{"api", "call", "demo", "listPets", "--set", "query.limit=2", "--dry-run", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "limit=1") {
		t.Fatalf("stale --set value leaked: %s", out)
	}
	if !strings.Contains(out, "limit=2") {
		t.Fatalf("current --set value missing: %s", out)
	}
}

// --- R5-M3: clone 树动态命令 flag 错误也要有输出且 exit 2 ---

func TestAPICloneDynamicFlagErrorPrintsOnce(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "demo", "pets", "list-pets", "--nope"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("unknown flag on dynamic command should fail")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if got := strings.Count(buf.String(), "unknown flag"); got != 1 {
		t.Fatalf("error printed %d times, want 1: %s", got, buf.String())
	}
}

// --- R5-m1: L2 叶子拒绝多余位置参数 ---

func TestAPICloneLeafRejectsExtraArgs(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "demo", "pets", "list-pets", "bogus"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("extra positional arg on leaf should fail")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(buf.String(), "bogus") {
		t.Fatalf("output should mention the extra arg: %s", buf.String())
	}
}

// --- R5-M3b: 生产 apiCmd 树上的动态命令也必须装上兜底错误函数 ---
// （applyAPIErrorFuncs 曾因 per-command 短路放在函数开头导致递归中断，
// 第二次 setAPIErrorFuncs 无法覆盖新挂载的动态命令——真机 Args 错误 exit 1 且静默。）

func TestProdTreeDynamicCommandsCovered(t *testing.T) {
	leaf := findProdCmdForTest("demo", "pets", "list-pets")
	if leaf == nil {
		t.Fatal("demo leaf not found on prod apiCmd tree")
	}
	if leaf.Args == nil {
		t.Fatal("leaf.Args is nil")
	}
	// 已包装的 Args 拒绝多余参数时应返回 exit 2 的错误（apiFail 包装），而非裸错误 exit 1
	err := leaf.Args(leaf, []string{"bogus"})
	if err == nil {
		t.Fatal("Args should reject bogus")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("prod-tree wrapped Args should exit 2, got %d (err=%v)", code, err)
	}
}

func findProdCmdForTest(path ...string) *cobra.Command {
	cur := apiCmd
	for _, seg := range path {
		found := false
		for _, c := range cur.Commands() {
			if c.Name() == seg {
				cur = c
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	return cur
}
