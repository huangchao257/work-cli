package bundle

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseDir(t *testing.T) {
	dir := t.TempDir()
	content := `name: test
version: 1.0.0
resources:
  skills:
    - id: s1
      source: ./skills/s1
`
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "test" || m.Version != "1.0.0" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

// FuzzParseBundle 对 bundle.Manifest 的 YAML 反序列化进行模糊测试，
// 验证任意输入不会导致 panic 或 invalid return。
func FuzzParseBundle(f *testing.F) {
	// 种子：有效输入
	f.Add([]byte(`name: test
version: 1.0.0
resources:
  skills:
    - id: s1
      source: ./skills/s1
`))
	f.Add([]byte(`name: empty-resources
version: 2.0.0
`))
	f.Add([]byte(`type: bundle
name: full
version: 3.0.0
description: 完整配置
targets: [qoder, cursor]
env:
  - name: API_KEY
    description: 秘钥
    required: true
resources:
  skills:
    - id: sk1
      source: ./skills/sk1
  rules:
    - id: r1
      source: ./rules/r1
      apply: always
      globs: ["*.go"]
  mcp:
    - id: m1
      source: ./mcp/m1
      env:
        - TOKEN: "${API_KEY}"
post_install:
  when_scope: project
  action: graph_init
`))
	f.Add([]byte(``))
	f.Add([]byte(`a: [1, 2, 3]`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// 对任意字节执行 YAML 反序列化，不应 panic
		var m Manifest
		err := yaml.Unmarshal(data, &m)
		// YAML 可能报错也可能成功，都不应 panic
		// 如果成功解析，再调用 Validate 检查安全性
		if err == nil {
			// Validate 也不应 panic，返回值可忽略
			_ = Validate(&m)
		}
		// 显式忽略 err，仅验证无 panic
		_ = err
	})
}
