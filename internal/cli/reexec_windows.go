//go:build windows

package cli

import (
	"os"
	"os/exec"
)

func reExecute() error {
	exe, err := resolveExecutable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, osArgs()...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// 子进程已结束并返回退出码：父进程必须以相同退出码退出，
			// 此处 os.Exit 是 reexec 语义的必要特例（非普通命令路径）。
			os.Exit(ee.ExitCode())
		}
		return err
	}
	// 子进程成功完成：父进程直接退出，避免继续执行原命令流程。
	// 此处 os.Exit(0) 是 reexec 语义的必要特例。
	os.Exit(0)
	return nil
}
