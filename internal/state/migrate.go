// Package state 提供已安装资源的状态持久化。
// migrate.go 定义状态文件格式（版本）迁移注册表与迁移步骤。
package state

// Migration 表示从版本 From 到版本 To 的格式迁移步骤。
type Migration struct {
	From  int
	To    int
	Apply func(*File) error
}

// migrations 是按 From 版本号索引的迁移步骤注册表。
// key = 当前版本号，value = 升级到下一个版本的迁移步骤。
//
// 当前无实际迁移步骤（版本 1 是最初版本），注册表已预置以供未来扩展：
//
//	migrations[1] = Migration{From: 1, To: 2, Apply: func(f *File) error {
//	    // 例如：将新字段补上默认值
//	    for i := range f.Bundles {
//	        if f.Bundles[i].Scope == "" {
//	            f.Bundles[i].Scope = "user"
//	        }
//	    }
//	    return nil
//	}}
var migrations = map[int]Migration{}
