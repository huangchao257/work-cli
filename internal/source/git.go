// Package source 解析安装引用的来源（内置 catalog / registry / git），并拉取包内容。

package source

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ResolveGit(url, ref, cacheDir string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("未找到 git 命令，请先安装 Git")
	}
	// 仅允许 HTTPS：防止 git 的 ext:: 伪协议（克隆即执行任意命令）、file:// 读本地
	// 仓库、ssh:// 触发对任意主机的连接等注入面。同时拒绝含空白/控制字符的 URL 与 ref。
	if !strings.HasPrefix(url, "https://") {
		return "", fmt.Errorf("git 下载地址必须使用 HTTPS")
	}
	if strings.ContainsAny(url, " \t\r\n") || strings.ContainsAny(ref, " \t\r\n") {
		return "", fmt.Errorf("git 下载地址或 ref 含非法字符")
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(url+"@"+ref)))
	dest := filepath.Join(cacheDir, "git", sum)
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		return dest, nil
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone 失败: %w", err)
	}
	return dest, nil
}
