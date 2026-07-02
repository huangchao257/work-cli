// Package search 实现 work search 命令：列出可安装的资源（内置 catalog + 可选 Registry 远程清单）。
//
// 仅依赖 Go 标准库与既有依赖（yaml.v3），与 work list（已安装）互补。
package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

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
	Query       string // 子串模糊匹配 name/description，不区分大小写；为空表示不过滤
	Remote      bool   // 是否同时查询 Registry 远程清单
	RegistryURL string // Registry 地址；Remote 为 true 时使用
}

// Result 是一次搜索的汇总结果。
type Result struct {
	Items    []Item   `json:"items"`
	Warnings []string `json:"warnings,omitempty"`
}

// manifestMeta 仅抽取 manifest 中搜索需要的字段。
// 适用于 bundle.yaml / installer.yaml / hooks.yaml 三种清单。
type manifestMeta struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

// Run 执行搜索：本地内置 catalog 收集 + 可选 Registry 远程清单，按 query 过滤后返回。
// 本地或远程收集失败只产生 warning，不返回 error（除非出现不可恢复的内部错误）。
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

// loadBuiltin 遍历内置 catalog，解析每个内置包的 manifest，返回条目与警告列表。
// Resolve 失败或 manifest 解析失败的条目跳过并附带 warning。
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

		manifestFile := manifestFileFor(kind)
		meta, err := parseManifestMeta(filepath.Join(dir, manifestFile))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("跳过内置资源 %s: 解析 %s 失败: %v", name, manifestFile, err))
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

// manifestFileFor 根据清单类型返回对应的 manifest 文件名。
func manifestFileFor(kind manifest.Kind) string {
	return manifest.FileName(kind)
}

// parseManifestMeta 读取并解析 manifest 文件中的 name/version/description。
func parseManifestMeta(path string) (manifestMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifestMeta{}, err
	}
	var meta manifestMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return manifestMeta{}, err
	}
	return meta, nil
}

// registryEntry 是 Registry GET /bundles 返回数组中的单条记录。
type registryEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// fetchRegistry 请求 {url}/bundles，返回远程条目（source 标记为 registry）。
// url 为空时返回 warning「未配置 registry.url，仅显示本地」；请求失败时返回 warning，不阻断。
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
