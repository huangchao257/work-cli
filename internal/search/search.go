// Package search 实现 work search 命令：列出可安装的资源（内置 catalog + 可选 Registry 远程清单）。
package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/huangchao257/work-cli/internal/catalog"
	"github.com/huangchao257/work-cli/internal/pkg/manifest"
)

// Item 表示一条可安装资源条目。
type Item struct {
	Name        string `json:"name" yaml:"name"`
	Version     string `json:"version" yaml:"version"`
	Type        string `json:"type" yaml:"type"`
	Description string `json:"description" yaml:"description"`
	Source      string `json:"source" yaml:"source"`
}

// Options 控制 Run 的行为。
type Options struct {
	Query       string
	Remote      bool
	RegistryURL string
}

// Result 是一次搜索的汇总结果。
type Result struct {
	Items    []Item   `json:"items"`
	Warnings []string `json:"warnings,omitempty"`
}

// Run 执行搜索：本地内置 catalog 收集 + 可选 Registry 远程清单。
func Run(opts Options) (Result, error) {
	result := Result{Items: []Item{}, Warnings: []string{}}

	items, warns := loadBuiltin()
	result.Items = append(result.Items, items...)
	result.Warnings = append(result.Warnings, warns...)

	if opts.Remote {
		remoteItems, warn := fetchRegistry(opts.RegistryURL)
		if warn != "" {
			result.Warnings = append(result.Warnings, warn)
		}
		result.Items = append(result.Items, remoteItems...)
	}

	if q := strings.TrimSpace(opts.Query); q != "" {
		filtered := result.Items[:0]
		needle := strings.ToLower(q)
		for _, it := range result.Items {
			if strings.Contains(strings.ToLower(it.Name), needle) ||
				strings.Contains(strings.ToLower(it.Description), needle) {
				filtered = append(filtered, it)
			}
		}
		result.Items = filtered
	}

	return result, nil
}

func loadBuiltin() ([]Item, []string) {
	items := []Item{}
	warnings := []string{}

	for _, name := range catalog.Names() {
		dir, ok := catalog.Resolve(name)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("跳过内置资源 %s: 无法解析目录", name))
			continue
		}

		kind, err := manifest.DetectKind(dir)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("跳过内置资源 %s: %v", name, err))
			continue
		}

		mf := manifest.FileName(kind)
		meta, err := manifest.ReadMeta(filepath.Join(dir, mf))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("跳过内置资源 %s: 解析 %s 失败: %v", name, mf, err))
			continue
		}

		items = append(items, Item{
			Name:        meta.Name,
			Version:     meta.Version,
			Type:        string(kind),
			Description: meta.Description,
			Source:      "builtin",
		})
	}

	return items, warnings
}

type registryEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

func fetchRegistry(url string) ([]Item, string) {
	if strings.TrimSpace(url) == "" {
		return nil, "未配置 registry.url，仅显示本地"
	}

	endpoint := strings.TrimRight(url, "/") + "/bundles"
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Sprintf("请求 Registry 失败（%s）: %v", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Sprintf("Registry 返回错误: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Sprintf("读取 Registry 响应失败: %v", err)
	}

	var entries []registryEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Sprintf("解析 Registry 响应失败: %v", err)
	}

	items := make([]Item, 0, len(entries))
	for _, e := range entries {
		items = append(items, Item{
			Name:        e.Name,
			Version:     e.Version,
			Type:        e.Type,
			Description: e.Description,
			Source:      "registry",
		})
	}
	return items, ""
}
