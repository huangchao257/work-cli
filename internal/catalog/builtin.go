package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// builtins maps registry names to example package directory names.
var builtins = map[string]string{
	"codegraph-stack": "codegraph-stack",
	"codegraph-kit":   "codegraph-kit",
	"codegraph":       "codegraph",
	"dev-kit":         "dev-kit",
	"company-hooks":   "company-hooks",
	"openspec":        "openspec",
	"openspec-mock":   "openspec-mock",
}

// Resolve returns the local directory for a built-in package name.
func Resolve(name string) (string, bool) {
	dir, ok := builtins[name]
	if !ok {
		return "", false
	}
	root, err := examplesRoot()
	if err != nil {
		return "", false
	}
	path := filepath.Join(root, dir)
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		return "", false
	}
	manifestOK := fileExists(filepath.Join(path, "installer.yaml")) ||
		fileExists(filepath.Join(path, "bundle.yaml")) ||
		fileExists(filepath.Join(path, "hooks.yaml"))
	if !manifestOK {
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	return abs, true
}

// Names returns sorted built-in package names (for docs/tests).
func Names() []string {
	out := make([]string, 0, len(builtins))
	for n := range builtins {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func examplesRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("WORK_EXAMPLES_DIR")); v != "" {
		return filepath.Abs(v)
	}
	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, ".work", "examples")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return filepath.Abs(p)
		}
	}
	if exe, err := os.Executable(); err == nil {
		exe, _ = filepath.EvalSymlinks(exe)
		dir := filepath.Dir(exe)
		for _, rel := range []string{
			"examples",
			filepath.Join("..", "examples"),
			filepath.Join("..", "share", "work", "examples"),
		} {
			p := filepath.Join(dir, rel)
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				return filepath.Abs(p)
			}
		}
	}
	// 开发：从模块源码树定位 examples（go test / go run）
	if wd, err := os.Getwd(); err == nil {
		if p := findExamplesUp(wd); p != "" {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到内置套装目录")
}

func findExamplesUp(start string) string {
	dir := start
	for i := 0; i < 8; i++ {
		p := filepath.Join(dir, "examples")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			if fileExists(filepath.Join(p, "dev-kit", "bundle.yaml")) {
				abs, _ := filepath.Abs(p)
				return abs
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
