package hooks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/huangchao257/work-cli/internal/platform"
)

func HooksConfigPath(ide, scope string) (string, error) {
	if scope == "project" {
		root, err := platform.ProjectRoot()
		if err != nil {
			return "", err
		}
		switch ide {
		case "cursor":
			return filepath.Join(root, ".cursor", "hooks.json"), nil
		case "qoder":
			return filepath.Join(root, ".qoder", "settings.json"), nil
		case "claude":
			return filepath.Join(root, ".claude", "settings.json"), nil
		default:
			return "", fmt.Errorf("未知 IDE: %s", ide)
		}
	}
	home, err := platform.UserHome()
	if err != nil {
		return "", err
	}
	switch ide {
	case "cursor":
		return filepath.Join(home, ".cursor", "hooks.json"), nil
	case "qoder":
		return filepath.Join(home, ".qoder", "settings.json"), nil
	case "claude":
		return filepath.Join(home, ".claude", "settings.json"), nil
	default:
		return "", fmt.Errorf("未知 IDE: %s", ide)
	}
}

func HooksScriptDir(ide, scope, kitName string) (string, error) {
	base, err := ideHooksBase(ide, scope)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, workTelemetryDir, kitName), nil
}

func ideHooksBase(ide, scope string) (string, error) {
	if scope == "project" {
		root, err := platform.ProjectRoot()
		if err != nil {
			return "", err
		}
		switch ide {
		case "cursor":
			return filepath.Join(root, ".cursor", "hooks"), nil
		case "qoder":
			return filepath.Join(root, ".qoder", "hooks"), nil
		case "claude":
			return filepath.Join(root, ".claude", "hooks"), nil
		default:
			return "", fmt.Errorf("未知 IDE: %s", ide)
		}
	}
	home, err := platform.UserHome()
	if err != nil {
		return "", err
	}
	switch ide {
	case "cursor":
		return filepath.Join(home, ".cursor", "hooks"), nil
	case "qoder":
		return filepath.Join(home, ".qoder", "hooks"), nil
	case "claude":
		return filepath.Join(home, ".claude", "hooks"), nil
	default:
		return "", fmt.Errorf("未知 IDE: %s", ide)
	}
}

func commandPathForIDE(ide, scope, kitName, scriptName string) (string, error) {
	dir, err := HooksScriptDir(ide, scope, kitName)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(filepath.Join(dir, scriptName))
	if err != nil {
		return "", err
	}
	if ide != "cursor" {
		return abs, nil
	}
	var base string
	if scope == "project" {
		base, err = platform.ProjectRoot()
	} else {
		base, err = platform.UserHome()
		if err == nil {
			base = filepath.Join(base, ".cursor")
		}
	}
	if err != nil {
		return abs, nil
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil {
		return abs, nil
	}
	return filepath.ToSlash(rel), nil
}

func writeExecutable(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return err
	}
	return nil
}
