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
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}
