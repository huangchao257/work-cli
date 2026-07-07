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
	info := LookupIDE(ide)
	if info == nil {
		return "", errUnknownIDE(ide)
	}
	dir, err := RuleDir(ide, scope)
	if err != nil {
		return "", err
	}
	ext := info.RuleExt
	if ext == "" {
		ext = ".md"
	}
	return filepath.Join(dir, ruleID+ext), nil
}

func RuleDir(ide IDE, scope string) (string, error) {
	info := LookupIDE(ide)
	if info == nil {
		return "", errUnknownIDE(ide)
	}
	base, err := ideBase(ide, scope)
	if err != nil {
		return "", err
	}
	if info.RulesSubdir == "" {
		return base, nil
	}
	return filepath.Join(base, info.RulesSubdir), nil
}

func MCPConfigPath(ide IDE, scope string) (string, error) {
	base, err := ideBase(ide, scope)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "mcp.json"), nil
}

func ideBase(ide IDE, scope string) (string, error) {
	info := LookupIDE(ide)
	if info == nil {
		return "", errUnknownIDE(ide)
	}
	if scope == "project" {
		root, err := ProjectRoot()
		if err != nil {
			return "", err
		}
		return filepath.Join(root, info.DotDir), nil
	}
	home, err := UserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, info.DotDir), nil
}

type unknownIDEError string

func (e unknownIDEError) Error() string { return fmt.Sprintf("未知 IDE: %s", string(e)) }
func errUnknownIDE(ide IDE) error       { return unknownIDEError(ide) }
