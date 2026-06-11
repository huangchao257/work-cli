package cli

import (
	"os"
	"path/filepath"
)

func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func osArgs() []string {
	if len(os.Args) > 1 {
		return os.Args[1:]
	}
	return nil
}

func osEnviron() []string {
	return os.Environ()
}
