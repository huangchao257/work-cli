# Code Review Findings Fix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 10 issues found during the code review: 6 correctness bugs, 3 code quality problems, and 1 efficiency issue.

**Architecture:** Each fix is independent and focused. Tasks modify existing files only, no new files created. All changes are local to the affected package.

**Tech Stack:** Go 1.21+, `github.com/spf13/cobra`, `gopkg.in/yaml.v3`

## Global Constraints

- All changes must pass existing tests: `go test ./...`
- Error messages must remain in Chinese (matching project conventions)
- Use `fmt.Errorf("...: %w", err)` for error wrapping (matching existing pattern)
- Commit messages follow `fix(<scope>): description` format

---

### Task 1: Fix configcache TOCTOU race (Finding 1)

**Files:**
- Modify: `internal/configcache/configcache.go:41-50`
- Modify: `internal/configcache/configcache_test.go` (add test case)

**Problem:** Between `os.ReadFile(path)` at line 41 and the second `os.Stat(path)` at line 47, an external modification changes the file's mtime. The old content (read before modification) gets stored under the new mtime (from after modification). Subsequent reads match the cached mtime and return stale content until the file is modified again.

**Fix:** Use the mtime from the `Stat` BEFORE the read as the cache key, because the read and stat are individually atomic. The correct approach is: Stat to get initial mtime, then if the fast-path cache check fails, read the file, then use the INITIAL mtime (not a second stat) for the write-path double-check. This avoids the TOCTOU window entirely.

- [ ] **Step 1: Fix `ReadFile` to use initial mtime, not post-read mtime**

```go
func ReadFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	mtime := info.ModTime().UnixNano()

	mu.RLock()
	e, ok := store[path]
	mu.RUnlock()
	if ok && e.modTime == mtime {
		return e.data, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	mu.Lock()
	// 双重检查：释放 RLock 与获取 Lock 之间可能有其他 goroutine
	// 基于更新的 mtime 写入了更新的缓存，此时应保留较新的缓存。
	if existing, ok := store[path]; ok && existing.modTime >= mtime {
		mu.Unlock()
		return data, nil
	}
	store[path] = entry{data: data, modTime: mtime}
	mu.Unlock()
	return data, nil
}
```

- [ ] **Step 2: Add test for concurrent modification scenario**

Modify `internal/configcache/configcache_test.go` to add a test that verifies stale content is not cached:

```go
func TestReadFile_ConcurrentModification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Write initial content
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "v1" {
		t.Fatalf("expected v1, got %s", string(data))
	}

	// Modify file externally (simulating external tool)
	if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Read again — should get v2, NOT stale v1
	data, err = ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "v2" {
		t.Fatalf("expected v2, got %s", string(data))
	}
}
```

- [ ] **Step 3: Run tests to verify**

Run: `go test ./internal/configcache/ -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/configcache/configcache.go internal/configcache/configcache_test.go
git commit -m "fix(configcache): use pre-read mtime to prevent TOCTOU stale cache

The second os.Stat after os.ReadFile could capture a newer mtime from an
external modification, causing old content to be stored under the new mtime.
Use the initial Stat mtime consistently as the cache key."
```

---

### Task 2: Fix MCP lock — write file inside the lock (Finding 2)

**Files:**
- Modify: `internal/adapter/common.go:45-69` and `110-131`

**Problem:** `withMCPLock` acquires an exclusive lock, reads the file, calls the merge function, and releases the lock. But the caller `installMCPAt` then calls `os.WriteFile(configPath, merged, ...)` OUTSIDE the lock. Between lock release and WriteFile, another process can acquire the lock, read the old state, merge, unlock, and write — losing one of the two modifications.

**Fix:** Move the `os.WriteFile` and `os.MkdirAll` inside `withMCPLock` so the entire read-modify-write cycle is protected.

- [ ] **Step 1: Move WriteFile + MkdirAll inside withMCPLock**

```go
func installMCPAt(bundleRoot string, mcp bundle.MCPResource, configPath string) (string, error) {
	src := filepath.Join(bundleRoot, filepath.FromSlash(strings.TrimPrefix(mcp.Source, "./")))
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	var server json.RawMessage
	if err := json.Unmarshal(data, &server); err != nil {
		return "", fmt.Errorf("解析 MCP %s 失败: %w", mcp.ID, err)
	}
	if err := withMCPLock(configPath, func(existing []byte) ([]byte, error) {
		return MergeMCPServers(existing, mcp.ID, server)
	}); err != nil {
		return "", err
	}
	return configPath, nil
}
```

