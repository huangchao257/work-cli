package platform

// EventBinding 描述 IDE 事件名与可选的匹配器正则。
type EventBinding struct {
	Event   string // IDE 事件名，如 "beforeShellExecution"
	Matcher string // 匹配器正则，如 "Bash"；空表示无需匹配
}

// IDEInfo 描述一个 IDE 的所有元数据，取代 hooks/paths.go 和 hooks/events.go 中的 switch 块。
type IDEInfo struct {
	ID          IDE
	DotDir      string // ".cursor", ".qoder", ".claude"
	HooksFile   string // "hooks.json" 或 "settings.json"
	RulesSubdir string // "rules" 或 ""（Claude 无 rules 子目录）
	RuleExt     string // ".mdc" 或 ".md"
	DetectFn    func() bool
	Events      map[string][]EventBinding // 抽象事件名 → IDE 事件绑定列表
}

// 抽象事件常量（从 hooks 包移过来以消除依赖；hooks 包引用这里的常量名）
const (
	EventShell    = "shell"
	EventMCP      = "mcp"
	EventFileRead = "file_read"
	EventFileEdit = "file_edit"
	EventPrompt   = "prompt"
	EventSession  = "session"
	EventTool     = "tool"
)

var ideRegistry = map[IDE]*IDEInfo{}

func init() {
	registerIDE(&IDEInfo{
		ID: IDECursor, DotDir: ".cursor", HooksFile: "hooks.json",
		RulesSubdir: "rules", RuleExt: ".mdc",
		DetectFn: DetectCursor,
		Events: map[string][]EventBinding{
			EventShell:    {{Event: "beforeShellExecution"}, {Event: "afterShellExecution"}},
			EventMCP:      {{Event: "beforeMCPExecution"}, {Event: "afterMCPExecution"}},
			EventFileRead: {{Event: "beforeReadFile"}},
			EventFileEdit: {{Event: "afterFileEdit"}},
			EventPrompt:   {{Event: "beforeSubmitPrompt"}},
			EventSession:  {{Event: "sessionStart"}, {Event: "sessionEnd"}},
			EventTool:     {{Event: "preToolUse"}, {Event: "postToolUse"}},
		},
	})
	registerIDE(&IDEInfo{
		ID: IDEQoder, DotDir: ".qoder", HooksFile: "settings.json",
		RulesSubdir: "rules", RuleExt: ".md",
		DetectFn: DetectQoder,
		Events: map[string][]EventBinding{
			EventShell:    {{Event: "PreToolUse", Matcher: "Bash"}, {Event: "PostToolUse", Matcher: "Bash"}},
			EventMCP:      {{Event: "PreToolUse", Matcher: "MCP.*|mcp__.*"}, {Event: "PostToolUse", Matcher: "MCP.*|mcp__.*"}},
			EventFileRead: {{Event: "PreToolUse", Matcher: "Read"}},
			EventFileEdit: {{Event: "PostToolUse", Matcher: "Write|Edit"}},
			EventPrompt:   {{Event: "UserPromptSubmit"}},
			EventSession:  nil, // Qoder 不支持 session
			EventTool:     {{Event: "PreToolUse"}, {Event: "PostToolUse"}},
		},
	})
	registerIDE(&IDEInfo{
		ID: IDEClaude, DotDir: ".claude", HooksFile: "settings.json",
		RulesSubdir: "", RuleExt: ".md",
		DetectFn: DetectClaude,
		Events: map[string][]EventBinding{
			EventShell:    {{Event: "PreToolUse", Matcher: "Bash"}, {Event: "PostToolUse", Matcher: "Bash"}},
			EventMCP:      {{Event: "PreToolUse", Matcher: "MCP.*|mcp__.*"}, {Event: "PostToolUse", Matcher: "MCP.*|mcp__.*"}},
			EventFileRead: {{Event: "PreToolUse", Matcher: "Read"}},
			EventFileEdit: {{Event: "PostToolUse", Matcher: "Write|Edit"}},
			EventPrompt:   {{Event: "UserPromptSubmit"}},
			EventSession:  {{Event: "SessionStart"}, {Event: "SessionEnd"}},
			EventTool:     {{Event: "PreToolUse"}, {Event: "PostToolUse"}},
		},
	})
}

func registerIDE(info *IDEInfo) { ideRegistry[info.ID] = info }

// LookupIDE 返回指定 IDE 的注册信息。未找到返回 nil。
func LookupIDE(ide IDE) *IDEInfo { return ideRegistry[ide] }

// AllIDEs 返回所有已注册的 IDE 信息（用于遍历）。
func AllIDEs() []*IDEInfo {
	out := make([]*IDEInfo, 0, len(ideRegistry))
	for _, info := range ideRegistry {
		out = append(out, info)
	}
	return out
}
