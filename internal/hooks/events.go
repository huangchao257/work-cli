package hooks

import (
	"fmt"
	"strings"

	"github.com/huangchao257/work-cli/internal/platform"
)

// Abstract event names used in hooks.yaml and config.
// 保持向后兼容的常量别名，实际定义在 platform 包。
const (
	EventShell    = platform.EventShell
	EventMCP      = platform.EventMCP
	EventFileRead = platform.EventFileRead
	EventFileEdit = platform.EventFileEdit
	EventPrompt   = platform.EventPrompt
	EventSession  = platform.EventSession
	EventTool     = platform.EventTool
)

const (
	PresetAudit = "audit"
	PresetAll   = "all"
)

// Binding is one IDE hook registration (event + optional matcher).
type Binding struct {
	IDEEvent string
	Matcher  string
}

func ResolveEvents(m *Manifest, userEvents []string) ([]string, error) {
	if len(userEvents) > 0 {
		return normalizeEvents(userEvents)
	}
	if len(m.Telemetry.Events) > 0 {
		return normalizeEvents(m.Telemetry.Events)
	}
	preset := strings.TrimSpace(m.Telemetry.Preset)
	if preset == "" {
		preset = PresetAudit
	}
	switch preset {
	case PresetAudit:
		return []string{EventShell, EventMCP, EventFileRead, EventFileEdit, EventPrompt}, nil
	case PresetAll:
		return []string{EventShell, EventMCP, EventFileRead, EventFileEdit, EventPrompt, EventSession, EventTool}, nil
	default:
		return nil, fmt.Errorf("未知 telemetry preset: %s", preset)
	}
}

func normalizeEvents(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, e := range in {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if !isValidAbstractEvent(e) {
			return nil, fmt.Errorf("未知抽象事件: %s", e)
		}
		if seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("事件列表为空")
	}
	return out, nil
}

func isValidAbstractEvent(e string) bool {
	switch e {
	case EventShell, EventMCP, EventFileRead, EventFileEdit, EventPrompt, EventSession, EventTool:
		return true
	default:
		return false
	}
}

// BindingsForIDE returns hook bindings for the given IDE and abstract events.
func BindingsForIDE(ide string, events []string) ([]Binding, []string) {
	var bindings []Binding
	var warnings []string
	for _, ev := range events {
		b, warn := bindingsForAbstract(ide, ev)
		if warn != "" {
			warnings = append(warnings, warn)
			continue
		}
		bindings = append(bindings, b...)
	}
	return bindings, warnings
}

func bindingsForAbstract(ide, abstract string) ([]Binding, string) {
	switch ide {
	case "cursor":
		return cursorBindings(abstract)
	case "qoder", "claude":
		return settingsBindings(ide, abstract)
	default:
		return nil, fmt.Sprintf("未知 IDE: %s", ide)
	}
}

func cursorBindings(abstract string) ([]Binding, string) {
	switch abstract {
	case EventShell:
		return []Binding{
			{IDEEvent: "beforeShellExecution"},
			{IDEEvent: "afterShellExecution"},
		}, ""
	case EventMCP:
		return []Binding{
			{IDEEvent: "beforeMCPExecution"},
			{IDEEvent: "afterMCPExecution"},
		}, ""
	case EventFileRead:
		return []Binding{{IDEEvent: "beforeReadFile"}}, ""
	case EventFileEdit:
		return []Binding{{IDEEvent: "afterFileEdit"}}, ""
	case EventPrompt:
		return []Binding{{IDEEvent: "beforeSubmitPrompt"}}, ""
	case EventSession:
		return []Binding{
			{IDEEvent: "sessionStart"},
			{IDEEvent: "sessionEnd"},
		}, ""
	case EventTool:
		return []Binding{
			{IDEEvent: "preToolUse"},
			{IDEEvent: "postToolUse"},
		}, ""
	default:
		return nil, fmt.Sprintf("未知抽象事件: %s", abstract)
	}
}

func settingsBindings(ide, abstract string) ([]Binding, string) {
	switch abstract {
	case EventShell:
		return []Binding{
			{IDEEvent: "PreToolUse", Matcher: "Bash"},
			{IDEEvent: "PostToolUse", Matcher: "Bash"},
		}, ""
	case EventMCP:
		return []Binding{
			{IDEEvent: "PreToolUse", Matcher: "MCP.*|mcp__.*"},
			{IDEEvent: "PostToolUse", Matcher: "MCP.*|mcp__.*"},
		}, ""
	case EventFileRead:
		return []Binding{{IDEEvent: "PreToolUse", Matcher: "Read"}}, ""
	case EventFileEdit:
		return []Binding{{IDEEvent: "PostToolUse", Matcher: "Write|Edit"}}, ""
	case EventPrompt:
		return []Binding{{IDEEvent: "UserPromptSubmit"}}, ""
	case EventSession:
		if ide == "qoder" {
			return nil, "Qoder 不支持 session 事件，已跳过"
		}
		return []Binding{
			{IDEEvent: "SessionStart"},
			{IDEEvent: "SessionEnd"},
		}, ""
	case EventTool:
		return []Binding{
			{IDEEvent: "PreToolUse"},
			{IDEEvent: "PostToolUse"},
		}, ""
	default:
		return nil, fmt.Sprintf("未知抽象事件: %s", abstract)
	}
}

// AbstractForIDEReport maps IDE event back to abstract name when possible.
func AbstractForIDEReport(ide, ideEvent string) string {
	events := []string{EventShell, EventMCP, EventFileRead, EventFileEdit, EventPrompt, EventSession, EventTool}
	for _, ev := range events {
		bindings, _ := bindingsForAbstract(ide, ev)
		for _, b := range bindings {
			if strings.EqualFold(b.IDEEvent, ideEvent) {
				return ev
			}
		}
	}
	return ideEvent
}