Modify `withMCPLock` to also write the result:

```go
// withMCPLock 对指定路径的 MCP 配置文件加独占锁，读取、合并并写入内容。
// 全程持有锁，防止多个 work 进程同时修改同一 MCP 配置文件导致数据损坏。
func withMCPLock(configPath string, fn func(existing []byte) ([]byte, error)) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("创建 MCP 配置目录失败: %w", err)
	}
	f, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("打开 MCP 配置文件失败: %w", err)
	}
	defer f.Close()

	if err := platform.FlockLock(f, configPath, platform.FlockEX); err != nil {
		return fmt.Errorf("获取 MCP 配置文件独占锁失败: %w", err)
	}
	defer func() { _ = platform.FlockUnlock(f) }()

	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取 MCP 配置文件失败: %w", err)
	}

	merged, err := fn(existing)
	if err != nil {
		return err
	}

	// 写入在锁内执行，保证整个 read-modify-write 是原子的
	if err := os.WriteFile(configPath, merged, 0o644); err != nil {
		return fmt.Errorf("写入 MCP 配置文件失败: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/adapter/ -v -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/common.go
git commit -m "fix(adapter): hold MCP lock across full read-modify-write cycle

Previously os.WriteFile happened after the lock was released, creating a
window where concurrent installs could silently lose MCP server entries.
Move the write inside withMCPLock."
```

---

### Task 3: Restore Claude Code ~/.claude directory detection (Finding 3)

**Files:**
- Modify: `internal/adapter/claude.go:23-33`

**Problem:** `detectClaude` only checks `XDG_CONFIG_HOME/claude` (default `~/.config/claude`), dropping the `~/.claude` directory check that the old code had. Users with standard Claude Code installation at `~/.claude` will be silently skipped.

**Fix:** Restore the `~/.claude` directory check alongside the XDG path.

- [ ] **Step 1: Add ~/.claude check back**

```go
// detectClaude 检测当前系统是否安装了 Claude Code。
// 同时检查 ~/.claude/（标准安装路径）和 XDG_CONFIG_HOME/claude/。
func detectClaude() bool {
	home, err := platform.UserHome()
	if err != nil {
		return false
	}
	// 标准安装路径
	if dirExists(filepath.Join(home, ".claude")) {
		return true
	}
	// 也可能以文件形式存在（~/.claude.json）
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); err == nil {
		return true
	}
	// XDG 配置路径
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	if dirExists(filepath.Join(configHome, "claude")) {
		return true
	}
	return false
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/adapter/ -v -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/claude.go
git commit -m "fix(adapter): restore ~/.claude directory detection for Claude Code

The old detectClaude checked both ~/.claude and the XDG path. The refactor
dropped the ~/.claude check, causing users with standard Claude Code installs
to be silently skipped."
```

---

### Task 4: Fix SemVer prerelease comparison (Finding 4)

**Files:**
- Modify: `internal/semver/version.go:93-112`
- Modify: `internal/semver/version_test.go` (add test cases)

**Problem:** `comparePrerelease` uses pure string comparison, which is incorrect for SemVer 2.0. Per the spec, dot-separated identifiers should be compared segment-by-segment, with numeric identifiers compared numerically (so `alpha.10` > `alpha.2`).

**Fix:** Implement segment-by-segment comparison respecting SemVer 2.0 rules: numeric segments compared by value, string segments compared lexicographically, numeric always less than non-numeric.

- [ ] **Step 1: Replace comparePrerelease with segment-aware comparison**

