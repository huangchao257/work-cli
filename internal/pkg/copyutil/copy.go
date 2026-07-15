package copyutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func CopyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("获取源目录信息失败 %s: %w", src, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s 不是目录", src)
	}
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, fi.Mode())
		}
		return copyFile(path, target, fi.Mode())
	})
}

func CopyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("获取源文件信息失败 %s: %w", src, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s 是目录", src)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return copyFile(src, dst, info.Mode())
}

func copyFile(src, dst string, mode os.FileMode) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败 %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("创建目标文件失败 %s: %w", dst, err)
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("复制文件内容失败 %s -> %s: %w", src, dst, err)
	}
	return nil
}
