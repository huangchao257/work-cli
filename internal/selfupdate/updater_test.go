package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestExtractFromTarGz(t *testing.T) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	// 构造看起来像 ELF 二进制的有效 payload（0x7F + "ELF" 前缀）
	payload := []byte("\x7FELFbinary-data-for-test")
	if err := tw.WriteHeader(&tar.Header{
		Name: "work",
		Mode: 0o755,
		Size: int64(len(payload)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := extractFromTarGz(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\x7FELFbinary-data-for-test" {
		t.Fatalf("got %q", got)
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "work")
	// 写入一个看起来像 ELF 的有效旧二进制
	if err := os.WriteFile(dest, []byte("\x7FELFold"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 新数据也是有效二进制（ELF magic）
	if err := replaceExecutable(dest, []byte("\x7FELFnew")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "\x7FELFnew" {
		t.Fatalf("got %q", data)
	}
}

func TestUpgradeDryRun(t *testing.T) {
	assetData := buildTarGz(t, []byte("new-binary"))
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		t.Skip("zip extraction test covered separately on windows CI")
	}
	assetName := fmt.Sprintf("work_2.0.0_%s_%s.%s", runtime.GOOS, runtime.GOARCH, ext)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/test/work-cli/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v2.0.0",
				"assets": []map[string]string{
					{
						"name":                 assetName,
						"browser_download_url": "http://" + r.Host + "/asset",
					},
				},
			})
		case r.URL.Path == "/asset":
			_, _ = w.Write(assetData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	u := NewUpdater("v1.0.0")
	u.Repo = "test/work-cli"
	u.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	dir := t.TempDir()
	exe := filepath.Join(dir, "work")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	u.Executable = func() (string, error) { return exe, nil }

	res, err := u.Upgrade(context.Background(), UpgradeOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.UpdateAvailable {
		t.Fatal("expected update available")
	}
	data, _ := os.ReadFile(exe)
	if string(data) != "old" {
		t.Fatal("dry-run should not replace binary")
	}
}

func TestCheckLatest(t *testing.T) {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	assetName := fmt.Sprintf("work_2.0.0_%s_%s.%s", runtime.GOOS, runtime.GOARCH, ext)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v2.0.0",
			"assets": []map[string]string{
				{
					"name":                 assetName,
					"browser_download_url": "http://example.com/asset",
				},
			},
		})
	}))
	defer srv.Close()

	u := NewUpdater("v1.0.0")
	u.Repo = "test/work-cli"
	u.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	res, err := u.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.UpdateAvailable || res.Latest != "v2.0.0" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func buildTarGz(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	if err := tw.WriteHeader(&tar.Header{Name: "work", Mode: 0o755, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDownloadAssetRejectsNonHTTPS(t *testing.T) {
	_, err := downloadAsset(context.Background(), http.DefaultClient, "http://example.com/work.tar.gz")
	if err == nil {
		t.Fatal("non-HTTPS asset URL should be rejected")
	}
}

func TestDownloadAssetRejectsOversizeContentLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(maxAssetSize+1))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := downloadAsset(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("oversized asset should be rejected")
	}
}
func TestDownloadAsset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "\x7FELFpayload")
	}))
	defer srv.Close()

	data, err := downloadAsset(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "\x7FELFpayload" {
		t.Fatalf("got %q", data)
	}
}

// newUpgradeTestServer 构造一个模拟 GitHub 的测试服务器：
// 提供 releases/latest 元数据、安装包与 checksums.txt 下载端点。
// checksums 内容可为空字符串（表示不提供校验文件下载），
// includeChecksumAsset 控制元数据中是否声明 checksums.txt 资产。
func newUpgradeTestServer(t *testing.T, assetData []byte, assetName, checksums string, includeChecksumAsset bool) *httptest.Server {
	t.Helper()
	var srvPtr *httptest.Server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/test/work-cli/releases/latest":
			assets := []map[string]string{
				{
					"name":                 assetName,
					"browser_download_url": "http://" + srvPtr.Listener.Addr().String() + "/asset",
				},
			}
			if includeChecksumAsset {
				assets = append(assets, map[string]string{
					"name":                 checksumsAssetName,
					"browser_download_url": "http://" + srvPtr.Listener.Addr().String() + "/checksums.txt",
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v2.0.0",
				"assets":   assets,
			})
		case r.URL.Path == "/asset":
			_, _ = w.Write(assetData)
		case r.URL.Path == "/checksums.txt":
			_, _ = io.WriteString(w, checksums)
		default:
			http.NotFound(w, r)
		}
	}))
	srvPtr = srv
	t.Cleanup(srv.Close)
	return srv
}

func newTestUpdater(srv *httptest.Server) *Updater {
	addr := srv.Listener.Addr().String()
	u := NewUpdater("v1.0.0")
	u.Repo = "test/work-cli"
	u.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = addr
			return http.DefaultTransport.RoundTrip(req)
		}),
	}
	return u
}

