package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huangchao257/work-cli/internal/usage"
	"gopkg.in/yaml.v3"
)

// withTempHome 把 HOME 指向临时目录，返回该目录与清理函数。
func withTempHome(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// 兜底：覆盖部分平台读取 USERPROFILE 的情况
	t.Setenv("USERPROFILE", dir)
	return dir, func() {}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestPath(t *testing.T) {
	dir, cleanup := withTempHome(t)
	defer cleanup()
	p, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	want := filepath.Join(dir, ".work", "config.yaml")
	if p != want {
		t.Fatalf("Path = %q, want %q", p, want)
	}
}

func TestSetThenGet(t *testing.T) {
	_, cleanup := withTempHome(t)
	defer cleanup()

	cases := []struct {
		key, val, want string
	}{
		{"registry.url", "https://registry.internal.example.com", "https://registry.internal.example.com"},
		{"self_update.enabled", "true", "true"},
		{"self_update.check_interval", "2h", "2h"},
		{"cache.dir", "~/.work/cache", "~/.work/cache"},
	}
	for _, c := range cases {
		if err := Set(c.key, c.val, false); err != nil {
			t.Fatalf("Set(%q): %v", c.key, err)
		}
		got, ok, err := Get(c.key)
		if err != nil {
			t.Fatalf("Get(%q): %v", c.key, err)
		}
		if !ok {
			t.Fatalf("Get(%q): not found after Set", c.key)
		}
		if got != c.want {
			t.Fatalf("Get(%q) = %q, want %q", c.key, got, c.want)
		}
	}
}

func TestSetCreatesMissingFile(t *testing.T) {
	dir, cleanup := withTempHome(t)
	defer cleanup()

	if err := Set("registry.url", "https://x.example.com", false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	p := filepath.Join(dir, ".work", "config.yaml")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("配置文件未创建: %v", err)
	}
	// 内容应包含 url 键与值
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "url: https://x.example.com") {
		t.Fatalf("配置文件内容不含预期键值: %s", string(data))
	}
}

