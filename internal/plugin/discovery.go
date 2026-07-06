// Package plugin 提供 work CLI 的插件系统骨架。
// discovery.go 负责扫描 ~/.work/plugins/ 目录，解析 plugin.yaml 清单文件，
// 并返回发现的所有插件。
package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/huangchao257/work-cli/internal/platform"
)

// PluginsDir 返回插件目录路径 ~/.work/plugins/，按需创建。
func PluginsDir() (string, error) {
	return platform.WorkSubDir("plugins")
}

// Discover 扫描 ~/.work/plugins/ 目录，返回所有发现的插件清单。
// 每个子目录需含一个 plugin.yaml 文件；解析失败时跳过并继续。
func Discover() ([]Manifest, error) {
	dir, err := PluginsDir()
	if err != nil {
		return nil, fmt.Errorf("定位插件目录失败: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取插件目录失败: %w", err)
	}

	var plugins []Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "plugin.yaml")
		m, err := parseManifest(manifestPath)
		if err != nil {
			// 解析失败时跳过，不中断整体扫描
			continue
		}
		if m.Name == "" {
			continue
		}
		// Name 必须与目录名一致
		if m.Name != entry.Name() {
			continue
		}
		plugins = append(plugins, m)
	}
	return plugins, nil
}

// Find 按名称查找插件，未找到返回 ErrNotFound。
func Find(name string) (*Manifest, error) {
	plugins, err := Discover()
	if err != nil {
		return nil, err
	}
	for i := range plugins {
		if plugins[i].Name == name {
			return &plugins[i], nil
		}
	}
	return nil, ErrNotFound
}

// parseManifest 解析单个 plugin.yaml 文件。
func parseManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("解析插件清单失败: %w", err)
	}
	return m, nil
}
