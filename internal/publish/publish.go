// Package publish 将 work pack 产出的归档上传至内部 Registry。
package publish

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	pkgmanifest "github.com/huangchao257/work-cli/internal/pkg/manifest"
	"github.com/huangchao257/work-cli/internal/usage"
	"gopkg.in/yaml.v3"
)

// Options 是 Run 的输入参数。
type Options struct {
	Archive     string // 归档文件路径（.tar.gz/.zip）
	Checksum    string // 校验和文件路径，空表示 <Archive>.sha256
	DryRun      bool   // 仅预览，不发送
	RegistryURL string // Registry 基地址，对应配置 registry.url
}

// Result 是 Run 的输出。
type Result struct {
	URL     string `json:"url"`     // 上传目标 URL
	Name    string `json:"name"`    // 推断的套装名
	Version string `json:"version"` // 推断的版本
	Type    string `json:"type"`    // 推断的类型：bundle/cli/hooks
}

// UsageError 表示用法错误（归档不存在、校验和不一致、未配置 registry.url 等），对应退出码 2。
type UsageError = usage.Error

// IsUsageError 判断 err 是否为用法错误。
var IsUsageError = usage.Is

func usageError(format string, args ...any) error {
	return usage.Wrapf(format, args...)
}

// checksumError 表示校验和文件为空或与归档不一致。
type checksumError struct {
	want string
	got  string
}

func (e *checksumError) Error() string {
	if e.want == "" && e.got == "" {
		return "校验和文件为空"
	}
	return fmt.Sprintf("校验和不匹配: 期望 %s，实际 %s", e.want, e.got)
}

// Run 执行发布流程：归档校验 → 校验和比对 → manifest 推断 → 上传。
// 退出码约定：UsageError→2，其它错误→1。
func Run(opts Options) (Result, error) {
	// 1. 归档必须存在
	info, err := os.Stat(opts.Archive)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{}, usageError("归档不存在: %s", opts.Archive)
		}
		return Result{}, fmt.Errorf("访问归档失败: %w", err)
	}
	if info.IsDir() {
		return Result{}, usageError("归档不能是目录: %s", opts.Archive)
	}

	// 2. 校验和文件：默认 <archive>.sha256，须存在且与归档一致
	checksumPath := opts.Checksum
	if strings.TrimSpace(checksumPath) == "" {
		checksumPath = opts.Archive + ".sha256"
	}
	if _, err := os.Stat(checksumPath); err != nil {
		if os.IsNotExist(err) {
			return Result{}, usageError("校验和文件不存在: %s", checksumPath)
		}
		return Result{}, fmt.Errorf("访问校验和文件失败: %w", err)
	}
	if err := verifyChecksumFile(opts.Archive, checksumPath); err != nil {
		var ce *checksumError
		if errors.As(err, &ce) {
			return Result{}, usageError("%w", ce)
		}
		return Result{}, err
	}

	// 3. 从归档内 manifest 推断 name/version/type
	name, version, typ, err := InspectArchive(opts.Archive)
	if err != nil {
		return Result{}, usageError("%w", err)
	}

	// 4. registry.url 必须配置
	if strings.TrimSpace(opts.RegistryURL) == "" {
		return Result{}, usageError("未配置 registry.url，请在 ~/.work/config.yaml 中设置")
	}
	target := strings.TrimRight(opts.RegistryURL, "/") + "/bundles"

	res := Result{
		URL:     target,
		Name:    name,
		Version: version,
		Type:    typ,
	}

	// 5. dry-run：不发送
	if opts.DryRun {
		return res, nil
	}

	// 6. 上传
	if err := upload(target, opts.Archive, checksumPath, name, version, typ); err != nil {
		return Result{}, err
	}
	return res, nil
}

// InspectArchive 从归档内读取根目录 manifest，推断 name/version/type。
func InspectArchive(path string) (name, version, typ string, err error) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return inspectZip(path)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return inspectTarGz(path)
	default:
		return "", "", "", fmt.Errorf("不支持的归档格式（仅支持 .zip/.tar.gz）: %s", path)
	}
}