// runUpgradeAttempt 执行一次真实（非 dry-run）升级，返回错误。
func runUpgradeAttempt(t *testing.T, u *Updater) error {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "work")
	if err := os.WriteFile(exe, []byte("\x7FELFold"), 0o755); err != nil {
		t.Fatal(err)
	}
	u.Executable = func() (string, error) { return exe, nil }
	_, err := u.Upgrade(context.Background(), UpgradeOptions{})
	// 返回前读回文件内容由调用方判断；此处仅转发错误
	return err
}

// buildPlatformArchive 构造当前平台资产归档（windows 用 zip，其它用 tar.gz）。
func buildPlatformArchive(t *testing.T, payload []byte) (data []byte, assetName string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		fw, err := zw.Create("work.exe")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes(), fmt.Sprintf("work_2.0.0_%s_%s.zip", runtime.GOOS, runtime.GOARCH)
	}
	return buildTarGz(t, payload), fmt.Sprintf("work_2.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
}

func TestUpgradeVerifiesAssetChecksumMatch(t *testing.T) {
	payload := []byte("\x7FELFnew-binary")
	assetData, assetName := buildPlatformArchive(t, payload)
	sum := fmt.Sprintf("%x", sha256.Sum256(assetData))
	checksums := sum + "  " + assetName + "\n" +
		fmt.Sprintf("%x", sha256.Sum256([]byte("other"))) + "  other.txt\n"

	srv := newUpgradeTestServer(t, assetData, assetName, checksums, true)
	u := newTestUpdater(srv)

	dir := t.TempDir()
	exe := filepath.Join(dir, "work")
	if err := os.WriteFile(exe, []byte("\x7FELFold"), 0o755); err != nil {
		t.Fatal(err)
	}
	u.Executable = func() (string, error) { return exe, nil }

	res, err := u.Upgrade(context.Background(), UpgradeOptions{})
	if err != nil {
		t.Fatalf("校验和匹配时升级应成功: %v", err)
	}
	if !res.UpdateAvailable || res.Latest != "v2.0.0" {
		t.Fatalf("unexpected result: %+v", res)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("二进制应被替换，got %q", data)
	}
}

func TestUpgradeRejectsChecksumMismatch(t *testing.T) {
	assetData, assetName := buildPlatformArchive(t, []byte("\x7FELFnew-binary"))
	// 故意写错的摘要
	checksums := strings.Repeat("0", 64) + "  " + assetName + "\n"

	srv := newUpgradeTestServer(t, assetData, assetName, checksums, true)
	u := newTestUpdater(srv)

	dir := t.TempDir()
	exe := filepath.Join(dir, "work")
	if err := os.WriteFile(exe, []byte("\x7FELFold"), 0o755); err != nil {
		t.Fatal(err)
	}
	u.Executable = func() (string, error) { return exe, nil }

	_, err := u.Upgrade(context.Background(), UpgradeOptions{})
	if err == nil {
		t.Fatal("校验和不匹配时应拒绝更新")
	}
	if !strings.Contains(err.Error(), "校验和不匹配") {
		t.Fatalf("错误应说明校验和不匹配: %v", err)
	}
	data, _ := os.ReadFile(exe)
	if string(data) != "\x7FELFold" {
		t.Fatal("校验失败时不应替换二进制")
	}
}

func TestUpgradeRejectsMissingChecksumEntry(t *testing.T) {
	assetData, assetName := buildPlatformArchive(t, []byte("\x7FELFnew-binary"))
	// checksums.txt 存在但没有该资产的条目
	checksums := fmt.Sprintf("%x", sha256.Sum256([]byte("other"))) + "  other.txt\n"

	srv := newUpgradeTestServer(t, assetData, assetName, checksums, true)
	u := newTestUpdater(srv)

	err := runUpgradeAttempt(t, u)
	if err == nil {
		t.Fatal("缺少条目时应拒绝更新")
	}
	if !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("错误应说明缺少条目: %v", err)
	}
}

func TestUpgradeRejectsMissingChecksumsAsset(t *testing.T) {
	assetData, assetName := buildPlatformArchive(t, []byte("\x7FELFnew-binary"))

	// Release 元数据中不声明 checksums.txt 资产
	srv := newUpgradeTestServer(t, assetData, assetName, "", false)
	u := newTestUpdater(srv)

	err := runUpgradeAttempt(t, u)
	if err == nil {
		t.Fatal("Release 缺少 checksums.txt 资产时应拒绝更新")
	}
	if !strings.Contains(err.Error(), checksumsAssetName) {
		t.Fatalf("错误应提到 checksums.txt: %v", err)
	}
}

func TestFindChecksumEntry(t *testing.T) {
	assetName := "work_2.0.0_linux_amd64.tar.gz"
	sum := strings.Repeat("ab", 32)
	// 两个空格的标准 goreleaser 格式 + 末尾无换行 + 干扰行
	body := "0000  other.txt\n" + sum + "  " + assetName
	got, err := findChecksumEntry([]byte(body), assetName)
	if err != nil {
		t.Fatal(err)
	}
	if got != sum {
		t.Fatalf("got %q", got)
	}

	if _, err := findChecksumEntry([]byte("0000  other.txt\n"), assetName); err == nil {
		t.Fatal("缺少条目应报错")
	}
}
