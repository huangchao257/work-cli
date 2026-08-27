package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huangchao257/work-cli/internal/api"
)

// --- R4-m2/m8: --data 校验 ---

func TestAPICloneDataRejectsNonJSON(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "call", "demo", "createPet", "--data", `{name: rex}`, "--dry-run"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("non-JSON --data should fail")
	}
	if !strings.Contains(buf.String(), "不是合法 JSON") {
		t.Fatalf("output = %s", buf.String())
	}
}

func TestAPICloneDataRejectsParentTraversal(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "call", "demo", "createPet", "--data", "@/../etc/hostname", "--yes", "--dry-run"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("--data @../ traversal should fail")
	}
	if !strings.Contains(buf.String(), "当前目录") {
		t.Fatalf("output = %s", buf.String())
	}
}

func TestAPICloneDataFromFileOK(t *testing.T) {
	dir := t.TempDir()
	// --data @file 相对 CWD；chdir 到临时目录
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.WriteFile(filepath.Join(dir, "pet.json"), []byte(`{"name":"rex"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "call", "demo", "createPet", "--data", "@pet.json", "--dry-run", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("valid @file should pass: %v (out=%s)", err, buf.String())
	}
	// JSON 信封中 body 字符串被转义；断言读到的原文内容即可
	if !strings.Contains(buf.String(), `\"name\":\"rex\"`) {
		t.Fatalf("body should include file content: %s", buf.String())
	}
}

// --- R4-m3: --params 数组/null 值拒绝 ---

func TestAPICloneParamsRejectsArrayValue(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "call", "demo", "listPets", "--params", `{"tags":["a","b"]}`, "--dry-run"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("array --params value should fail")
	}
	if !strings.Contains(buf.String(), "标量") {
		t.Fatalf("output = %s", buf.String())
	}
}

// --- R6: 参数格式错误统一 usage 退出码 2（此前 fmt.Errorf 导致 exit 1，与文档矩阵不一致） ---

func TestAPICloneParamFormatErrorsExit2(t *testing.T) {
	cases := [][]string{
		{"api", "call", "demo", "createPet", "--data", `{bad}`, "--dry-run"},
		{"api", "call", "demo", "listPets", "--params", `{"tags":["a"]}`, "--dry-run"},
		{"api", "call", "demo", "listPets", "--set", "noequals", "--dry-run"},
		{"api", "call", "demo", "listPets", "--header", "noequals", "--dry-run"},
		{"api", "call", "demo", "createPet", "--data", "@/../x", "--dry-run"},
	}
	for _, args := range cases {
		cmd, buf := newTestAPICmd(t)
		cmd.SetArgs(args)
		err := cmd.Execute()
		if err == nil {
			t.Fatalf("%v should fail", args)
		}
		if code := ExitCode(err); code != 2 {
			t.Fatalf("%v: exit = %d, want 2 (output: %s)", args, code, buf.String())
		}
	}
}

// --- R6: handler 型 shortcut dry-run 的 --json 输出 JSON 信封 ---

func TestAPICloneHandlerShortcutDryRunJSON(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "demo", "+seed", "--dry-run", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run failed: %v (out=%s)", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, `"dry_run": true`) {
		t.Fatalf("JSON envelope missing dry_run: %s", out)
	}
	// 必须是合法 JSON（首字符为 {），而非人类可读文本
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("--json must emit JSON envelope, got: %s", out)
	}
}

// --- R4-m9: import --base-url 校验 ---

func TestAPICloneImportRejectsSchemelessBaseURL(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	specPath := filepath.Join(t.TempDir(), "spec.yaml")
	spec := `
openapi: 3.0.3
info: {title: Mini, version: "1"}
servers: [{url: https://mini.invalid}]
paths:
  /ping:
    get: {operationId: ping, responses: {"200": {description: ok}}}
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd.SetArgs([]string{"api", "import", "noscheme", specPath, "--base-url", "example.com"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("schemeless --base-url should fail")
	}
	if !strings.Contains(buf.String(), "http(s)://") {
		t.Fatalf("output = %s", buf.String())
	}
	if api.SystemExists("noscheme") {
		t.Fatal("failed import must not leave system behind")
	}
}

// --- R4-m7: 分组节点未知子命令（临时审计发现的 exit 0） ---

func TestAPICloneGroupNodeUnknownSubcommand(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "demo", "pets", "nosuchop"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("unknown subcommand under group node should fail")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(buf.String(), "nosuchop") {
		t.Fatalf("output should mention the unknown token: %s", buf.String())
	}
}
