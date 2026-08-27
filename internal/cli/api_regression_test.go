package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- R2: 未知子命令 exit 2 且有输出 ---

func TestAPICloneUnknownSubcommandExits2(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "nosuchcmd"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("unknown subcommand should fail")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(buf.String(), "未知子命令") {
		t.Fatalf("output = %s", buf.String())
	}
}

func TestAPICloneUnknownSystemSubcommandExits2(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "demo", "nosuchsub"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("unknown system subcommand should fail")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(buf.String(), "未知子命令") {
		t.Fatalf("output = %s", buf.String())
	}
}

// --- R3: Args 校验 / flag 解析错误不再静默 ---

func TestAPICloneArgsErrorPrintsOnce(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	// schema 缺参数（RangeArgs 校验失败）
	cmd.SetArgs([]string{"api", "schema"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("missing arg should fail")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if got := strings.Count(buf.String(), "accepts between"); got != 1 {
		t.Fatalf("error printed %d times, want 1: %s", got, buf.String())
	}
}

func TestAPICloneFlagErrorPrintsOnce(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "list", "--nope"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("unknown flag should fail")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if got := strings.Count(buf.String(), "unknown flag"); got != 1 {
		t.Fatalf("error printed %d times, want 1: %s", got, buf.String())
	}
}

// --- R1/R2: 退出码分类（usage→2 / environment→3） ---

func TestAPICloneMissingCredentialExits3(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	specPath := filepath.Join(t.TempDir(), "secure.yaml")
	spec := `
openapi: 3.0.3
info: {title: Secure, version: "1"}
servers: [{url: https://secure.invalid}]
tags: [{name: t}]
paths:
  /secret:
    get:
      operationId: getSecret
      tags: [t]
      responses: {"200": {description: ok}}
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd.SetArgs([]string{"api", "import", "secure", specPath, "--auth", "bearer", "--credential-env", "SECURE_TOKEN_XYZ"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SECURE_TOKEN_XYZ", "")

	cmd.SetArgs([]string{"api", "call", "secure", "getSecret"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("missing credential should fail")
	}
	if code := ExitCode(err); code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
	if !strings.Contains(buf.String(), "SECURE_TOKEN_XYZ") {
		t.Fatalf("output should mention env var: %s", buf.String())
	}
}

// --- R1: 无 tags 规范不 panic（default 分组兜底） ---

func TestAPICloneNoTagsSpecUsesDefaultGroup(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	specPath := filepath.Join(t.TempDir(), "notags.yaml")
	spec := `
openapi: 3.0.3
info: {title: NoTags, version: "1"}
servers: [{url: https://notags.invalid}]
paths:
  /a:
    get:
      operationId: doA
      responses: {"200": {description: ok}}
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd.SetArgs([]string{"api", "import", "notags", specPath, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("import no-tags spec failed: %v (output: %s)", err, buf.String())
	}
	// 导入成功后（本进程命令树已装配），schema 应能查到 default 分组
	cmd.SetArgs([]string{"api", "schema", "notags"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("schema on no-tags system failed: %v", err)
	}
	if !strings.Contains(buf.String(), "doA") {
		t.Fatalf("schema output missing doA: %s", buf.String())
	}
	// catalog 的 cli-path 应落 default 组
	systems, warnings := defaultAPIDeps().collectSystems()
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
	found := false
	for _, s := range systems {
		if s.Manifest().Name != "notags" {
			continue
		}
		found = true
		catalog, err := s.Catalog(cmd.Context())
		if err != nil {
			t.Fatal(err)
		}
		op, ok := catalog.FindByID("doA")
		if !ok || !op.Dynamic {
			t.Fatalf("doA should be dynamic: %#v", op)
		}
		if len(op.CLIPath) != 2 || op.CLIPath[0] != "default" {
			t.Fatalf("cli-path = %v, want [default do-a]", op.CLIPath)
		}
	}
	if !found {
		t.Fatal("notags not in collectSystems() — cli-path 断言被静默跳过")
	}
}

// --- R2: shortcut dry-run 合并预设参数 ---

func TestAPICloneShortcutDryRunIncludesPresetParams(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "demo", "+top", "--dry-run", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run failed: %v (output: %s)", err, buf.String())
	}
	if !strings.Contains(buf.String(), "limit=5") {
		t.Fatalf("dry-run URL should include preset limit=5: %s", buf.String())
	}
	// 显式参数覆盖预设
	buf.Reset()
	cmd.SetArgs([]string{"api", "demo", "+top", "--dry-run", "--json", "--set", "query.limit=2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "limit=2") {
		t.Fatalf("explicit param should override preset: %s", buf.String())
	}
}
