package source

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/huangchao257/work-cli/internal/configcache"
	"github.com/huangchao257/work-cli/internal/platform"
)

type RegistryConfig struct {
	URL string `yaml:"url"`
}

type UserConfig struct {
	Registry RegistryConfig `yaml:"registry"`
	Cache    struct {
		Dir string `yaml:"dir"`
	} `yaml:"cache"`
}

type registryResponse struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum"`
	Ref         string `json:"ref"`
	Subdir      string `json:"subdir"`
}

func LoadUserConfig() (*UserConfig, error) {
	path, err := platform.ConfigFilePath()
	if err != nil {
		return nil, err
	}
	data, err := configcache.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &UserConfig{}, nil
		}
		return nil, fmt.Errorf("读取用户配置失败: %w", err)
	}
	var cfg UserConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析用户配置失败: %w", err)
	}
	return &cfg, nil
}

func CacheDir(cfg *UserConfig) (string, error) {
	if cfg != nil && strings.TrimSpace(cfg.Cache.Dir) != "" {
		return expandHome(cfg.Cache.Dir)
	}
	base, err := platform.WorkConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "cache"), nil
}

func ResolveRegistry(name string, cfg *UserConfig) (string, error) {
	if cfg == nil || strings.TrimSpace(cfg.Registry.URL) == "" {
		return "", fmt.Errorf("未配置 registry.url，请在 ~/.work/config.yaml 中设置")
	}
	// 元数据请求本身必须走 HTTPS，否则 download_url 可被明文掉包。
	base := strings.TrimSpace(cfg.Registry.URL)
	if !strings.HasPrefix(base, "https://") {
		return "", fmt.Errorf("registry.url 必须使用 HTTPS")
	}
	url := strings.TrimRight(base, "/") + "/bundles/" + name + "/latest"
	client := newSecureHTTPClient(60 * time.Second)
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("请求 Registry 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Registry 返回错误: %s", resp.Status)
	}
	var meta registryResponse
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return "", fmt.Errorf("解析 Registry 响应失败: %w", err)
	}
	cache, err := CacheDir(cfg)
	if err != nil {
		return "", err
	}
	switch meta.Type {
	case "git":
		return resolveGit(meta, cache)
	case "maven":
		return resolveMaven(meta, cfg.Registry.URL, cache)
	case "":
		return resolveHTTP(meta, cache)
	default:
		return "", fmt.Errorf("不支持的 registry 类型: %q", meta.Type)
	}
}

func resolveHTTP(meta registryResponse, cache string) (string, error) {
	if meta.Version == "" {
		return "", fmt.Errorf("registry 响应缺少 version 字段")
	}
	if err := validatePathComponent(meta.Name); err != nil {
		return "", fmt.Errorf("registry 返回非法 bundle 名称: %w", err)
	}
	if err := validatePathComponent(meta.Version); err != nil {
		return "", fmt.Errorf("registry 返回非法版本号: %w", err)
	}
	dest := filepath.Join(cache, "registry", meta.Name, meta.Version)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	// 前置校验校验和非空：避免在缺失完整性的情况下下载（浪费带宽且内容不可信）。
	if strings.TrimSpace(meta.Checksum) == "" {
		return "", fmt.Errorf("registry 响应缺少 checksum（sha256），拒绝安装")
	}
	// 任一步失败都清理 dest：半成品目录一旦残留，上方 Stat 会永久命中，
	// 该版本被空目录污染且无法自愈（DetectKind 找不到 manifest）。
	if err := func() (err error) {
		defer func() {
			if err != nil {
				_ = os.RemoveAll(dest)
			}
		}()
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return fmt.Errorf("创建缓存目录失败: %w", err)
		}
		zipPath := filepath.Join(cache, "registry", meta.Name, meta.Version+".zip")
		if err := downloadFile(meta.DownloadURL, zipPath); err != nil {
			return fmt.Errorf("下载归档失败: %w", err)
		}
		// 校验和必填：无 sha256 校验和则拒绝安装，避免 MITM/恶意元数据掉包后的内容零完整性验证。
		if err := verifyChecksumRequired(zipPath, meta.Checksum); err != nil {
			return err
		}
		if err := unzip(zipPath, dest); err != nil {
			return fmt.Errorf("解压归档失败: %w", err)
		}
		return nil
	}(); err != nil {
		return "", err
	}
	return dest, nil
}

