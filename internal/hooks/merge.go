package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type CursorHooksFile struct {
	Version int                          `json:"version"`
	Hooks   map[string][]CursorHookEntry `json:"hooks"`
}

type CursorHookEntry struct {
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

// MergeCursorHooks 将套装的 hook 条目合并进 Cursor hooks.json。
// kitName 用于按套装置换旧条目：只删本套装先前写入的条目，不动其他套装。
func MergeCursorHooks(configPath string, kitName string, entries []SidecarEntry) error {
	var cfg CursorHooksFile
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("读取 Cursor hooks.json 失败: %w", err)
		}
		cfg = CursorHooksFile{Version: 1, Hooks: map[string][]CursorHookEntry{}}
	} else {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("解析 Cursor hooks.json 失败: %w", err)
		}
		if cfg.Hooks == nil {
			cfg.Hooks = map[string][]CursorHookEntry{}
		}
		if cfg.Version == 0 {
			cfg.Version = 1
		}
	}

	// Remove prior work-managed entries
	for event, list := range cfg.Hooks {
		filtered := make([]CursorHookEntry, 0, len(list))
		for _, e := range list {
			if IsWorkManagedCommand(e.Command, kitName) {
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
		cfg.Hooks[ent.IDEEvent] = append(cfg.Hooks[ent.IDEEvent], CursorHookEntry{
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
	return atomicWriteJSON(configPath, out)
}

func MergeSettingsHooks(configPath string, kitName string, entries []SidecarEntry) error {
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
		b, err := json.Marshal(raw)
		if err != nil {
			return fmt.Errorf("编码现有 hooks 失败: %w", err)
		}
		if err := json.Unmarshal(b, &hooks); err != nil {
			return fmt.Errorf("解析现有 hooks 失败（不支持的格式，请手动迁移）: %w", err)
		}
	}
	if hooks.Hooks == nil {
		hooks.Hooks = map[string][]matcherGroup{}
	}

	for event, groups := range hooks.Hooks {
		kept := make([]matcherGroup, 0, len(groups))
		for _, g := range groups {
			inner := make([]settingsHook, 0, len(g.Hooks))
			for _, h := range g.Hooks {
				if IsWorkManagedCommand(h.Command, kitName) {
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
	return atomicWriteJSON(configPath, out)
}

func UnmergeCursorHooks(configPath string, kitName string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取 Cursor hooks.json 失败: %w", err)
	}
	var cfg CursorHooksFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析 Cursor hooks.json 失败: %w", err)
	}
	for event, list := range cfg.Hooks {
		filtered := make([]CursorHookEntry, 0, len(list))
		for _, e := range list {
			if IsWorkManagedCommand(e.Command, kitName) {
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
	return atomicWriteJSON(configPath, out)
}

func UnmergeSettingsHooks(configPath string, kitName string) error {
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
	b, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("编码 hooks 字段失败: %w", err)
	}
	var hooks settingsFile
	if err := json.Unmarshal(b, &hooks); err != nil {
		return fmt.Errorf("解析 hooks 字段失败: %w", err)
	}
	for event, groups := range hooks.Hooks {
		kept := make([]matcherGroup, 0, len(groups))
		for _, g := range groups {
			inner := make([]settingsHook, 0, len(g.Hooks))
			for _, h := range g.Hooks {
				if IsWorkManagedCommand(h.Command, kitName) {
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
	return atomicWriteJSON(configPath, out)
}

func atomicWriteJSON(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".hook-*.json")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	out := append(data, '\n')
	if _, err := tmp.Write(out); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("原子替换文件失败: %w", err)
	}
	cleanup = false
	return nil
}
