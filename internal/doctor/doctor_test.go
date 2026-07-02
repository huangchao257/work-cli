package doctor

import (
	"os"
	"testing"
)

// TestParseConfigYAML_Valid 验证合法 YAML 可正常解析。
func TestParseConfigYAML_Valid(t *testing.T) {
	data := []byte("self_update:\n  enabled: true\n  check_interval: 2h\n")
	if err := ParseConfigYAML(data); err != nil {
		t.Fatalf("合法 YAML 解析失败: %v", err)
	}
}

// TestParseConfigYAML_Invalid 验证非法 YAML 解析失败，
// 对应「JSON/YAML 解析失败标失败」分支。
func TestParseConfigYAML_Invalid(t *testing.T) {
	// 缩进/结构非法的 YAML：键后紧跟未闭合的引号与制表符混用。
	data := []byte("self_update:\n\tenabled: true\n")
	if err := ParseConfigYAML(data); err == nil {
		t.Fatal("非法 YAML 应解析失败，但返回 nil")
	}
}

// TestParseMCPJSON_Valid 验证合法 JSON 可正常解析。
func TestParseMCPJSON_Valid(t *testing.T) {
	data := []byte(`{"mcpServers": {"foo": {}}}`)
	if err := ParseMCPJSON(data); err != nil {
		t.Fatalf("合法 JSON 解析失败: %v", err)
	}
}

// TestParseMCPJSON_Invalid 验证非法 JSON 解析失败。
func TestParseMCPJSON_Invalid(t *testing.T) {
	data := []byte(`{"mcpServers": `)
	if err := ParseMCPJSON(data); err == nil {
		t.Fatal("非法 JSON 应解析失败，但返回 nil")
	}
}

// TestCheckMCPFile_NotExist 验证 MCP 配置文件不存在不算失败。
func TestCheckMCPFile_NotExist(t *testing.T) {
	cr := checkMCPFile("qoder", "/path/does/not/exist/mcp.json")
	if cr.Severity != SeverityError {
		t.Fatalf("期望 severity=error，实际 %s", cr.Severity)
	}
	if !cr.OK {
		t.Fatalf("文件不存在应标 OK，detail=%s", cr.Detail)
	}
}

// TestCheckMCPFile_Invalid 验证 MCP 配置 JSON 非法时标失败。
func TestCheckMCPFile_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/mcp.json"
	if err := writeBytes(path, []byte(`{bad json`)); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}
	cr := checkMCPFile("qoder", path)
	if cr.OK {
		t.Fatalf("非法 JSON 应标失败，detail=%s", cr.Detail)
	}
}

// TestCheckMCPFile_Valid 验证合法 MCP 配置标通过。
func TestCheckMCPFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/mcp.json"
	if err := writeBytes(path, []byte(`{"mcpServers": {}}`)); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}
	cr := checkMCPFile("cursor", path)
	if !cr.OK {
		t.Fatalf("合法 MCP 配置应标通过，detail=%s", cr.Detail)
	}
}

// TestHasError 验证 HasError 只在 error 项未通过时返回 true。
func TestHasError(t *testing.T) {
	allOK := []CheckResult{
		{Name: "a", OK: true, Severity: SeverityError},
		{Name: "b", OK: true, Severity: SeverityInfo},
	}
	if HasError(allOK) {
		t.Fatal("全部通过时不应有 error")
	}
	withErr := []CheckResult{
		{Name: "a", OK: true, Severity: SeverityError},
		{Name: "b", OK: false, Severity: SeverityError},
		{Name: "c", OK: false, Severity: SeverityInfo},
	}
	if !HasError(withErr) {
		t.Fatal("存在 error 项未通过时应返回 true")
	}
}

// TestSummary 验证通过/失败统计（info 项未通过不计失败）。
func TestSummary(t *testing.T) {
	results := []CheckResult{
		{Name: "a", OK: true, Severity: SeverityError},
		{Name: "b", OK: false, Severity: SeverityError},
		{Name: "c", OK: false, Severity: SeverityInfo},
	}
	passed, failed := Summary(results)
	if passed != 2 || failed != 1 {
		t.Fatalf("期望 passed=2 failed=1，实际 passed=%d failed=%d", passed, failed)
	}
}

// TestCheckWorkInPath 至少保证函数可执行并返回结果（CI 环境可能无 work）。
func TestCheckWorkInPath(t *testing.T) {
	cr := checkWorkInPath()
	if cr.Name == "" {
		t.Fatal("Name 不应为空")
	}
}

// TestRunSmoke 烟测 Run 不 panic 且返回非空结果集。
func TestRunSmoke(t *testing.T) {
	results := Run(Options{Scope: "user", IDEs: nil})
	if len(results) == 0 {
		t.Fatal("Run 应返回非空检查结果集")
	}
	// 至少应包含一个 error 严重度条目（work 在 PATH）。
	var hasErrorSeverity bool
	for _, r := range results {
		if r.Severity == SeverityError {
			hasErrorSeverity = true
			break
		}
	}
	if !hasErrorSeverity {
		t.Fatal("结果集应至少包含一个 severity=error 的条目")
	}
}

// writeBytes 是测试辅助函数，向临时路径写入字节。
func writeBytes(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
