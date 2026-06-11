//go:build !windows

package cli

import (
	"os"
	"syscall"
)

func reExecute() error {
	exe, err := resolveExecutable()
	if err != nil {
		return err
	}
	args := make([]string, len(os.Args))
	copy(args, os.Args)
	args[0] = exe
	return syscall.Exec(exe, args, osEnviron())
}
