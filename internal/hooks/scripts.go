package hooks

import (
	"fmt"
	"path/filepath"
	"strings"
)

func CommandPathForIDE(ide, scope, kitName, scriptName string) (string, error) {
	return commandPathForIDE(ide, scope, kitName, scriptName)
}

func WriteTelemetryScript(path, workBin, kitName, scope string) error {
	content := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
input=$(cat)
"%s" hooks report --ide "${WORK_HOOKS_IDE}" --event "${WORK_HOOKS_EVENT}" --hooks-kit "%s" --scope "%s" <<< "$input" || true
printf '%%s' "$input"
exit 0
`, escapeShell(workBin), kitName, scope)
	return writeExecutable(path, content)
}

func WriteWrapperScript(path, baseScript, ide, ideEvent, kitName, scope string) error {
	content := fmt.Sprintf(`#!/usr/bin/env bash
export WORK_HOOKS_IDE=%q
export WORK_HOOKS_EVENT=%q
export WORK_HOOKS_KIT=%q
export WORK_HOOKS_SCOPE=%q
exec %q
`, ide, ideEvent, kitName, scope, baseScript)
	return writeExecutable(path, content)
}

func escapeShell(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

func ScriptDirRel(ide, scope, kitName string) (string, error) {
	dir, err := HooksScriptDir(ide, scope, kitName)
	if err != nil {
		return "", err
	}
	return filepath.Base(filepath.Dir(dir)) + "/" + filepath.Base(dir), nil
}
