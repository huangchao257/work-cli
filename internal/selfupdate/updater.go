package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type CheckResult struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	AssetURL        string `json:"asset_url,omitempty"`
	AssetName       string `json:"asset_name,omitempty"`
}

type UpgradeOptions struct {
	Version string
	DryRun  bool
	Repo    string
}

type Updater struct {
	CurrentVersion string
	Repo           string
	HTTPClient     *http.Client
	Executable     func() (string, error)
}

func NewUpdater(currentVersion string) *Updater {
	return &Updater{
		CurrentVersion: currentVersion,
		Repo:           DefaultRepo,
		HTTPClient:     http.DefaultClient,
		Executable:     os.Executable,
	}
}

func (u *Updater) repo() string {
	if strings.TrimSpace(u.Repo) == "" {
		return DefaultRepo
	}
	return u.Repo
}

func (u *Updater) Check(ctx context.Context) (*CheckResult, error) {
	info, err := fetchLatestRelease(ctx, u.HTTPClient, u.repo())
	if err != nil {
		return nil, fmt.Errorf("查询最新版本失败: %w", err)
	}
	asset, err := resolveAsset(info, "")
	if err != nil {
		return nil, fmt.Errorf("解析下载资产失败: %w", err)
	}
	latest := normalizeTag(info.TagName)
	current := normalizeTag(u.CurrentVersion)
	return &CheckResult{
		Current:         current,
		Latest:          latest,
		UpdateAvailable: CompareVersions(current, latest) < 0,
		AssetURL:        asset.URL,
		AssetName:       asset.Name,
	}, nil
}

func (u *Updater) Upgrade(ctx context.Context, opts UpgradeOptions) (*CheckResult, error) {
	repo := u.repo()
	if strings.TrimSpace(opts.Repo) != "" {
		repo = opts.Repo
	}

	var info *releaseInfo
	var err error
	if strings.TrimSpace(opts.Version) == "" {
		info, err = fetchLatestRelease(ctx, u.HTTPClient, repo)
	} else {
		info, err = fetchReleaseByTag(ctx, u.HTTPClient, repo, opts.Version)
	}
	if err != nil {
		return nil, err
	}

	asset, err := resolveAsset(info, info.TagName)
	if err != nil {
		return nil, fmt.Errorf("解析下载资产失败: %w", err)
	}

	current := normalizeTag(u.CurrentVersion)
	latest := normalizeTag(info.TagName)
	result := &CheckResult{
		Current:         current,
		Latest:          latest,
		UpdateAvailable: CompareVersions(current, latest) < 0,
		AssetURL:        asset.URL,
		AssetName:       asset.Name,
	}

	if !result.UpdateAvailable {
		return result, nil
	}
	if opts.DryRun {
		return result, nil
	}

	exe, err := u.Executable()
	if err != nil {
		return nil, fmt.Errorf("定位当前可执行文件失败: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return nil, fmt.Errorf("解析可执行文件路径失败: %w", err)
	}

	data, err := downloadAsset(ctx, u.HTTPClient, asset.URL)
	if err != nil {
		return nil, fmt.Errorf("下载安装包失败: %w", err)
	}
	binData, err := extractBinary(asset.Name, data)
	if err != nil {
		return nil, fmt.Errorf("解压二进制失败: %w", err)
	}
	if err := replaceExecutable(exe, binData); err != nil {
		return nil, fmt.Errorf("替换可执行文件失败: %w", err)
	}
	return result, nil
}

func normalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "dev"
	}
	if !strings.HasPrefix(tag, "v") && tag != "dev" {
		return "v" + tag
	}
	return tag
}

func downloadAsset(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "work-cli")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载安装包失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("下载失败 (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(resp.Body)
}

func extractBinary(assetName string, data []byte) ([]byte, error) {
	if strings.HasSuffix(assetName, ".zip") {
		return extractFromZip(data)
	}
	return extractFromTarGz(data)
}

func extractFromTarGz(data []byte) ([]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("解压 gzip 失败: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取 tar 失败: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Base(hdr.Name)
		if isWorkBinary(name) {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("压缩包中未找到 work 二进制")
}

func extractFromZip(data []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("打开 zip 失败: %w", err)
	}
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		if !isWorkBinary(name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("压缩包中未找到 work 二进制")
}

func isWorkBinary(name string) bool {
	name = strings.ToLower(name)
	return name == "work" || name == "work.exe"
}

func replaceExecutable(dest string, data []byte) error {
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".work-upgrade-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// 清理临时文件：失败时已通过 Rename 移走，此处为幂等兜底，错误可忽略
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		// 写入失败后关闭临时文件以便 Remove 清理；关闭错误无意义
		_ = tmp.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}

	mode := os.FileMode(0o755)
	if info, err := os.Stat(dest); err == nil {
		mode = info.Mode() & 0o777
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("设置权限失败: %w", err)
	}

	if runtime.GOOS == "windows" {
		backup := dest + ".old"
		// 清理可能残留的旧备份；不存在时 Remove 报错可忽略
		_ = os.Remove(backup)
		if err := os.Rename(dest, backup); err != nil {
			return fmt.Errorf("Windows 下替换二进制失败，请关闭占用进程后重试: %w", err)
		}
		if err := os.Rename(tmpName, dest); err != nil {
			// 回滚：把备份恢复回去；失败只能忽略，原二进制已丢失
			_ = os.Rename(backup, dest)
			return fmt.Errorf("安装新版本失败: %w", err)
		}
		// 成胜后清理备份；失败不影响结果
		_ = os.Remove(backup)
		return nil
	}

	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("替换二进制失败: %w", err)
	}
	return nil
}
