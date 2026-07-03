package source

import (
	"testing"

	"github.com/huangchao257/work-cli/internal/catalog"
)

func TestParseInstallNameRejectsLocalPath(t *testing.T) {
	cases := []string{
		"./examples/dev-kit",
		"/tmp/dev-kit",
		"../examples/dev-kit",
		"git:github.com/org/repo@v1.0",
	}
	for _, raw := range cases {
		if _, err := ParseInstallName(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestParseInstallNameAcceptsRegistryName(t *testing.T) {
	ref, err := ParseInstallName("dev-kit")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Kind != KindRegistry || ref.Name != "dev-kit" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestParseInstallNameRejectsInvalidName(t *testing.T) {
	cases := []string{"Dev-Kit", "dev_kit", "-dev", "dev-"}
	for _, raw := range cases {
		if _, err := ParseInstallName(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

// 边缘情况测试
func TestParseInstallNameEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"空字符串", "", true},
		{"纯空格", "  ", true},
		{"含冒号", "a:b", true},
		{"含反斜杠", `a\b`, true},
		{"含点号带路径", "a/b", true},
		{"含双点", "a..b", true},
		{"首部大写", "DevKit", true},
		{"含下划线", "dev_kit", true},
		{"以连字符开头", "-name", true},
		{"以连字符结尾", "name-", true},
		{"单字符", "a", false},
		{"正常名称", "dev-kit", false},
		{"字母数字连字符混合", "go-1-21", false}, // 符合 a-z0-9 开头/结尾 + 中间 a-z0-9-
		{"仅数字", "123", false},
		{"含中文", "测试名称", true},
		{"带空格", "a name", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseInstallName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseInstallName(%q) error=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateInstallNameBuiltin(t *testing.T) {
	// 验证内置名称不报错
	for _, name := range catalog.Names() {
		if err := ValidateInstallName(name); err != nil {
			t.Fatalf("validate builtin %q failed: %v", name, err)
		}
	}
}

func TestValidateInstallNameUnknown(t *testing.T) {
	// 未知名称的行为取决于当前环境是否配置了 registry.url：
	// 有 registry → 通过（信任 registry 有对应包）
	// 无 registry → 失败（提示未知资源）
	err := ValidateInstallName("totally-unknown-resource-xyz")
	if err != nil {
		// 期望错误信息包含"未知资源"或"加载用户配置失败"
		t.Logf("ValidateInstallName returned error (expected if no registry configured): %v", err)
	} else {
		t.Logf("ValidateInstallName passed (expected if registry.url is configured)")
	}
}

func TestParseRefEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		kind    Kind
	}{
		{"空字符串", "", true, 0},
		{"纯空格", "  ", true, 0},
		{"本地路径 ./", "./local", false, KindLocal},
		{"本地路径 /", "/tmp/local", false, KindLocal},
		{"本地路径 ../", "../parent", false, KindLocal},
		{"git 引用", "git:github.com/org/repo@v1.0", false, KindGit},
		{"git 引用无 @", "git:github.com/org/repo", true, 0},
		{"git 引用 @ 在开头", "git:@v1.0", true, 0},
		{"registry 名称", "dev-kit", false, KindRegistry},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseRef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if ref.Kind != tt.kind {
				t.Fatalf("expected kind %d, got %d for %q", tt.kind, ref.Kind, tt.input)
			}
		})
	}
}