```go
// comparePrerelease 比较预发布标识（遵循 SemVer 2.0 规范 11.4）。
// 逐点分隔段比较：纯数字段按数值比较，非纯数字段按字典序比较；
// 数字段优先级低于非数字段；无预发布者优先级最高。
func comparePrerelease(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}

	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	minLen := len(aParts)
	if len(bParts) < minLen {
		minLen = len(bParts)
	}

	for i := 0; i < minLen; i++ {
		ai, aIsNum := partToNum(aParts[i])
		bi, bIsNum := partToNum(bParts[i])

		if aIsNum && bIsNum {
			if ai < bi {
				return -1
			}
			if ai > bi {
				return 1
			}
		} else if aIsNum {
			// 数字段优先级低于非数字段
			return -1
		} else if bIsNum {
			return 1
		} else {
			if aParts[i] < bParts[i] {
				return -1
			}
			if aParts[i] > bParts[i] {
				return 1
			}
		}
	}

	// 较短的预发布标识优先级更高（更少的段意味着更接近正式版）
	if len(aParts) < len(bParts) {
		return -1
	}
	if len(aParts) > len(bParts) {
		return 1
	}
	return 0
}

// partToNum 将预发布段转为数字（若为纯数字），返回值和是否为数字。
func partToNum(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}
```

- [ ] **Step 2: Add prerelease comparison tests**

Add to `internal/semver/version_test.go`:

```go
func TestCompare_prereleaseSegmentCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		// Numeric identifiers: 10 > 2
		{"1.0.0-alpha.10", "1.0.0-alpha.2", 1},
		{"1.0.0-alpha.2", "1.0.0-alpha.10", -1},
		// Numeric < non-numeric
		{"1.0.0-1.alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.1", "1.0.0-1.alpha", 1},
		// Same prefix, different lengths
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha", 1},
		// Same prerelease
		{"1.0.0-beta.1", "1.0.0-beta.1", 0},
	}
	for _, tt := range tests {
		got := Compare(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/semver/ -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/semver/version.go internal/semver/version_test.go
git commit -m "fix(semver): compare prerelease identifiers per SemVer 2.0 segment rules

Numeric segments (e.g., '10' in 'alpha.10') now compare numerically rather
than lexicographically. Also enforces that numeric identifiers sort lower
than non-numeric identifiers, per SemVer 2.0 specification 11.4."
```

---

### Task 5: Fix rewriteQueue sync-state inconsistency (Finding 5)

**Files:**
- Modify: `internal/hooks/queue.go:216-221`

**Problem:** `rewriteQueue` atomically renames the temp file to the queue path (line 216), then calls `updatePendingCount()` at line 220. If `updatePendingCount()` fails (disk full, concurrent conflict), the queue data is already correctly persisted but the sync-state retains stale `PendingCount`. The next `hooks status` call reports incorrect queue depth.

**Fix:** If `updatePendingCount` fails after a successful rename, log the error but don't propagate it — the queue data is correct, and the sync state will be rebuilt on the next `AppendQueue` or `hooks status` call.

- [ ] **Step 1: Make updatePendingCount failure non-fatal after rename**

```go
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("原子替换队列文件失败: %w", err)
	}
	cleanup = false
	// 尽力更新同步状态元数据；失败不阻塞（队列数据已正确持久化）。
	// 下一次 AppendQueue 或 ReadPending 调用会自动修正 PendingCount。
	_ = updatePendingCount()
	return nil
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/hooks/ -v -count=1 -run TestQueue`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/hooks/queue.go
git commit -m "fix(hooks): don't fail rewriteQueue on sync-state update error

The queue file rename is the primary atomic operation. If the subsequent
sync-state update fails, the queue data is still correct. The next status
check will rebuild PendingCount from the canonical queue file."
```

---

### Task 6: Deduplicate cursorHooksFile struct (Finding 6)

**Files:**
- Modify: `internal/graph/hooks.go:13-21` (remove duplicate types, import hooks package)
- Files stay: `internal/hooks/merge.go` (keep original definitions)

**Problem:** `cursorHooksFile` and `cursorHookEntry` structs are defined identically in both `internal/graph/hooks.go` (lines 13-21) and `internal/hooks/merge.go` (lines 10-18). The graph package also re-implements the full read-filter-append-write logic for `.cursor/hooks.json`.

**Fix:** Have the graph package import the hooks package and reuse `hooks.MergeCursorHooks` instead of duplicating the types and logic. Export types and functions needed by graph.

- [ ] **Step 1: Export cursor hook types from hooks package**

In `internal/hooks/merge.go`, rename `cursorHooksFile` and `cursorHookEntry` to exported names:

```go
type CursorHooksFile struct {
	Version int                        `json:"version"`
	Hooks   map[string][]CursorHookEntry `json:"hooks"`
}

