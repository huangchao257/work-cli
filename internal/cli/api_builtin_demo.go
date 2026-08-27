// Package cli 的内置系统装配。
// api_builtin_demo.go 通过 blank-import 触发 demo 系统向 DefaultRegistry 注册；
// 后续内置系统各增加一个 api_builtin_<system>.go，无需修改既有文件。
package cli

import _ "github.com/huangchao257/work-cli/internal/api/demo"
