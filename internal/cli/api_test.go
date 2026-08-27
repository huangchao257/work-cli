package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/huangchao257/work-cli/internal/api"
	"github.com/huangchao257/work-cli/internal/api/demo"
)

// newTestAPICmd 构造隔离的 api 命令树（不动包级 apiCmd，避免污染其他测试）。
// 注意：包级 flag 变量（apiCall*/apiImport*/asJSON 等）由 singleton 命令绑定，
// pflag 不会在两次 Execute 之间复位旧值，必须在每个测试开头手动重置。
// 勿对本包测试使用 t.Parallel（包级 flag 变量与 attachSystemCmd 非并发安全）。
func newTestAPICmd(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	resetAPIFlagState()

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{Use: "work", SilenceErrors: true, SilenceUsage: true}
	cmd.PersistentFlags().BoolVar(&asJSON, "json", false, "JSON 格式输出")
	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "仅预览将执行的操作")
	apiCmdClone := &cobra.Command{
		Use:   "api",
		Short: apiCmd.Short,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return apiUnknownTarget(cmd, "子命令", args[0], "运行 work api --help 查看可用命令")
			}
			return cmd.Help()
		},
	}
	apiCmdClone.AddCommand(apiListCmd, apiInfoCmd, apiImportCmd, apiRefreshCmd, apiRemoveCmd, apiSchemaCmd, apiCallCmd)
	// 动态命令：注册 demo 到 clone 树
	originalAttach := attachSystemCmd
	attachSystemCmd = func(cmd *cobra.Command) { apiCmdClone.AddCommand(cmd) }
	defer func() { attachSystemCmd = originalAttach }()
	registerSystemCommands(demo.New(), nil, map[string]bool{})
	attachSystemCmd = originalAttach
	// clone 树与生产树同待遇：装兜底错误函数（静态命令是单例，生产 init 已装过、
	// per-command 幂等标记会跳过；动态命令是本测试新建的，会真正装上）
	setAPIErrorFuncs(apiCmdClone)
	cmd.AddCommand(apiCmdClone)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())
	return cmd, buf
}

// resetAPIFlagState 重置 api 子树绑定的全部包级 flag 变量到默认值。
// singleton 子命令的 flag 只注册一次，pflag 在多次 Execute 间不复位旧值；
// 不重置则先跑的测试会把 --base-url/--auth/--yes 等状态泄漏给后续测试。
func resetAPIFlagState() {
	asJSON = false
	dryRun = false
	apiCallYes = false
	apiCallData = ""
	apiCallParams = nil
	apiCallSet = nil
	apiCallHeader = nil
	apiImportBaseURL = ""
	apiImportAuthKind = ""
	apiImportCredEnv = ""
	apiImportAuthHeader = ""
	apiImportAuthQuery = ""
	apiImportOverwrite = false
	apiSchemaCompact = false
	apiSchemaAll = false
	apiRemoveYes = false
}

func TestAPICloneListIncludesDemo(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "demo") {
		t.Fatalf("list output missing demo:\n%s", buf.String())
	}
}

func TestAPICloneSchemaDemo(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "schema", "demo", "--compact"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"listPets", "createPet", "delete-pet", "+seed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %s:\n%s", want, out)
		}
	}
}

func TestAPICloneCallRead(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "call", "demo", "listPets", "--params", `{"limit":2}`, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"ok": true`) {
		t.Fatalf("call output = %s", buf.String())
	}
}

func TestAPICloneCallWriteFailClosed(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "call", "demo", "createPet", "--data", `{"name":"rex"}`})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("write call should fail closed")
	}
	if code := ExitCode(err); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	// 锁定结构化确认语义（而非宽泛的"确认"二字）
	if !strings.Contains(buf.String(), "非交互环境且未提供 --yes") {
		t.Fatalf("output = %s", buf.String())
	}
}

func TestAPICloneCallDryRun(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "call", "demo", "createPet", "--data", `{"name":"rex"}`, "--dry-run", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"dry_run": true`) {
		t.Fatalf("dry-run output = %s", buf.String())
	}
}

func TestAPICloneUnknownSystemExit2(t *testing.T) {
	cmd, _ := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "call", "nope", "x"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("unknown system should fail")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestAPICloneImportAndRemove(t *testing.T) {
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
	cmd.SetArgs([]string{"api", "import", "mini", specPath, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"name": "mini"`) {
		t.Fatalf("import output = %s", buf.String())
	}

	// asJSON 会跨 Execute 泄漏（包级变量），显式复位以锁定本测试的渲染路径
	asJSON = false
	buf.Reset()
	cmd.SetArgs([]string{"api", "remove", "mini", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// human 渲染路径也应被覆盖到
	if !strings.Contains(buf.String(), "mini") {
		t.Fatalf("remove output = %s", buf.String())
	}
	if api.SystemExists("mini") {
		t.Fatal("mini should be removed")
	}
}

func TestAPICloneDynamicCommandHelp(t *testing.T) {
	cmd, buf := newTestAPICmd(t)
	cmd.SetArgs([]string{"api", "demo"})
	// 无参调用 demo 系统命令时输出帮助（含 pets 组）
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "pets") {
		t.Fatalf("demo help missing pets group:\n%s", buf.String())
	}
}