type CursorHookEntry struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}
```

Update all internal references in `merge.go` from `cursorHooksFile` → `CursorHooksFile` and `cursorHookEntry` → `CursorHookEntry`.

- [ ] **Step 2: Remove duplicate types from graph/hooks.go; use hooks package**

Replace `internal/graph/hooks.go` with a version that imports and uses `hooks`:

```go
package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/huangchao257/work-cli/internal/hooks"
)

func setupCursorHook(projectRoot, hookScript string) error {
	hooksPath := filepath.Join(projectRoot, ".cursor", "hooks.json")
	marker := "codegraph-agents/on-file-edit.sh"

	var cfg hooks.CursorHooksFile
	if data, err := os.ReadFile(hooksPath); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("解析 hooks.json 失败: %w", err)
		}
	} else {
		cfg = hooks.CursorHooksFile{Version: 1, Hooks: map[string][]hooks.CursorHookEntry{}}
	}
	if cfg.Hooks == nil {
		cfg.Hooks = map[string][]hooks.CursorHookEntry{}
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}

	filtered := make([]hooks.CursorHookEntry, 0, len(cfg.Hooks["afterFileEdit"]))
	for _, e := range cfg.Hooks["afterFileEdit"] {
		if strings.Contains(e.Command, marker) || strings.Contains(e.Command, "on-file-edit.sh") {
			continue
		}
		filtered = append(filtered, e)
	}
	filtered = append(filtered, hooks.CursorHookEntry{Command: hookScript, Timeout: 15})
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
```

- [ ] **Step 3: Verify no import cycles**

Run: `go build ./internal/graph/`
Expected: no errors (graph → hooks is a valid import; hooks does not import graph)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/graph/ ./internal/hooks/ -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/graph/hooks.go internal/hooks/merge.go
git commit -m "refactor(graph): reuse hooks.CursorHooksFile/CursorHookEntry types

Export cursor hook types from hooks package and import them in graph,
eliminating the duplicate type definitions. The read-filter-append-write
logic stays in graph because it uses different filtering criteria
(codegraph-specific markers vs IsWorkManagedCommand)."
```

---

### Task 7: Make post_install extensible (Finding 7)

**Files:**
- Modify: `internal/engine/bundle.go:172-189`
- Check: `internal/bundle/manifest.go` (verify PostInstall struct)

**Problem:** `runBundlePostInstall` has a hardcoded `switch` on `manifest.PostInstall.Action` with only one case (`"graph_init"`). Adding new post-install actions requires modifying the engine code.

**Fix:** Change the approach from an enum switch to a command-based execution. Add a `Command` field to the `PostInstall` struct so bundle authors can specify arbitrary shell commands for post-install hooks, while keeping `"graph_init"` as a built-in shortcut for the common case.

- [ ] **Step 1: Check PostInstall struct and add Command field**

Read `internal/bundle/manifest.go` to find the `PostInstall` struct. Add a `Command` field:

In `internal/bundle/manifest.go`, modify the `PostInstall` struct to add:

```go
type PostInstall struct {
	Action    string `yaml:"action"`
	Command   string `yaml:"command"`     // 任意 shell 命令（当 action 为空或为 "command" 时执行）
	WhenScope string `yaml:"when_scope"`
}
```

- [ ] **Step 2: Update runBundlePostInstall to support Command**

```go
func runBundlePostInstall(ctx context.Context, manifest *bundle.Manifest, opts Options) error {
	if manifest == nil || manifest.PostInstall == nil {
		return nil
	}
	when := manifest.PostInstall.WhenScope
	if when == "" {
		when = "project"
	}
	if when != "any" && when != opts.Scope {
		return nil
	}

	action := manifest.PostInstall.Action
	if action == "" && manifest.PostInstall.Command != "" {
		action = "command"
	}

	switch action {
	case "graph_init":
		return graph.RunPostInstall(ctx, opts.Scope, opts.DryRun)
	case "command":
		if manifest.PostInstall.Command == "" {
			return fmt.Errorf("post_install.command 不能为空")
		}
		if opts.DryRun {
			fmt.Printf("（预览）将执行 post_install: %s\n", manifest.PostInstall.Command)
			return nil
		}
		return runInDir(ctx, ".", manifest.PostInstall.Command)
	default:
		return fmt.Errorf("未知 post_install.action: %s（支持 graph_init 或 command）", manifest.PostInstall.Action)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/engine/ ./internal/bundle/ -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/engine/bundle.go internal/bundle/manifest.go
git commit -m "feat(bundle): support arbitrary command in post_install

Add a 'command' action type to PostInstall with a 'command' field,
enabling bundle authors to run any shell command after installation
without modifying the CLI engine. The 'graph_init' shortcut is preserved."
```