// verifyChecksumFile 读取校验和文件（取首段 hex），重算归档 sha256 比对。
func verifyChecksumFile(archive, checksumPath string) error {
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("读取校验和文件失败: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(data)))
	if len(fields) == 0 {
		return &checksumError{}
	}
	want := fields[0]

	f, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("打开归档失败: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("计算校验和失败: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return &checksumError{want: want, got: got}
	}
	return nil
}

type manifestMeta struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// parseManifest 解析 manifest 内容并按文件名映射 type。
func parseManifest(fileName string, data []byte) (string, string, string, error) {
	var m manifestMeta
	if err := yaml.Unmarshal(data, &m); err != nil {
		return "", "", "", fmt.Errorf("解析 manifest 失败: %w", err)
	}
	kind, ok := kindFromManifestName(fileName)
	if !ok {
		return "", "", "", fmt.Errorf("未知的 manifest 文件名: %s", fileName)
	}
	return m.Name, m.Version, string(kind), nil
}

func kindFromManifestName(name string) (pkgmanifest.Kind, bool) {
	return pkgmanifest.KindFromFile(name)
}

// manifestTargets 是支持的 manifest 文件名集合，从 pkgmanifest 中获取。
func manifestTargetsSet() map[string]bool {
	m := make(map[string]bool)
	for _, n := range pkgmanifest.ManifestFileNames() {
		m[n] = true
	}
	return m
}

var manifestTargets = manifestTargetsSet()

// depth 返回归档内条目的路径层数（无斜号为 0，a/b 为 1）。
func depth(name string) int {
	return strings.Count(filepath.ToSlash(name), "/")
}

func inspectZip(path string) (string, string, string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", "", "", fmt.Errorf("打开 zip 失败: %w", err)
	}
	defer r.Close()

	var found *zip.File
	for i := range r.File {
		f := r.File[i]
		if f.FileInfo().IsDir() {
			continue
		}
		if !manifestTargets[filepath.Base(f.Name)] {
			continue
		}
		// 优先取最浅（根目录）的 manifest
		if found == nil || depth(f.Name) < depth(found.Name) {
			found = f
		}
	}
	if found == nil {
		return "", "", "", fmt.Errorf("归档内未找到 bundle.yaml/installer.yaml/hooks.yaml")
	}

	rc, err := found.Open()
	if err != nil {
		return "", "", "", fmt.Errorf("打开 manifest 失败: %w", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", "", "", fmt.Errorf("读取 manifest 失败: %w", err)
	}
	return parseManifest(found.Name, data)
}

func inspectTarGz(path string) (string, string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", "", fmt.Errorf("打开归档失败: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", "", "", fmt.Errorf("读取 gzip 失败: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	var foundName string
	var foundData []byte
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", "", fmt.Errorf("读取 tar 失败: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		base := filepath.Base(hdr.Name)
		if !manifestTargets[base] {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return "", "", "", fmt.Errorf("读取 manifest %s 失败: %w", hdr.Name, err)
		}
		// 优先取最浅（根目录）的 manifest
		if foundName == "" || depth(hdr.Name) < depth(foundName) {
			foundName = hdr.Name
			foundData = data
		}
	}
	if foundName == "" {
		return "", "", "", fmt.Errorf("归档内未找到 bundle.yaml/installer.yaml/hooks.yaml")
	}
	return parseManifest(foundName, foundData)
}

// upload 以 multipart/form-data 流式上传归档与校验和至 target。
// 使用 io.Pipe 避免将整个归档缓冲到内存。
func upload(target, archive, checksumPath, name, version, typ string) error {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	// 在 goroutine 中构建 multipart 表单，写入 pipe。
	uploadErr := make(chan error, 1)
	go func() {
		defer pw.Close()
		defer mw.Close()
		if err := mw.WriteField("name", name); err != nil {
			uploadErr <- fmt.Errorf("构造表单失败: %w", err)
			return
		}
		if err := mw.WriteField("version", version); err != nil {
			uploadErr <- fmt.Errorf("构造表单失败: %w", err)
			return
		}
		if err := mw.WriteField("type", typ); err != nil {
			uploadErr <- fmt.Errorf("构造表单失败: %w", err)
			return
		}
		if err := writeFileField(mw, "archive", archive); err != nil {
			uploadErr <- err
			return
		}
		if err := writeFileField(mw, "checksum", checksumPath); err != nil {
			uploadErr <- err
			return
		}
		uploadErr <- nil
	}()

	req, err := http.NewRequest(http.MethodPost, target, pr)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}
	defer resp.Body.Close()

	// 等待表单构建完成以检测构建阶段的错误。
	if err := <-uploadErr; err != nil {
		return err
	}

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
		return nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Registry 拒绝上传 (%s): %s", resp.Status, strings.TrimSpace(string(b)))
	default:
		return fmt.Errorf("Registry 返回错误: %s", resp.Status)
	}
}

// writeFileField 将本地文件写入 multipart 表单字段。
func writeFileField(mw *multipart.Writer, fieldname, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开 %s 失败: %w", fieldname, err)
	}
	defer f.Close()

	w, err := mw.CreateFormFile(fieldname, filepath.Base(path))
	if err != nil {
		return fmt.Errorf("创建表单字段 %s 失败: %w", fieldname, err)
	}
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("写入表单字段 %s 失败: %w", fieldname, err)
	}
	return nil
}
