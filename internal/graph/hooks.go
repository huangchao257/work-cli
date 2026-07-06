// Package graph 封装 codegraph CLI，提供知识图谱 init/sync/status 与 AGENTS.md 自动同步。

package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type cursorHooksFile struct {
	Version int                          `json:"version"`
	Hooks   map[string][]cursorHookEntry `json:"hooks"`
}

type cursorHookEntry struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

func setupCursorHook(projectRoot, hookScript string) error {
	hooksPath := filepath.Join(projectRoot, ".cursor", "hooks.json")
	marker := "codegraph-agents/on-file-edit.sh"

	var cfg cursorHooksFile
	if data, err := os.ReadFile(hooksPath); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("解析 hooks.json 失败: %w", err)
		}
	} else {
		cfg = cursorHooksFile{Version: 1, Hooks: map[string][]cursorHookEntry{}}
	}
	if cfg.Hooks == nil {
		cfg.Hooks = map[string][]cursorHookEntry{}
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}

	filtered := make([]cursorHookEntry, 0, len(cfg.Hooks["afterFileEdit"]))
	for _, e := range cfg.Hooks["afterFileEdit"] {
		if strings.Contains(e.Command, marker) || strings.Contains(e.Command, "on-file-edit.sh") {
			continue
		}
		filtered = append(filtered, e)
	}
	filtered = append(filtered, cursorHookEntry{Command: hookScript, Timeout: 15})
	cfg.Hooks["afterFileEdit"] = filtered

	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		return fmt.Errorf("创建 hooks 目录失败: %w", err)
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 hooks.json 失败: %w", err)
	}
	if err := os.WriteFile(hooksPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入 hooks.json 失败: %w", err)
	}
	return nil
}

func hookConfigured(projectRoot string) bool {
	data, err := os.ReadFile(filepath.Join(projectRoot, ".cursor", "hooks.json"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "codegraph-agents") || strings.Contains(string(data), "on-file-edit.sh")
}