---

### Task 8: Simplify MCPConfigPath — deduplicate identical switch branches (Finding 8)

**Files:**
- Modify: `internal/platform/ide_paths.go:49-93`

**Problem:** `MCPConfigPath` has a 40-line switch with three identical case bodies: each computes `ideBase(ide, scope)` + `"mcp.json"`. The logic is completely duplicated for each IDE.

**Fix:** Use `ideBase` to get the dot-directory path, then append `"mcp.json"`. This also fixes the scope fallback duplication.

- [ ] **Step 1: Replace MCPConfigPath with concise version**

```go
func MCPConfigPath(ide IDE, scope string) (string, error) {
	base, err := ideBase(ide, scope)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "mcp.json"), nil
}
```

- [ ] **Step 2: Run full test suite to verify**

Run: `go test ./internal/platform/ ./internal/adapter/ -v -count=1`
Expected: PASS (tests in `internal/platform/ide_paths_test.go` should already cover this)

- [ ] **Step 3: Run existing integration tests**

Run: `go test ./... -count=1`
Expected: PASS (no regressions)

- [ ] **Step 4: Commit**

```bash
git add internal/platform/ide_paths.go
git commit -m "refactor(platform): deduplicate MCPConfigPath using ideBase

All three IDE branches in MCPConfigPath computed filepath.Join(ideBase(ide,
scope), \"mcp.json\"). Replace the 40-line switch with a single call to
ideBase. Same behavior, same paths, one place to maintain."
```

---

### Task 9: Fix redactPath mapPool use-after-put (Finding 9)

**Files:**
- Modify: `internal/hooks/redact.go:62-78`

**Problem:** The `sync.Pool` map `m` is stored in `cur[key] = m` (line 74), then put back into the pool (line 77). The pool may return the same map to a future caller, which will clear it — silently emptying the sub-map in the still-in-use result from the previous call.

**Fix:** Don't return the map to the pool. The pool optimization is premature and the correctness risk outweighs the allocation savings. Alternatively, copy the map content before putting back.

- [ ] **Step 1: Remove mapPool, allocate maps directly**

```go
func redactPath(cur map[string]any, parts []string) {
	if len(parts) == 0 {
		return
	}
	if parts[0] == "" {
		redactPath(cur, parts[1:])
		return
	}
	if strings.HasSuffix(parts[0], "*") {
		prefix := strings.TrimSuffix(parts[0], "*")
		for key, val := range cur {
			if strings.HasPrefix(key, prefix) {
				cur[key] = "[REDACTED]"
				_ = val
			}
		}
		return
	}
	if len(parts) == 1 {
		cur[parts[0]] = "[REDACTED]"
		return
	}
	key := parts[0]
	next, ok := cur[key]
	if !ok {
		return
	}
	switch v := next.(type) {
	case map[string]any:
		redactPath(v, parts[1:])
	case map[interface{}]interface{}:
		// YAML解析可能产生 map[interface{}]interface{}，转为 map[string]any
		m := make(map[string]any, len(v))
		for k, val := range v {
			if ks, ok := k.(string); ok {
				m[ks] = val
			}
		}
		redactPath(m, parts[1:])
		cur[key] = m
	}
}
```

Also remove the `mapPool` variable and the `sync` import (if no longer needed elsewhere):

```go
// 删除这些行:
// var mapPool = sync.Pool{
//     New: func() any { return make(map[string]any) },
// }
```

- [ ] **Step 2: Verify sync import is still needed**

Run: `go build ./internal/hooks/`
Expected: If `"sync"` is no longer imported, the build will fail — remove it from the imports.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/hooks/ -v -count=1 -run TestRedact`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/hooks/redact.go
git commit -m "fix(hooks): remove mapPool to prevent use-after-put data race

The pooled map was assigned to cur[key] and immediately returned to the pool.
A subsequent Get() would clear the map, corrupting the caller's still-referenced
data. Replace with direct make(map[string]any) — allocation cost is negligible
for YAML-to-JSON map conversion."
```