func resolveGit(meta registryResponse, cache string) (string, error) {
	if meta.Version == "" {
		return "", fmt.Errorf("git 类型缺少 version 字段")
	}
	if meta.DownloadURL == "" {
		return "", fmt.Errorf("git 类型缺少 download_url 字段")
	}
	if err := validatePathComponent(meta.Name); err != nil {
		return "", fmt.Errorf("registry 返回非法 bundle 名称: %w", err)
	}
	if err := validatePathComponent(meta.Version); err != nil {
		return "", fmt.Errorf("registry 返回非法版本号: %w", err)
	}
	dest := filepath.Join(cache, "registry", meta.Name, meta.Version)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	ref := meta.Ref
	if ref == "" {
		ref = meta.Version
	}
	cloneDir, err := ResolveGit(meta.DownloadURL, ref, cache)
	if err != nil {
		return "", fmt.Errorf("git clone 失败: %w", err)
	}
	if meta.Subdir != "" {
		src := filepath.Join(cloneDir, meta.Subdir)
		if !strings.HasPrefix(filepath.Clean(src), filepath.Clean(cloneDir)+string(os.PathSeparator)) {
			return "", fmt.Errorf("子目录 %q 试图逃逸仓库目录", meta.Subdir)
		}
		if _, err := os.Stat(src); err != nil {
			return "", fmt.Errorf("子目录 %q 不存在: %w", meta.Subdir, err)
		}
		return dest, copyDir(src, dest)
	}
	return dest, copyDir(cloneDir, dest)
}

func expandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func validatePathComponent(s string) error {
	if s == "" {
		return fmt.Errorf("路径组件不能为空")
	}
	if strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return fmt.Errorf("路径组件不能包含分隔符")
	}
	if s == "." || s == ".." {
		return fmt.Errorf("路径组件不能是 . 或 ..")
	}
	return nil
}

func downloadFile(url, dest string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("下载 URL 必须使用 HTTPS")
	}
	client := newSecureHTTPClient(120 * time.Second)
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("请求下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: %s", resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("创建下载文件失败: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("写入下载文件失败: %w", err)
	}
	return nil
}

func verifyChecksum(path, checksum string) error {
	parts := strings.SplitN(checksum, ":", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return fmt.Errorf("不支持的校验算法，仅支持 sha256")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取校验文件失败: %w", err)
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != parts[1] {
		return fmt.Errorf("checksum 不匹配")
	}
	return nil
}

// verifyChecksumRequired 校验下载产物的 sha256 校验和；校验和缺失或为空时报错。
func verifyChecksumRequired(path, checksum string) error {
	if strings.TrimSpace(checksum) == "" {
		return fmt.Errorf("registry 响应缺少 checksum（sha256），拒绝安装")
	}
	if err := verifyChecksum(path, checksum); err != nil {
		return fmt.Errorf("校验和不匹配: %w", err)
	}
	return nil
}

// newSecureHTTPClient 返回一个禁止跨协议降级重定向的 HTTP 客户端。
// 防止恶意 Registry 将 https 的 download_url 302 到 http://（含内网/元数据服务）
// 从而绕过 downloadFile 入口的 "必须 HTTPS" 检查。本项目面向内网 Registry，
// 故不拒私网地址，仅禁止 https→http 降级。
func newSecureHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("重定向次数过多")
			}
			if req.URL.Scheme != "https" {
				return fmt.Errorf("拒绝重定向到非 HTTPS 协议: %s", req.URL.Scheme)
			}
			return nil
		},
	}
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("打开 zip 失败: %w", err)
	}
	defer r.Close()
	for _, f := range r.File {
		// 拒绝符号链接，防止通过 zip 中的 symlink 逃逸目标目录
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip 中的符号链接不被允许: %s", f.Name)
		}
		target := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("非法 zip 路径")
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("创建父目录失败: %w", err)
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("打开 zip 条目失败: %w", err)
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return fmt.Errorf("创建文件失败: %w", err)
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if err != nil {
			return fmt.Errorf("解压文件失败: %w", err)
		}
	}
	return nil
}

func copyDir(src, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dest, rel)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("非法路径: %s", rel)
		}
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			in.Close()
			return fmt.Errorf("创建父目录失败: %w", err)
		}
		out, err := os.Create(target)
		if err != nil {
			in.Close()
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
		return err
	})
}
