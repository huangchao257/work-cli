package publish

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/huangchao257/work-cli/internal/pack"
)

// makePubkitArchive 用 internal/pack 生成一个含 bundle.yaml 的 tar.gz 归档与校验和。
func makePubkitArchive(t *testing.T) (archive, checksum string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "pubkit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	manifest := "name: pubkit\nversion: 0.2.0\ntype: bundle\ndescription: test publish kit\n"
	if err := os.WriteFile(filepath.Join(dir, "bundle.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("写入 manifest 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill.md"), []byte("# pubkit\n"), 0o644); err != nil {
		t.Fatalf("写入资源失败: %v", err)
	}
	res, err := pack.Run(pack.Options{Dir: dir, Format: pack.FormatTarGz})
	if err != nil {
		t.Fatalf("pack.Run 失败: %v", err)
	}
	return res.Archive, res.Checksum
}

// writeTarGzWithFiles 用标准库手工构造一个 tar.gz，包含给定文件（路径→内容）。
func writeTarGzWithFiles(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	// 稳定顺序
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		content := files[n]
		hdr := &tar.Header{
			Name: n,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return err
		}
	}
	return nil
}

func TestInspectArchive_TarGz(t *testing.T) {
	archive, _ := makePubkitArchive(t)
	name, version, typ, err := InspectArchive(archive)
	if err != nil {
		t.Fatalf("InspectArchive 失败: %v", err)
	}
	if name != "pubkit" {
		t.Errorf("name = %q, want pubkit", name)
	}
	if version != "0.2.0" {
		t.Errorf("version = %q, want 0.2.0", version)
	}
	if typ != "bundle" {
		t.Errorf("type = %q, want bundle", typ)
	}
}

func TestInspectArchive_Zip(t *testing.T) {
	// 构造一个 zip 归档（复用 pack 的 zip 路径）
	root := t.TempDir()
	dir := filepath.Join(root, "pubkit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	manifest := "name: pubkit\nversion: 0.2.0\n"
	if err := os.WriteFile(filepath.Join(dir, "bundle.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("写入 manifest 失败: %v", err)
	}
	res, err := pack.Run(pack.Options{Dir: dir, Format: pack.FormatZip})
	if err != nil {
		t.Fatalf("pack.Run 失败: %v", err)
	}
	name, version, typ, err := InspectArchive(res.Archive)
	if err != nil {
		t.Fatalf("InspectArchive 失败: %v", err)
	}
	if name != "pubkit" || version != "0.2.0" || typ != "bundle" {
		t.Errorf("got %s/%s/%s, want pubkit/0.2.0/bundle", name, version, typ)
	}
}

func TestInspectArchive_NoManifest(t *testing.T) {
	// 手工构造一个不含 manifest 的 tar.gz
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "plain.tar.gz")
	if err := writeTarGzWithFiles(archive, map[string]string{"readme.txt": "x"}); err != nil {
		t.Fatalf("构造归档失败: %v", err)
	}
	if _, _, _, err := InspectArchive(archive); err == nil {
		t.Fatal("无 manifest 应返回错误，实际 nil")
	}
}

func TestInspectArchive_UnsupportedFormat(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "foo.rar")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
	if _, _, _, err := InspectArchive(p); err == nil {
		t.Fatal("不支持的格式应返回错误")
	}
}

func TestVerifyChecksumFile_Mismatch(t *testing.T) {
	archive, _ := makePubkitArchive(t)
	badChecksum := archive + ".bad.sha256"
	// 写一个错误的 sha256 内容
	if err := os.WriteFile(badChecksum, []byte("0000000000000000000000000000000000000000000000000000000000000000  "+filepath.Base(archive)+"\n"), 0o644); err != nil {
		t.Fatalf("写入校验和失败: %v", err)
	}
	err := verifyChecksumFile(archive, badChecksum)
	if err == nil {
		t.Fatal("校验和不匹配应返回错误")
	}
	var ce *checksumError
	if !errors.As(err, &ce) {
		t.Fatalf("期望 *checksumError，实际 %T", err)
	}
}

func TestRun_DryRun_EmptyRegistryURL_UsageError(t *testing.T) {
	archive, checksum := makePubkitArchive(t)
	_, err := Run(Options{
		Archive:     archive,
		Checksum:    checksum,
		DryRun:      true,
		RegistryURL: "",
	})
	if err == nil {
		t.Fatal("空 RegistryURL 应返回错误")
	}
	if !IsUsageError(err) {
		t.Fatalf("空 RegistryURL 应为 UsageError（exit 2），实际 %T: %v", err, err)
	}
}

