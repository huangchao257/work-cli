package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func ResolveCommand(spec CommandSpec) (string, error) {
	if p, ok := spec.Platforms[runtime.GOOS]; ok && strings.TrimSpace(p.Run) != "" {
		return p.Run, nil
	}
	if strings.TrimSpace(spec.Run) != "" {
		return spec.Run, nil
	}
	return "", fmt.Errorf("当前系统 %s 无可用命令", runtime.GOOS)
}

func Run(ctx context.Context, command string) error {
	shell, flag := defaultShell()
	cmd := exec.CommandContext(ctx, shell, flag, command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行命令失败: %w", err)
	}
	return nil
}

func RunCommand(ctx context.Context, parts []string) error {
	if len(parts) == 0 {
		return fmt.Errorf("verify 命令为空")
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("验证命令失败: %w", err)
	}
	return nil
}

func defaultShell() (string, string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", "/C"
	}
	return "sh", "-c"
}
