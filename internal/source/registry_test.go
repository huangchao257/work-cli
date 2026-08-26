package source

import (
	"archive/zip"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sha256Of 返回文件内容的 "sha256:<hex>" 校验和字符串。
func sha256Of(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func createZip(t *testing.T, base string, files map[string]string) string {
	t.Helper()
	zipPath := filepath.Join(base, "bundle.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, content := range files {
		cw, err := w.Create(name)
		if err != nil {
			f.Close()
			t.Fatal(err)
		}
		if _, err := cw.Write([]byte(content)); err != nil {
			f.Close()
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	return zipPath
}

func TestResolveHTTPMissingChecksum(t *testing.T) {
	// 校验和必填：缺失时拒绝安装（在缓存命中之前校验）。
	_, err := resolveHTTP(registryResponse{Name: "test", Version: "1.0.0"}, t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing checksum")
	}
}

func TestDownloadRejectsNonHTTPSRedirect(t *testing.T) {
	// 恶意服务器将 https 302 到 http：应被 CheckRedirect 拒绝。
	tmp := t.TempDir()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer redirect.Close()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirect.URL, http.StatusFound)
	}))
	defer srv.Close()

	err := downloadFile(srv.URL+"/dl", filepath.Join(tmp, "out"))
	if err == nil {
		t.Fatal("expected redirect-to-http to be rejected")
	}
	if !strings.Contains(err.Error(), "HTTPS") && !strings.Contains(err.Error(), "https") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveHTTP(t *testing.T) {
	tmp := t.TempDir()
	files := map[string]string{
		"installer.yaml": "kind: cli\n",
	}
	zipPath := createZip(t, tmp, files)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, zipPath)
	}))
	defer srv.Close()

	meta := registryResponse{
		Name:        "test-bundle",
		Version:     "1.0.0",
		DownloadURL: srv.URL + "/bundle.zip",
		Checksum:    sha256Of(t, zipPath),
	}
	dest, err := resolveHTTP(meta, tmp)
	if err != nil {
		t.Fatalf("resolveHTTP failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "installer.yaml")); err != nil {
		t.Fatalf("expected installer.yaml in dest: %v", err)
	}
}

func TestResolveHTTPCache(t *testing.T) {
	tmp := t.TempDir()
	meta := registryResponse{
		Name:    "test-bundle",
		Version: "1.0.0",
	}
	dest := filepath.Join(tmp, "registry", meta.Name, meta.Version)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := resolveHTTP(meta, tmp)
	if err != nil {
		t.Fatalf("resolveHTTP cache hit failed: %v", err)
	}
	if result != dest {
		t.Fatalf("expected cached dest %q, got %q", dest, result)
	}
}

func TestResolveHTTPMissingVersion(t *testing.T) {
	_, err := resolveHTTP(registryResponse{Name: "test"}, "")
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestResolveRegistryDefaultHTTP(t *testing.T) {
	tmp := t.TempDir()
	files := map[string]string{"installer.yaml": "kind: cli\n"}
	zipPath := createZip(t, tmp, files)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bundles/test-bundle/latest" {
			json.NewEncoder(w).Encode(registryResponse{
				Name:        "test-bundle",
				Version:     "1.0.0",
				DownloadURL: "https://" + r.Host + "/download/bundle.zip",
				Checksum:    sha256Of(t, zipPath),
			})
			return
		}
		if r.URL.Path == "/download/bundle.zip" {
			http.ServeFile(w, r, zipPath)
			return
		}
	}))
	defer srv.Close()

	cfg := &UserConfig{Registry: RegistryConfig{URL: srv.URL}}
	cfg.Cache.Dir = filepath.Join(tmp, "cache")
	dest, err := ResolveRegistry("test-bundle", cfg)
	if err != nil {
		t.Fatalf("ResolveRegistry failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "installer.yaml")); err != nil {
		t.Fatalf("expected installer.yaml in dest: %v", err)
	}
}

func TestResolveRegistryGitType(t *testing.T) {
	gitPath, ok := exec.LookPath("git")
	if gitPath == "" || ok != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}
	runGit("init")
	runGit("config", "user.email", "test@test")
	runGit("config", "user.name", "test")
	runGit("checkout", "-b", "main")
	subdir := filepath.Join(repoDir, "bundles", "my-bundle")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "installer.yaml"), []byte("kind: cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "-A")
	runGit("commit", "-m", "init")

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(registryResponse{
			Name:        "test-git-bundle",
			Type:        "git",
			Version:     "main",
			DownloadURL: "https://example.com/org/repo.git",
			Subdir:      "bundles/my-bundle",
		})
	}))
	defer srv.Close()

	cacheDir := filepath.Join(tmp, "cache")
	cfg := &UserConfig{Registry: RegistryConfig{URL: srv.URL}}
	cfg.Cache.Dir = cacheDir
	// download_url 为 https 但仓库不可达：应通过协议校验、失败于网络/仓库（而非协议拒绝）。
	_, err := ResolveRegistry("test-git-bundle", cfg)
	if err == nil {
		t.Fatal("expected network/clone error for unreachable https repo")
	}
	if strings.Contains(err.Error(), "必须使用 HTTPS") {
		t.Fatalf("https URL 不应被协议校验拒绝: %v", err)
	}
}

