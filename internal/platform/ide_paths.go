package platform

import (
	"fmt"
	"path/filepath"
)

type IDE string

const (
	IDEQoder  IDE = "qoder"
	IDECursor IDE = "cursor"
	IDEClaude IDE = "claude"
)

func SkillDir(ide IDE, scope, skillID string) (string, error) {
	base, err := ideBase(ide, scope)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "skills", skillID), nil
}

func RuleFile(ide IDE, scope, ruleID string) (string, error) {
	dir, err := RuleDir(ide, scope)
	if err != nil {
		return "", err
	}
	ext := ".md"
	if ide == IDECursor {
		ext = ".mdc"
	}
	return filepath.Join(dir, ruleID+ext), nil
}

func RuleDir(ide IDE, scope string) (string, error) {
	base, err := ideBase(ide, scope)
	if err != nil {
		return "", err
	}
	switch ide {
	case IDEClaude:
		return base, nil
	default:
		return filepath.Join(base, "rules"), nil
	}
}

func MCPConfigPath(ide IDE, scope string) (string, error) {
	switch ide {
	case IDECursor:
		if scope == "project" {
			root, err := ProjectRoot()
			if err != nil {
				return "", err
			}
			return filepath.Join(root, ".cursor", "mcp.json"), nil
		}
		home, err := UserHome()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cursor", "mcp.json"), nil
	case IDEQoder:
		if scope == "project" {
			root, err := ProjectRoot()
			if err != nil {
				return "", err
			}
			return filepath.Join(root, ".qoder", "mcp.json"), nil
		}
		home, err := UserHome()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".qoder", "mcp.json"), nil
	case IDEClaude:
		if scope == "project" {
			root, err := ProjectRoot()
			if err != nil {
				return "", err
			}
			return filepath.Join(root, ".claude", "mcp.json"), nil
		}
		home, err := UserHome()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".claude", "mcp.json"), nil
	default:
		return "", errUnknownIDE(ide)
	}
}

func ideBase(ide IDE, scope string) (string, error) {
	if scope == "project" {
		root, err := ProjectRoot()
		if err != nil {
			return "", err
		}
		switch ide {
		case IDEQoder:
			return filepath.Join(root, ".qoder"), nil
		case IDECursor:
			return filepath.Join(root, ".cursor"), nil
		case IDEClaude:
			return filepath.Join(root, ".claude"), nil
		default:
			return "", errUnknownIDE(ide)
		}
	}
	home, err := UserHome()
	if err != nil {
		return "", err
	}
	switch ide {
	case IDEQoder:
		return filepath.Join(home, ".qoder"), nil
	case IDECursor:
		return filepath.Join(home, ".cursor"), nil
	case IDEClaude:
		return filepath.Join(home, ".claude"), nil
	default:
		return "", errUnknownIDE(ide)
	}
}

type unknownIDEError string

func (e unknownIDEError) Error() string { return fmt.Sprintf("未知 IDE: %s", string(e)) }
func errUnknownIDE(ide IDE) error       { return unknownIDEError(ide) }
