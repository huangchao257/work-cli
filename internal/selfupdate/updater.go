package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type CheckResult struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	AssetURL        string `json:"asset_url,omitempty"`
	AssetName       string `json:"asset_name,omitempty"`
}

type UpgradeOptions struct {
	Version   string
	DryRun    bool
	CheckOnly bool // 仅检查不下载
	Repo      string
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
	if opts.DryRun || opts.CheckOnly {
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

	// 指数退避重试：最多 3 次，间隔 1s/2s/4s，仅对瞬时错误重试
	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		data, err := downloadAssetOnce(ctx, client, url)
		if err == nil {
			return data, nil
		}
		lastErr = err

		// 非瞬时错误不重试
		if !isTransientError(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("下载安装包失败（已重试 %d 次）: %w", maxRetries, lastErr)
}

// downloadAssetOnce 执行单次下载，带 Content-Length 校验。
func downloadAssetOnce(ctx context.Context, client *http.Client, url string) ([]byte, error) {
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

	// 如果响应包含 Content-Length，用于预分配缓冲区并校验完整性
	var data []byte
	if cl := resp.ContentLength; cl > 0 {
		data = make([]byte, 0, cl)
		buf := make([]byte, 32*1024)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				data = append(data, buf[:n]...)
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return nil, fmt.Errorf("读取下载数据失败: %w", readErr)
			}
		}
		if int64(len(data)) != cl {
			return nil, fmt.Errorf("下载不完整：预期 %d 字节，实际收到 %d 字节", cl, len(data))
		}
		return data, nil
	}

	// 无 Content-Length 时回退到 ReadAll
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取下载数据失败: %w", err)
	}
	return data, nil
}

// isTransientError 判断是否为可重试的瞬时网络错误。
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// 网络超时、连接重置等瞬时错误
	if neterr, ok := err.(net.Error); ok && (neterr.Timeout() || neterr.Temporary()) {
		return true
	}
	transientKeywords := []string{
		"connection reset",
		"connection refused",
		"no such host",
		"tls handshake timeout",
		"i/o timeout",
		"broken pipe",
		"EOF",
		"http2: ",
	}
	lower := strings.ToLower(msg)
	for _, kw := range transientKeywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	// 5xx 服务端错误可重试
	if strings.Contains(msg, "下载失败 (5") {
		return true
	}
	return false
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
	if len(data) == 0 {
		return fmt.Errorf("下载的二进制数据为空")
	}
	// 基本校验：检查 ELF magic（Linux）或 PE magic（Windows）或 Mach-O magic（macOS）
	if !isValidBinary(data) {
		return fmt.Errorf("下载的文件不是有效的可执行二进制")
	}

	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".work-upgrade-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		// 成功时 cleanup=false，失败时清理临时文件——幂等兜底，错误可忽略
		if cleanup {
			_ = os.Remove(tmpName)
		}
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
		return replaceWindows(dest, tmpName)
	}

	// Unix: 先备份原文件，rename 后若失败则回滚
	backup := dest + ".backup"
	// 清理可能残留的旧备份；不存在时 Remove 报错可忽略
	_ = os.Remove(backup)
	if err := os.Rename(dest, backup); err != nil {
		return fmt.Errorf("备份当前二进制失败: %w", err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		// 回滚：把备份恢复回去
		if rerr := os.Rename(backup, dest); rerr != nil {
			return fmt.Errorf("替换二进制失败且回滚也失败（备份位于 %s）: %w", backup, err)
		}
		return fmt.Errorf("替换二进制失败，已回滚: %w", err)
	}
	// 成功后清理备份；失败不影响结果
	_ = os.Remove(backup)
	cleanup = false
	return nil
}

func replaceWindows(dest, tmpName string) error {
	backup := dest + ".old"
	// 清理可能残留的旧备份；不存在时 Remove 报错可忽略
	_ = os.Remove(backup)
	if err := os.Rename(dest, backup); err != nil {
		return fmt.Errorf("Windows 下替换二进制失败，请关闭占用进程后重试: %w", err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		// 回滚：把备份恢复回去
		if rerr := os.Rename(backup, dest); rerr != nil {
			return fmt.Errorf("安装新版本失败且回滚也失败（备份位于 %s）: %w", backup, err)
		}
		return fmt.Errorf("安装新版本失败，已回滚: %w", err)
	}
	// 成功后清理备份；失败不影响结果
	_ = os.Remove(backup)
	return nil
}

// isValidBinary 检查数据是否为可执行文件（ELF/PE/Mach-O magic）。
func isValidBinary(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	// ELF magic: 0x7F 'E' 'L' 'F'
	if data[0] == 0x7F && data[1] == 'E' && data[2] == 'L' && data[3] == 'F' {
		return true
	}
	// PE magic: 'M' 'Z'
	if data[0] == 'M' && data[1] == 'Z' {
		return true
	}
	// Mach-O magic: 0xFEEDFACE / 0xFEEDFACF (64-bit) / 0xCEFAEDFE / 0xCFFAEDFE
	if len(data) >= 4 {
		magic := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
		switch magic {
		case 0xFEEDFACE, 0xFEEDFACF, 0xCEFAEDFE, 0xCFFAEDFE:
			return true
		}
	}
	// fat binary: 0xCAFEBABE / 0xBEBAFECA
	if len(data) >= 4 {
		magic := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
		switch magic {
		case 0xCAFEBABE, 0xBEBAFECA:
			return true
		}
	}
	return false
}
