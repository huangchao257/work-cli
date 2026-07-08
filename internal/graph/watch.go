package graph

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchOptions controls the watch daemon behavior.
type WatchOptions struct {
	ProjectPath string
	Debounce    time.Duration
}

// Watch 启动文件系统监控守护进程。
// 检测源码文件变更后防抖等待，然后执行 codegraph sync + generate-agents.sh。
// 前台运行，Ctrl+C 退出。
func Watch(ctx context.Context, opts WatchOptions) error {
	root, err := resolveRoot(opts.ProjectPath)
	if err != nil {
		return fmt.Errorf("解析项目根目录失败: %w", err)
	}
	if opts.Debounce <= 0 {
		opts.Debounce = 2 * time.Second
	}

	// 确保 codegraph 索引已初始化
	if err := ensureCodegraph(root, false, true); err != nil {
		return err
	}

	genScript, err := findScript(root, "generate-agents.sh")
	if err != nil {
		return fmt.Errorf("未找到 generate-agents.sh，请先执行: work install codegraph-kit --scope project")
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建文件监控器失败: %w", err)
	}
	defer w.Close()

	// 递归添加目录，排除构建产物和依赖
	if err := addDirs(w, root); err != nil {
		return fmt.Errorf("添加监控目录失败: %w", err)
	}

	fmt.Printf("监听 %s 源码变更（防抖 %v，Ctrl+C 停止）...\n", root, opts.Debounce)

	// 防抖：合并短时间内的连续变更
	var mu sync.Mutex
	var timer *time.Timer

	triggerSync := func() {
		codegraphSync(root)
		if err := runBash(ctx, root, genScript, "--quiet", "--skip-sync", "-p", root); err != nil {
			fmt.Fprintf(os.Stderr, "生成 AGENTS.md 失败: %v\n", err)
		} else {
			fmt.Printf("[%s] AGENTS.md 已更新\n", time.Now().Format("15:04:05"))
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !isSourceFile(event.Name) {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			mu.Lock()
			if timer != nil {
				timer.Stop()
			}
			fmt.Printf("[%s] 检测到变更: %s\n", time.Now().Format("15:04:05"),
				shortPath(root, event.Name))
			timer = time.AfterFunc(opts.Debounce, func() {
				mu.Lock()
				triggerSync()
				mu.Unlock()
			})
			mu.Unlock()

		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "文件监控错误: %v\n", err)
		}
	}
}

// addDirs 递归添加需要监控的目录，跳过 .git/.codegraph/node_modules/vendor 等。
func addDirs(w *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		name := info.Name()
		// 跳过构建产物、依赖与特殊目录
		switch name {
		case ".git", ".codegraph", "node_modules", "vendor", "dist", "build", "target",
			".goreleaser", "__pycache__":
			return filepath.SkipDir
		}
		if strings.HasPrefix(name, ".") && name != "." {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}

// isSourceFile 检查文件是否为需要监控的源代码文件。
func isSourceFile(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".go", ".yaml", ".yml", ".json", ".md", ".py", ".rs", ".ts", ".tsx",
		".js", ".jsx", ".java", ".kt", ".swift", ".c", ".cpp", ".h", ".hpp",
		".rb", ".php", ".scala", ".cs", ".proto":
		return true
	}
	return false
}

// codegraphSync 执行 codegraph sync，失败时静默。
func codegraphSync(root string) {
	cmd := exec.Command("codegraph", "sync", "-q", root)
	cmd.Dir = root
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}

func shortPath(root, full string) string {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return full
	}
	return rel
}
