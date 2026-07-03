package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type cursorHooksFile struct {
	Version int                          `json:"version"`
	Hooks   map[string][]cursorHookEntry `json:"hooks"`
}

type cursorHookEntry struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

type settingsFile struct {
	Hooks map[string][]matcherGroup `json:"hooks"`
}

type matcherGroup struct {
	Matcher string         `json:"matcher,omitempty"`
	Hooks   []settingsHook `json:"hooks"`
}

type settingsHook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

func MergeCursorHooks(configPath string, entries []SidecarEntry) error {
	var cfg cursorHooksFile
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("读取 Cursor hooks.json 失败: %w", err)
		}
		cfg = cursorHooksFile{Version: 1, Hooks: map[string][]cursorHookEntry{}}
	} else {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("解析 Cursor hooks.json 失败: %w", err)
		}
		if cfg.Hooks == nil {
			cfg.Hooks = map[string][]cursorHookEntry{}
		}
		if cfg.Version == 0 {
			cfg.Version = 1
		}
	}

	// Remove prior work-managed entries
	for event, list := range cfg.Hooks {
		filtered := make([]cursorHookEntry, 0, len(list))
		for _, e := range list {
			if IsWorkManagedCommand(e.Command) {
				continue
			}
			filtered = append(filtered, e)
		}
		if len(filtered) == 0 {
			delete(cfg.Hooks, event)
		} else {
			cfg.Hooks[event] = filtered
		}
	}

	for _, ent := range entries {
		cfg.Hooks[ent.IDEEvent] = append(cfg.Hooks[ent.IDEEvent], cursorHookEntry{
			Command: ent.Command,
			Timeout: 3,
		})
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 hooks.json 失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("创建 hooks 目录失败: %w", err)
	}
	if err := os.WriteFile(configPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入 hooks.json 失败: %w", err)
	}
	return nil
}

func MergeSettingsHooks(configPath string, entries []SidecarEntry) error {
	root := map[string]any{}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("读取 settings.json 失败: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("解析 settings.json 失败: %w", err)
		}
	}

	var hooks settingsFile
	if raw, ok := root["hooks"]; ok {
		b, _ := json.Marshal(raw)
		_ = json.Unmarshal(b, &hooks)
	}
	if hooks.Hooks == nil {
		hooks.Hooks = map[string][]matcherGroup{}
	}

	for event, groups := range hooks.Hooks {
		var kept []matcherGroup
		for _, g := range groups {
			var inner []settingsHook
			for _, h := range g.Hooks {
				if IsWorkManagedCommand(h.Command) {
					continue
				}
				inner = append(inner, h)
			}
			if len(inner) == 0 {
				continue
			}
			g.Hooks = inner
			kept = append(kept, g)
		}
		if len(kept) == 0 {
			delete(hooks.Hooks, event)
		} else {
			hooks.Hooks[event] = kept
		}
	}

	for _, ent := range entries {
		group := matcherGroup{
			Matcher: ent.Matcher,
			Hooks: []settingsHook{{
				Type:    "command",
				Command: ent.Command,
				Timeout: 3,
			}},
		}
		hooks.Hooks[ent.IDEEvent] = append(hooks.Hooks[ent.IDEEvent], group)
	}

	root["hooks"] = hooks.Hooks
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 settings.json 失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("创建 settings 目录失败: %w", err)
	}
	if err := os.WriteFile(configPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入 settings.json 失败: %w", err)
	}
	return nil
}

func UnmergeCursorHooks(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取 Cursor hooks.json 失败: %w", err)
	}
	var cfg cursorHooksFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析 Cursor hooks.json 失败: %w", err)
	}
	for event, list := range cfg.Hooks {
		filtered := make([]cursorHookEntry, 0, len(list))
		for _, e := range list {
			if IsWorkManagedCommand(e.Command) {
				continue
			}
			filtered = append(filtered, e)
		}
		if len(filtered) == 0 {
			delete(cfg.Hooks, event)
		} else {
			cfg.Hooks[event] = filtered
		}
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 hooks.json 失败: %w", err)
	}
	if err := os.WriteFile(configPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入 hooks.json 失败: %w", err)
	}
	return nil
}

func UnmergeSettingsHooks(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取 settings.json 失败: %w", err)
	}
	root := map[string]any{}
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析 settings.json 失败: %w", err)
	}
	raw, ok := root["hooks"]
	if !ok {
		return nil
	}
	b, _ := json.Marshal(raw)
	var hooks settingsFile
	if err := json.Unmarshal(b, &hooks); err != nil {
		return fmt.Errorf("解析 hooks 字段失败: %w", err)
	}
	for event, groups := range hooks.Hooks {
		var kept []matcherGroup
		for _, g := range groups {
			var inner []settingsHook
			for _, h := range g.Hooks {
				if IsWorkManagedCommand(h.Command) {
					continue
				}
				inner = append(inner, h)
			}
			if len(inner) == 0 {
				continue
			}
			g.Hooks = inner
			kept = append(kept, g)
		}
		if len(kept) == 0 {
			delete(hooks.Hooks, event)
		} else {
			hooks.Hooks[event] = kept
		}
	}
	if len(hooks.Hooks) == 0 {
		delete(root, "hooks")
	} else {
		root["hooks"] = hooks.Hooks
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 settings.json 失败: %w", err)
	}
	if err := os.WriteFile(configPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入 settings.json 失败: %w", err)
	}
	return nil
}
