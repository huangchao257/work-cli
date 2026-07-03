// Package platform 提供跨平台路径解析、IDE 目录定位与环境变量提示。

package platform

import "runtime"

func EnvSetHint(name string) string {
	switch runtime.GOOS {
	case "windows":
		return "PowerShell: $env:" + name + "=\"你的值\"\nCMD: set " + name + "=你的值"
	default:
		return "export " + name + "=你的值"
	}
}
