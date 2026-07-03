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
	Name         string `json:"name"`
	Type         string `json:"type"`
	Version      string `json:"version"`
	DownloadURL  string `json:"download_url"`
	Checksum     string `json:"checksum"`
}

func LoadUserConfig() (*UserConfig, error) {
	path, err := platform.ConfigFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &UserConfig{}, nil
		}
		return nil, err
	}
	var cfg UserConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
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
	url := strings.TrimRight(cfg.Registry.URL, "/") + "/bundles/" + name + "/latest"
	client := &http.Client{Timeout: 60 * time.Second}
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
		return "", err
	}
	cache, err := CacheDir(cfg)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(cache, "registry", name, meta.Version)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	zipPath := filepath.Join(cache, "registry", name, meta.Version+".zip")
	if err := downloadFile(meta.DownloadURL, zipPath); err != nil {
		return "", err
	}
	if meta.Checksum != "" {
		if err := verifyChecksum(zipPath, meta.Checksum); err != nil {
			return "", err
		}
	}
	if err := unzip(zipPath, dest); err != nil {
		return "", err
	}
	return dest, nil
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

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: %s", resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func verifyChecksum(path, checksum string) error {
	parts := strings.SplitN(checksum, ":", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != parts[1] {
		return fmt.Errorf("checksum 不匹配")
	}
	return nil
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("非法 zip 路径")
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
