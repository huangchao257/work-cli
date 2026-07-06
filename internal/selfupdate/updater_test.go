package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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
