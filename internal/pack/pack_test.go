package pack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeBundle 在临时目录下创建一个含 bundle.yaml 与资源文件的套装目录。
func makeBundle(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "mykit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	manifest := "name: mykit\nversion: 0.1.0\ntype: bundle\ndescription: test bundle\n"
	if err := os.WriteFile(filepath.Join(dir, "bundle.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("写入 manifest 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "resource.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("写入资源文件失败: %v", err)
	}
	return dir
}

func assertArchiveAndChecksum(t *testing.T, res Result) {
	t.Helper()
	if _, err := os.Stat(res.Archive); err != nil {
		t.Fatalf("归档文件不存在 %s: %v", res.Archive, err)
	}
	data, err := os.ReadFile(res.Checksum)
	if err != nil {
		t.Fatalf("校验和文件不存在 %s: %v", res.Checksum, err)
	}
	// 仿 sha256sum：<hex>  <filename>
	parts := strings.SplitN(strings.TrimSpace(string(data)), "  ", 2)
	if len(parts) != 2 {
		t.Fatalf("校验和格式非法: %q", string(data))
	}
	if len(parts[0]) != 64 {
		t.Fatalf("sha256 长度异常: %d", len(parts[0]))
	}
	if parts[1] != filepath.Base(res.Archive) {
		t.Fatalf("校验和文件名不匹配: got %q want %q", parts[1], filepath.Base(res.Archive))
	}
	if res.FileCount <= 0 {
		t.Fatalf("FileCount 应 >0，实际 %d", res.FileCount)
	}
	if res.Name != "mykit" || res.Version != "0.1.0" {
		t.Fatalf("name/version 不匹配: got %s/%s", res.Name, res.Version)
	}
}

func TestRunTarGz(t *testing.T) {
	dir := makeBundle(t)
	res, err := Run(Options{Dir: dir, Format: FormatTarGz})
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if !strings.HasSuffix(res.Archive, ".tar.gz") {
		t.Fatalf("归档扩展名错误: %s", res.Archive)
	}
	assertArchiveAndChecksum(t, res)
}

func TestRunZip(t *testing.T) {
	dir := makeBundle(t)
	res, err := Run(Options{Dir: dir, Format: FormatZip})
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if !strings.HasSuffix(res.Archive, ".zip") {
		t.Fatalf("归档扩展名错误: %s", res.Archive)
	}
	assertArchiveAndChecksum(t, res)
}

func TestRunDryRun(t *testing.T) {
	dir := makeBundle(t)
	res, err := Run(Options{Dir: dir, Format: FormatTarGz, DryRun: true})
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if res.FileCount <= 0 {
		t.Fatalf("预览 FileCount 应 >0，实际 %d", res.FileCount)
	}
	if _, err := os.Stat(res.Archive); err == nil {
		t.Fatalf("dry-run 不应写盘，但归档存在: %s", res.Archive)
	}
	if _, err := os.Stat(res.Checksum); err == nil {
		t.Fatalf("dry-run 不应写盘，但校验和存在: %s", res.Checksum)
	}
}

func TestRunNoManifest(t *testing.T) {
	root := t.TempDir()
	// 仅放置非 manifest 文件
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
	_, err := Run(Options{Dir: root, Format: FormatTarGz})
	if err == nil {
		t.Fatal("无 manifest 应返回错误")
	}
	if !IsUsageError(err) {
		t.Fatalf("无 manifest 应为 UsageError，实际: %T (%v)", err, err)
	}
}

func TestParseFormat(t *testing.T) {
	cases := []struct {
		in   string
		want Format
		err  bool
	}{
		{"tar.gz", FormatTarGz, false},
		{"tgz", FormatTarGz, false},
		{"", FormatTarGz, false},
		{"zip", FormatZip, false},
		{"rar", "", true},
	}
	for _, c := range cases {
		got, err := ParseFormat(c.in)
		if c.err {
			if err == nil {
				t.Errorf("ParseFormat(%q) 期望错误", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseFormat(%q) 意外错误: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ParseFormat(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
