// Package pack 将本地套装目录打包为可分发归档（tar.gz/zip）并生成 sha256 校验和。
package pack

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	pkgmanifest "github.com/huangchao257/work-cli/internal/pkg/manifest"
	"github.com/huangchao257/work-cli/internal/usage"
	"gopkg.in/yaml.v3"
)

// Format 表示归档格式。
type Format string

const (
	FormatTarGz Format = "tar.gz"
	FormatZip   Format = "zip"
)

// ParseFormat 将字符串解析为 Format。空字符串视为 tar.gz。
func ParseFormat(s string) (Format, error) {
	switch s {
	case "", "tar.gz", "tgz":
		return FormatTarGz, nil
	case "zip":
		return FormatZip, nil
	default:
		return "", fmt.Errorf("不支持的归档格式 %q（可选 zip 或 tar.gz）", s)
	}
}

// Options 是 pack.Run 的输入参数。
type Options struct {
	Dir    string // 套装目录
	Format Format // 归档格式
	Output string // 输出路径（目录或完整文件路径，空表示默认）
	DryRun bool   // 仅预览，不写盘
}

// Result 是 pack.Run 的输出。
type Result struct {
	Archive   string `json:"archive"`  // 归档文件路径
	Checksum  string `json:"checksum"` // 校验和文件路径
	FileCount int    `json:"files"`    // 打包文件数量
	Name      string `json:"name"`     // 套装名
	Version   string `json:"version"`  // 套装版本
}

// UsageError 表示用法错误（目录无 manifest、格式非法等），对应退出码 2。
type UsageError = usage.Error

// IsUsageError 判断 err 是否为用法错误。
var IsUsageError = usage.Is

func usageError(format string, args ...any) error {
	return usage.Wrapf(format, args...)
}

// Run 执行打包。退出码约定：UsageError→2，其它错误→1。
func Run(opts Options) (Result, error) {
	dir := opts.Dir
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{}, usageError("目录不存在: %s", dir)
		}
		return Result{}, fmt.Errorf("访问目录失败: %w", err)
	}
	if !info.IsDir() {
		return Result{}, usageError("不是目录: %s", dir)
	}

	// 校验 manifest 存在
	kind, err := pkgmanifest.DetectKind(dir)
	if err != nil {
		return Result{}, usageError("%w", err)
	}

	// 读取 manifest 的 name/version
	meta, err := readManifestMeta(dir, kind)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(meta.Name) == "" || strings.TrimSpace(meta.Version) == "" {
		return Result{}, usageError("manifest 缺少 name 或 version 字段")
	}

	// 确定输出路径
	ext := ".tar.gz"
	if opts.Format == FormatZip {
		ext = ".zip"
	}
	defaultName := fmt.Sprintf("%s-%s%s", meta.Name, meta.Version, ext)
	archivePath := resolveOutputPath(opts.Output, dir, defaultName)
	checksumPath := archivePath + ".sha256"

	// 收集待打包文件（相对路径）
	files, err := collectFiles(dir)
	if err != nil {
		return Result{}, fmt.Errorf("遍历目录失败: %w", err)
	}

	res := Result{
		Archive:   archivePath,
		Checksum:  checksumPath,
		FileCount: len(files),
		Name:      meta.Name,
		Version:   meta.Version,
	}

	if opts.DryRun {
		return res, nil
	}

	// 写归档并同步计算 sha256
	sum, err := writeArchive(opts.Format, dir, files, archivePath)
	if err != nil {
		return Result{}, err
	}

	// 写校验和文件（仿 sha256sum：<hex>  <filename>）
	line := fmt.Sprintf("%s  %s\n", sum, filepath.Base(archivePath))
	if err := os.WriteFile(checksumPath, []byte(line), 0o644); err != nil {
		return Result{}, fmt.Errorf("写入校验和文件失败: %w", err)
	}

	return res, nil
}

type manifestMeta struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

func readManifestMeta(dir string, kind pkgmanifest.Kind) (manifestMeta, error) {
	name := manifestFileName(kind)
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return manifestMeta{}, fmt.Errorf("读取 manifest 失败: %w", err)
	}
	var m manifestMeta
	if err := yaml.Unmarshal(data, &m); err != nil {
		return manifestMeta{}, usageError("解析 manifest 失败: %w", err)
	}
	return m, nil
}

func manifestFileName(kind pkgmanifest.Kind) string {
	return pkgmanifest.FileName(kind)
}

// resolveOutputPath 解析输出路径：空→<dir>/../<defaultName>；已存在目录→其下 <defaultName>；否则视为完整文件路径。
func resolveOutputPath(output, dir, defaultName string) string {
	if output == "" {
		return filepath.Join(filepath.Dir(dir), defaultName)
	}
	if info, err := os.Stat(output); err == nil && info.IsDir() {
		return filepath.Join(output, defaultName)
	}
	return output
}

// collectFiles 以 dir 为归档根，返回相对路径列表（按字典序）。
func collectFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// writeArchive 创建归档文件，返回其 sha256 十六进制摘要。
func writeArchive(format Format, dir string, files []string, out string) (string, error) {
	f, err := os.Create(out)
	if err != nil {
		return "", fmt.Errorf("创建归档文件失败: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	mw := io.MultiWriter(f, h)

	switch format {
	case FormatZip:
		err = writeZip(mw, dir, files)
	default:
		err = writeTarGz(mw, dir, files)
	}
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeTarGz(w io.Writer, dir string, files []string) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, rel := range files {
		full := filepath.Join(dir, rel)
		info, err := os.Lstat(full)
		if err != nil {
			return fmt.Errorf("读取文件信息失败 %s: %w", rel, err)
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("构造 tar 头失败 %s: %w", rel, err)
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("写入 tar 头失败 %s: %w", rel, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := copyFileContent(tw, full); err != nil {
			return err
		}
	}
	return nil
}

func writeZip(w io.Writer, dir string, files []string) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, rel := range files {
		full := filepath.Join(dir, rel)
		info, err := os.Lstat(full)
		if err != nil {
			return fmt.Errorf("读取文件信息失败 %s: %w", rel, err)
		}
		name := filepath.ToSlash(rel)
		if info.IsDir() {
			if _, err := zw.Create(name + "/"); err != nil {
				return fmt.Errorf("写入 zip 目录项失败 %s: %w", rel, err)
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		fw, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("创建 zip 项失败 %s: %w", rel, err)
		}
		if err := copyFileContent(fw, full); err != nil {
			return err
		}
	}
	return nil
}

func copyFileContent(dst io.Writer, path string) error {
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开文件失败 %s: %w", path, err)
	}
	defer src.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("写入文件内容失败 %s: %w", path, err)
	}
	return nil
}