func TestRun_DryRun_WithRegistry(t *testing.T) {
	archive, checksum := makePubkitArchive(t)
	res, err := Run(Options{
		Archive:     archive,
		Checksum:    checksum,
		DryRun:      true,
		RegistryURL: "https://registry.example.com/",
	})
	if err != nil {
		t.Fatalf("dry-run 失败: %v", err)
	}
	if res.Name != "pubkit" || res.Version != "0.2.0" || res.Type != "bundle" {
		t.Errorf("推断结果异常: %+v", res)
	}
	if res.URL != "https://registry.example.com/bundles" {
		t.Errorf("URL 归一化错误: got %s", res.URL)
	}
}

func TestRun_ArchiveNotExist_UsageError(t *testing.T) {
	_, err := Run(Options{Archive: "/nonexistent/archive.tar.gz"})
	if !IsUsageError(err) {
		t.Fatalf("归档不存在应为 UsageError，实际 %T: %v", err, err)
	}
}

func TestRun_ChecksumNotExist_UsageError(t *testing.T) {
	archive, _ := makePubkitArchive(t)
	_, err := Run(Options{
		Archive:     archive,
		Checksum:    archive + ".missing.sha256",
		RegistryURL: "https://registry.example.com",
	})
	if !IsUsageError(err) {
		t.Fatalf("校验和不存在应为 UsageError，实际 %T: %v", err, err)
	}
}

func newMockRegistry(t *testing.T, status int, checkMultipart bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bundles" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if checkMultipart {
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Errorf("ParseMultipartForm 失败: %v", err)
				http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
				return
			}
			if r.FormValue("name") != "pubkit" {
				http.Error(w, "bad name", http.StatusBadRequest)
				return
			}
			if r.FormValue("version") != "0.2.0" {
				http.Error(w, "bad version", http.StatusBadRequest)
				return
			}
			if r.FormValue("type") != "bundle" {
				http.Error(w, "bad type", http.StatusBadRequest)
				return
			}
			if _, _, err := r.FormFile("archive"); err != nil {
				t.Errorf("缺少 archive 字段: %v", err)
				http.Error(w, "no archive", http.StatusBadRequest)
				return
			}
			if _, _, err := r.FormFile("checksum"); err != nil {
				t.Errorf("缺少 checksum 字段: %v", err)
				http.Error(w, "no checksum", http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, fmt.Sprintf(`{"status":%d}`, status))
	}))
}

func TestRun_Upload_Success(t *testing.T) {
	archive, checksum := makePubkitArchive(t)
	ts := newMockRegistry(t, http.StatusCreated, true)
	defer ts.Close()

	res, err := Run(Options{
		Archive:     archive,
		Checksum:    checksum,
		RegistryURL: ts.URL,
	})
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	if res.URL != ts.URL+"/bundles" {
		t.Errorf("URL = %s, want %s/bundles", res.URL, ts.URL)
	}
	if res.Name != "pubkit" || res.Version != "0.2.0" || res.Type != "bundle" {
		t.Errorf("推断结果异常: %+v", res)
	}
}

func TestRun_Upload_4xx_NonUsageError(t *testing.T) {
	archive, checksum := makePubkitArchive(t)
	ts := newMockRegistry(t, http.StatusBadRequest, false)
	defer ts.Close()

	_, err := Run(Options{
		Archive:     archive,
		Checksum:    checksum,
		RegistryURL: ts.URL,
	})
	if err == nil {
		t.Fatal("4xx 应返回错误")
	}
	if IsUsageError(err) {
		t.Fatalf("4xx 不应为 UsageError（应为 exit 1），实际: %v", err)
	}
	if !strings.Contains(err.Error(), "Registry 拒绝上传") {
		t.Errorf("错误信息异常: %v", err)
	}
}

func TestRun_Upload_5xx_NonUsageError(t *testing.T) {
	archive, checksum := makePubkitArchive(t)
	ts := newMockRegistry(t, http.StatusInternalServerError, false)
	defer ts.Close()

	_, err := Run(Options{
		Archive:     archive,
		Checksum:    checksum,
		RegistryURL: ts.URL,
	})
	if err == nil {
		t.Fatal("5xx 应返回错误")
	}
	if IsUsageError(err) {
		t.Fatalf("5xx 不应为 UsageError（应为 exit 1），实际: %v", err)
	}
}

func TestRun_Upload_NetworkError(t *testing.T) {
	archive, checksum := makePubkitArchive(t)
	// 起一个立即关闭的服务器模拟连接失败
	ts := newMockRegistry(t, http.StatusOK, false)
	ts.Close()

	_, err := Run(Options{
		Archive:     archive,
		Checksum:    checksum,
		RegistryURL: ts.URL,
	})
	if err == nil {
		t.Fatal("网络错误应返回错误")
	}
	if IsUsageError(err) {
		t.Fatalf("网络错误不应为 UsageError，实际: %v", err)
	}
}
