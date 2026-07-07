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
// It reads event bindings from the platform IDE registry, replacing the former
// switch-based cursorBindings/settingsBindings/bindingsForAbstract functions.
func BindingsForIDE(ide string, events []string) ([]Binding, []string) {
	info := platform.LookupIDE(platform.IDE(ide))
	if info == nil {
		return nil, []string{fmt.Sprintf("未知 IDE: %s", ide)}
	}
	var bindings []Binding
	var warnings []string
	for _, ev := range events {
		eventBindings, ok := info.Events[ev]
		if !ok || len(eventBindings) == 0 {
			warnings = append(warnings, fmt.Sprintf("%s 不支持事件 %s，已跳过", ide, ev))
			continue
		}
		for _, eb := range eventBindings {
			bindings = append(bindings, Binding{IDEEvent: eb.Event, Matcher: eb.Matcher})
		}
	}
	return bindings, warnings
}

// AbstractForIDEReport maps an IDE-specific event name back to the abstract
// event name when possible, using the platform IDE registry's event bindings.
func AbstractForIDEReport(ide, ideEvent string) string {
	info := platform.LookupIDE(platform.IDE(ide))
	if info == nil {
		return ideEvent
	}
	for abstract, bindings := range info.Events {
		for _, b := range bindings {
			if strings.EqualFold(b.Event, ideEvent) {
				return abstract
			}
		}
	}
	return ideEvent
}
