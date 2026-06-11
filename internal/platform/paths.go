package platform

import (
	"os"
	"path/filepath"
)

func UserHome() (string, error) {
	return os.UserHomeDir()
}

func WorkConfigDir() (string, error) {
	home, err := UserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".work"), nil
}

func WorkStatePath(scope string) (string, error) {
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".work", "installed.json"), nil
	}
	base, err := WorkConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "installed.json"), nil
}

func ProjectRoot() (string, error) {
	return os.Getwd()
}
