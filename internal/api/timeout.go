package api

import (
	"time"
)

const defaultTimeout = 30 * time.Second

// TimeoutOptions 对应 ~/.work/config.yaml 的 api.timeout 配置。
// 仅声明 yaml tag（由配置层统一反序列化），本包不直接依赖 yaml 库。
type TimeoutOptions struct {
	API struct {
		Timeout string `yaml:"timeout"`
	} `yaml:"api"`
}

// parseTimeout 解析超时字符串（如 "30s"、"1m"）；空值使用默认 30s，非法值返回错误。
func parseTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		return defaultTimeout, nil
	}
	dur, err := time.ParseDuration(raw)
	if err != nil || dur <= 0 {
		return 0, yamlTimeoutError(raw)
	}
	return dur, nil
}

type timeoutError struct{ raw string }

func (e *timeoutError) Error() string {
	return "api.timeout 配置非法: " + e.raw + "（示例：30s、1m）"
}

func yamlTimeoutError(raw string) error { return &timeoutError{raw: raw} }