func TestResolveRegistryMavenType(t *testing.T) {
	tmp := t.TempDir()
	files := map[string]string{"installer.yaml": "kind: cli\n"}
	jarPath := createZip(t, tmp, files)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bundles/test-maven/latest" {
			json.NewEncoder(w).Encode(registryResponse{
				Name:        "test-maven",
				Type:        "maven",
				Version:     "2.0.0",
				DownloadURL: "com.example:test-bundle:2.0.0",
				Checksum:    sha256Of(t, jarPath),
			})
			return
		}
		if r.URL.Path == "/com/example/test-bundle/2.0.0/test-bundle-2.0.0.jar" {
			http.ServeFile(w, r, jarPath)
			return
		}
	}))
	defer srv.Close()

	cacheDir := filepath.Join(tmp, "cache")
	cfg := &UserConfig{Registry: RegistryConfig{URL: srv.URL}}
	cfg.Cache.Dir = cacheDir
	dest, err := ResolveRegistry("test-maven", cfg)
	if err != nil {
		t.Fatalf("ResolveRegistry maven type failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "installer.yaml")); err != nil {
		t.Fatalf("expected installer.yaml in dest: %v", err)
	}
}

func TestResolveRegistryInvalidType(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(registryResponse{
			Name:    "test",
			Version: "1.0.0",
			Type:    "ftp",
		})
	}))
	defer srv.Close()

	cfg := &UserConfig{Registry: RegistryConfig{URL: srv.URL}}
	_, err := ResolveRegistry("test", cfg)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestResolveMavenInvalidCoords(t *testing.T) {
	_, err := resolveMaven(registryResponse{Name: "test", Version: "1.0", DownloadURL: "bad"}, "", t.TempDir())
	if err == nil {
		t.Fatal("expected error for invalid maven coords")
	}
}

func TestResolveMavenMissingVersion(t *testing.T) {
	_, err := resolveMaven(registryResponse{Name: "test"}, "", t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestResolveGitMissingVersion(t *testing.T) {
	_, err := resolveGit(registryResponse{Name: "test"}, t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestResolveGitMissingDownloadURL(t *testing.T) {
	_, err := resolveGit(registryResponse{Name: "test", Version: "v1"}, t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing download_url")
	}
}

func TestResolveGitNoSubdir(t *testing.T) {
	gitPath, ok := exec.LookPath("git")
	if gitPath == "" || ok != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}
	runGit("init")
	runGit("config", "user.email", "test@test")
	runGit("config", "user.name", "test")
	runGit("checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoDir, "installer.yaml"), []byte("kind: cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "-A")
	runGit("commit", "-m", "init")

	cacheDir := filepath.Join(tmp, "cache")
	meta := registryResponse{
		Name:        "test-git",
		Version:     "main",
		DownloadURL: repoDir,
	}
	// 本地路径 URL 必须被拒绝（仅允许 https），不应触发 clone。
	if _, err := resolveGit(meta, cacheDir); err == nil {
		t.Fatal("expected local-path git URL to be rejected")
	}
}

func TestResolveGitRejectsNonHTTPS(t *testing.T) {
	// 即使 git 可用，非 https 协议也必须被拒绝，防止 ext:: / file:// / ssh:// 注入。
	for _, u := range []string{
		"ext::sh -c touch /tmp/pwned",
		"file:///etc",
		"ssh://git@example.com/repo",
		"http://example.com/repo",
	} {
		if _, err := ResolveGit(u, "main", t.TempDir()); err == nil {
			t.Fatalf("expected rejection for URL %q", u)
		}
	}
}

func TestResolveGitRejectsControlChars(t *testing.T) {
	if _, err := ResolveGit("https://example.com/repo", "main\nrm -rf /", t.TempDir()); err == nil {
		t.Fatal("expected rejection for ref with control chars")
	}
}

func TestCopyDir(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}
	for _, f := range []string{"a.txt", "sub/b.txt"} {
		if _, err := os.Stat(filepath.Join(dst, f)); err != nil {
			t.Fatalf("expected %s in dst: %v", f, err)
		}
	}
}

// init overrides HTTP transport for tests that use httptest.NewTLSServer
func init() {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	http.DefaultClient = &http.Client{Transport: tr}
	http.DefaultTransport = tr
}