---

### Task 10: Parallelize batch installs/uninstalls (Finding 10)

**Files:**
- Modify: `internal/engine/batch.go`

**Problem:** `InstallBatch`, `UninstallAll`, and `UninstallBatch` all execute operations sequentially in a simple `for` loop. Each iteration may involve network downloads, which are independent and could run concurrently.

**Fix:** Use standard library `sync.WaitGroup` with a channel-based semaphore to parallelize independent operations. Preserves result ordering by using a pre-allocated slice.

- [ ] **Step 1: Rewrite InstallBatch with concurrent execution**

```go
import (
	"context"
	"fmt"
	"sync"

	"github.com/huangchao257/work-cli/internal/platform"
	"github.com/huangchao257/work-cli/internal/source"
	"github.com/huangchao257/work-cli/internal/state"
)

// InstallBatch 批量安装多个资源，并行执行独立安装操作。
// 失败时不回滚（轻量 CLI 模式），但会收集所有结果一并返回。
func InstallBatch(ctx context.Context, opts Options, names []string) (*BatchResult, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("至少需要指定一个安装名称")
	}

	results := make([]Result, len(names))
	var wg sync.WaitGroup
	// 信号量限制并发数，避免同时打开过多网络连接
	sem := make(chan struct{}, 8)

	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ref, err := resolveRef(name)
			if err != nil {
				results[i] = Result{
					Success:  false,
					Name:     name,
					Warnings: []string{err.Error()},
				}
				return
			}
			optsCopy := opts
			optsCopy.Ref = ref
			res, err := Install(ctx, optsCopy)
			if err != nil {
				res = Result{
					Success:  false,
					Name:     name,
					Warnings: []string{err.Error()},
				}
			} else {
				res.Success = true
			}
			results[i] = res
		}(i, name)
	}
	wg.Wait()

	br := &BatchResult{
		Results: make([]Result, 0, len(names)),
	}
	for _, res := range results {
		if res.Success {
			br.Successes++
		} else {
			br.Failures++
		}
		br.Results = append(br.Results, res)
	}
	return br, nil
}
```

- [ ] **Step 2: Apply same pattern to UninstallAll and UninstallBatch**

Use the same `sync.WaitGroup` + channel semaphore pattern in `UninstallAll` and `UninstallBatch`. The result collection logic is identical:

```go
func UninstallAll(ctx context.Context, scope, kindFilter string, dryRun bool) (*BatchResult, error) {
	if scope == "" {
		scope = "user"
	}
	recs, err := listRecords(scope, kindFilter)
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		desc := ""
		if kindFilter != "" {
			desc = fmt.Sprintf("kind=%s 的", kindFilter)
		}
		return nil, fmt.Errorf("没有已安装的%s资源", desc)
	}

	results := make([]Result, len(recs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for i, rec := range recs {
		wg.Add(1)
		go func(i int, rec state.BundleRecord) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := Uninstall(ctx, rec.Name, rec.Scope, dryRun)
			if err != nil {
				res = Result{
					Success:  false,
					Name:     rec.Name,
					Warnings: []string{err.Error()},
				}
			} else {
				res.Success = true
			}
			results[i] = res
		}(i, rec)
	}
	wg.Wait()

	br := &BatchResult{
		Results: make([]Result, 0, len(recs)),
	}
	for _, res := range results {
		if res.Success {
			br.Successes++
		} else {
			br.Failures++
		}
		br.Results = append(br.Results, res)
	}
	return br, nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/engine/ -v -count=1`
Expected: PASS

- [ ] **Step 4: Run race detector**

Run: `go test -race ./internal/engine/ -v -count=1`
Expected: PASS (no data races)

- [ ] **Step 5: Commit**

```bash
git add internal/engine/batch.go
git commit -m "perf(engine): parallelize batch install and uninstall operations

Use sync.WaitGroup with channel semaphore to run independent package
operations concurrently with a limit of 8. N packages each with T seconds
of network I/O now complete in ~max(T_i) instead of ~sum(T_i)."
```