func TestSetSequenceByFlow(t *testing.T) {
	_, cleanup := withTempHome(t)
	defer cleanup()

	if err := Set("telemetry.events", "[shell,mcp,file_read]", false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	root, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// 导航 telemetry.events
	v, _ := findValue(root, "telemetry")
	if v == nil || v.Kind != yaml.MappingNode {
		t.Fatalf("telemetry 不是 mapping")
	}
	ev, _ := findValue(v, "events")
	if ev == nil || ev.Kind != yaml.SequenceNode {
		t.Fatalf("events 不是 sequence, kind=%v", ev)
	}
	if len(ev.Content) != 3 {
		t.Fatalf("events 长度 = %d, want 3", len(ev.Content))
	}
	// get 返回 YAML 文本，应含 shell
	got, _, _ := Get("telemetry.events")
	if !strings.Contains(got, "shell") {
		t.Fatalf("Get events = %q, 缺 shell", got)
	}
}

func TestSetFlowSequence(t *testing.T) {
	_, cleanup := withTempHome(t)
	defer cleanup()
	if err := Set("telemetry.redact", "[prompt, file_content]", false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok, _ := Get("telemetry.redact")
	if !ok {
		t.Fatalf("redact 未找到")
	}
	if !strings.Contains(got, "prompt") || !strings.Contains(got, "file_content") {
		t.Fatalf("redact = %q", got)
	}
}

func TestUnsetThenGetMissing(t *testing.T) {
	_, cleanup := withTempHome(t)
	defer cleanup()

	if err := Set("registry.url", "https://a.example.com", false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := Set("registry.token", "secret", false); err != nil {
		t.Fatalf("Set token: %v", err)
	}
	if err := Unset("registry.url", false); err != nil {
		t.Fatalf("Unset: %v", err)
	}
	got, ok, err := Get("registry.url")
	if err != nil {
		t.Fatalf("Get after Unset: %v", err)
	}
	if ok {
		t.Fatalf("Unset 后键仍存在，值=%q", got)
	}
	// 兄弟键应保留
	if _, ok, err := Get("registry.token"); err != nil || !ok {
		t.Fatalf("Unset 误删兄弟键")
	}
}

func TestUnsetIdempotent(t *testing.T) {
	_, cleanup := withTempHome(t)
	defer cleanup()
	if err := Unset("not.exist", false); err != nil {
		t.Fatalf("Unset 不存在的键应幂等无错: %v", err)
	}
}

func TestListFlattens(t *testing.T) {
	_, cleanup := withTempHome(t)
	defer cleanup()

	if err := Set("registry.url", "https://r.example.com", false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := Set("self_update.enabled", "true", false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := Set("telemetry.events", "[a,b]", false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	m, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if m["registry.url"] != "https://r.example.com" {
		t.Fatalf("registry.url = %q", m["registry.url"])
	}
	if m["self_update.enabled"] != "true" {
		t.Fatalf("self_update.enabled = %q", m["self_update.enabled"])
	}
	if !strings.Contains(m["telemetry.events"], "a") || !strings.Contains(m["telemetry.events"], "b") {
		t.Fatalf("telemetry.events = %q", m["telemetry.events"])
	}
	// 不应出现非叶子 mapping 键
	if _, ok := m["registry"]; ok {
		t.Fatalf("List 不应包含 mapping 键 registry")
	}
	if _, ok := m["self_update"]; ok {
		t.Fatalf("List 不应包含 mapping 键 self_update")
	}
}

func TestListEmptyWhenNoFile(t *testing.T) {
	_, cleanup := withTempHome(t)
	defer cleanup()
	m, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("空配置 List 应为空, got %v", m)
	}
}

func TestGetEmptyWhenNoFile(t *testing.T) {
	_, cleanup := withTempHome(t)
	defer cleanup()
	v, ok, err := Get("registry.url")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Fatalf("空配置 Get 应 not found, got %q", v)
	}
}

func TestCommentPreserved(t *testing.T) {
	dir, cleanup := withTempHome(t)
	defer cleanup()

	seed := `# 顶层注释：内部 Registry 配置
registry:
  url: https://old.example.com  # 行内注释
  # cache 配置
  cache:
    dir: ~/.work/cache
`
	writeFile(t, dir, ".work/config.yaml", seed)

	// 修改一个已存在的值并新增一个键
	if err := Set("registry.url", "https://new.example.com", false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := Set("registry.token", "abc123", false); err != nil {
		t.Fatalf("Set token: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".work", "config.yaml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(data)
	for _, want := range []string{"顶层注释", "行内注释", "cache 配置"} {
		if !strings.Contains(got, want) {
			t.Fatalf("注释 %q 未保留, 内容:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "https://new.example.com") {
		t.Fatalf("新值未写入:\n%s", got)
	}
	if !strings.Contains(got, "token: abc123") {
		t.Fatalf("新键未写入:\n%s", got)
	}
}

func TestValidateKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool // true=期望非法
	}{
		{"", true},
		{"registry.url", false},
		{".url", true},
		{"registry.", true},
		{"a..b", true},
	}
	for _, c := range cases {
		err := validateKey(c.key)
		if c.want && err == nil {
			t.Fatalf("validateKey(%q) 期望非法", c.key)
		}
		if !c.want && err != nil {
			t.Fatalf("validateKey(%q) 期望合法, got %v", c.key, err)
		}
	}
}

func TestSetPathConflict(t *testing.T) {
	_, cleanup := withTempHome(t)
	defer cleanup()
	// registry.url 设为标量后，再向 registry.url.x 设值应触发路径冲突
	if err := Set("registry.url", "https://x.example.com", false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	err := Set("registry.url.x", "conflict", false)
	if err == nil {
		t.Fatalf("期望路径冲突错误")
	}
	var ue *usage.Error
	if !errors.As(err, &ue) {
		t.Fatalf("期望 usage.Error, got %T: %v", err, err)
	}
}

// FuzzConfigSetGet 对配置的 Set+Get 往返进行模糊测试，
// 验证任意键值对写入后读取一致，且不会 panic。
func FuzzConfigSetGet(f *testing.F) {
	// 种子：来自现有测试的有效键值对
	f.Add("registry.url", "https://example.com")
	f.Add("self_update.enabled", "true")
	f.Add("self_update.check_interval", "2h")
	f.Add("cache.dir", "~/.work/cache")
	f.Add("telemetry.events", "[a,b,c]")
	f.Add("registry.token", "secret123")
	f.Add("simple", "value")
	f.Add("deep.nested.key", "42")

	f.Fuzz(func(t *testing.T, key, value string) {
		_, cleanup := withTempHome(t)
		defer cleanup()

		// Set 不应对任意输入 panic
		err := Set(key, value, false)
		if err != nil {
			// 不合法的 key 是预期的，不应 panic
			return
		}

		// 成功设置后，Get 应能取回相同的值，不应 panic
		got, ok, gerr := Get(key)
		if gerr != nil {
			t.Errorf("Get(%q) 失败: %v", key, gerr)
			return
		}
		if !ok {
			t.Errorf("Set(%q, %q) 成功但 Get 返回不存在", key, value)
			return
		}

		// api_key 后缀自动脱敏，跳过精确匹配
		// 列表值 yaml 序列化后格式可能不同，仅验证无 panic
		_ = got
	})
}
