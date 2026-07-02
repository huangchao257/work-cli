package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInTemp 在临时目录下运行一次 scaffold.Run，返回生成的文件绝对路径列表与根目录。
func runInTemp(t *testing.T, typ Type, name string) ([]string, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	files, err := Run(Options{Type: typ, Name: name, Dir: dir})
	if err != nil {
		t.Fatalf("Run(%s) 失败: %v", typ, err)
	}
	return files, dir
}

func TestRunBundle(t *testing.T) {
	files, dir := runInTemp(t, TypeBundle, "my-kit")

	want := map[string]bool{
		filepath.Join(dir, "bundle.yaml"):                  true,
		filepath.Join(dir, "skills", "my-kit", "SKILL.md"): true,
		filepath.Join(dir, "rules", "sample.md"):           true,
		filepath.Join(dir, "mcp", "sample.json"):           true,
	}
	got := make(map[string]bool, len(files))
	for _, f := range files {
		got[f] = true
	}
	for w := range want {
		if !got[w] {
			t.Errorf("缺少文件 %s", w)
		}
	}
	for _, w := range []string{"bundle.yaml", "skills", "rules", "mcp"} {
		if _, err := os.Stat(filepath.Join(dir, w)); err != nil {
			t.Errorf("文件不存在 %s: %v", w, err)
		}
	}

	// manifest 含正确 name/version。
	b, err := os.ReadFile(filepath.Join(dir, "bundle.yaml"))
	if err != nil {
		t.Fatalf("读取 bundle.yaml 失败: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, "name: my-kit") {
		t.Errorf("bundle.yaml 缺少 name: my-kit\n%s", s)
	}
	if !strings.Contains(s, "version: 0.1.0") {
		t.Errorf("bundle.yaml 缺少 version: 0.1.0\n%s", s)
	}
}

func TestRunCLI(t *testing.T) {
	files, dir := runInTemp(t, TypeCLI, "my-tool")

	want := map[string]bool{
		filepath.Join(dir, "installer.yaml"): true,
		filepath.Join(dir, "README.md"):      true,
	}
	got := make(map[string]bool, len(files))
	for _, f := range files {
		got[f] = true
	}
	for w := range want {
		if !got[w] {
			t.Errorf("缺少文件 %s", w)
		}
	}

	b, err := os.ReadFile(filepath.Join(dir, "installer.yaml"))
	if err != nil {
		t.Fatalf("读取 installer.yaml 失败: %v", err)
	}
	s := string(b)
	for _, want := range []string{"type: cli", "name: my-tool", "version: 0.1.0"} {
		if !strings.Contains(s, want) {
			t.Errorf("installer.yaml 缺少 %q\n%s", want, s)
		}
	}
}

func TestRunHooks(t *testing.T) {
	files, dir := runInTemp(t, TypeHooks, "my-hooks")

	script := filepath.Join(dir, "scripts", "telemetry.sh")
	want := map[string]bool{
		filepath.Join(dir, "hooks.yaml"): true,
		script:                           true,
	}
	got := make(map[string]bool, len(files))
	for _, f := range files {
		got[f] = true
	}
	for w := range want {
		if !got[w] {
			t.Errorf("缺少文件 %s", w)
		}
	}

	// telemetry.sh 有可执行位。
	info, err := os.Stat(script)
	if err != nil {
		t.Fatalf("stat telemetry.sh 失败: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("telemetry.sh 缺少可执行位: %v", info.Mode())
	}

	// manifest 含正确 name/version/type。
	b, err := os.ReadFile(filepath.Join(dir, "hooks.yaml"))
	if err != nil {
		t.Fatalf("读取 hooks.yaml 失败: %v", err)
	}
	s := string(b)
	for _, want := range []string{"type: hooks", "name: my-hooks", "version: 0.1.0", "preset: audit"} {
		if !strings.Contains(s, want) {
			t.Errorf("hooks.yaml 缺少 %q\n%s", want, s)
		}
	}
}

func TestParseTypeInvalid(t *testing.T) {
	if _, err := ParseType("unknown"); err == nil {
		t.Fatal("ParseType 对非法值应返回错误")
	}
	for _, ok := range []string{"bundle", "cli", "hooks"} {
		if _, err := ParseType(ok); err != nil {
			t.Errorf("ParseType(%q) 不应报错: %v", ok, err)
		}
	}
}

func TestRunDirExistsNonEmpty(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "exist")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Run(Options{Type: TypeBundle, Name: "x", Dir: dir})
	if err == nil {
		t.Fatal("目录已存在且非空应返回错误")
	}
	if !IsUsageError(err) {
		t.Errorf("目录已存在非空应为 usage error，got %T: %v", err, err)
	}
}

func TestRunDryRun(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "preview")
	files, err := Run(Options{Type: TypeCLI, Name: "preview-tool", Dir: dir, DryRun: true})
	if err != nil {
		t.Fatalf("DryRun 失败: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("DryRun 应返回 2 个文件路径，got %d", len(files))
	}
	// 预览模式不应写盘。
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("DryRun 不应创建目录 %s", dir)
	}
}
