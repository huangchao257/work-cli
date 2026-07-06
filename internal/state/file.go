// Package state 提供已安装资源的状态持久化。
// file.go 定义状态文件的顶层结构、版本号与 JSON 序列化。
package state

import (
	"encoding/json"
	"fmt"
)

// CurrentVersion 是 installed.json 当前支持的格式版本号。
const CurrentVersion = 1

// File 是 installed.json 的顶层结构，包含版本号和已安装记录列表。
type File struct {
	Version int            `json:"version"`
	Bundles []BundleRecord `json:"bundles"`
}

// MarshalJSON 持久化 File，始终写入当前版本号。
func (f *File) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Version int            `json:"version"`
		Bundles []BundleRecord `json:"bundles"`
	}{
		Version: CurrentVersion,
		Bundles: f.Bundles,
	})
}

// UnmarshalJSON 解析 File，处理向后兼容：
// - 旧格式无 version 字段时默认为版本 1
// - 版本高于当前支持时返回明确错误
func (f *File) UnmarshalJSON(data []byte) error {
	var raw struct {
		Version int            `json:"version"`
		Bundles []BundleRecord `json:"bundles"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("解析状态文件失败: %w", err)
	}
	if raw.Version == 0 {
		raw.Version = 1 // 旧格式无版本号，视为版本 1
	}
	if raw.Version > CurrentVersion {
		return fmt.Errorf("状态文件版本 %d 不受支持，当前支持版本 %d，请升级 work CLI", raw.Version, CurrentVersion)
	}
	f.Version = raw.Version
	f.Bundles = raw.Bundles
	return nil
}

// Migrate 将 File 从当前版本迁移至最新版本。若版本已是最新，则直接返回 nil。
// 迁移步骤通过注册表（migrate.go）按顺序执行。
func (f *File) Migrate() error {
	for {
		if f.Version >= CurrentVersion {
			return nil
		}
		m, ok := migrations[f.Version]
		if !ok {
			return fmt.Errorf("状态文件版本 %d 没有可用的迁移路径", f.Version)
		}
		if err := m.Apply(f); err != nil {
			return fmt.Errorf("状态文件从版本 %d 迁移到版本 %d 失败: %w", m.From, m.To, err)
		}
	}
}
