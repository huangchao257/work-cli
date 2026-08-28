package api

import (
	"fmt"
	"strings"

	"github.com/huangchao257/work-cli/internal/openapi"
	"github.com/huangchao257/work-cli/internal/usage"
)

// RiskLevel 是操作的风险等级。冲突时取更保守（更大）值。
type RiskLevel int

const (
	RiskRead RiskLevel = iota
	RiskWrite
	RiskDangerous
)

func (r RiskLevel) String() string {
	switch r {
	case RiskRead:
		return "read"
	case RiskWrite:
		return "write"
	default:
		return "dangerous"
	}
}

// ParseRiskLevel 解析风险等级，非法值返回错误。
func ParseRiskLevel(raw string) (RiskLevel, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "read":
		return RiskRead, nil
	case "write":
		return RiskWrite, nil
	case "dangerous":
		return RiskDangerous, nil
	default:
		return 0, usage.Newf("非法风险等级 %q（支持 read/write/dangerous）", raw)
	}
}

// AssessRisk 计算 operation 的有效风险。
// 与导入期 normalizeRisk（openapi/catalog.go）保持同语义：
// - 空 risk → 按 method 推断（GET/HEAD/OPTIONS→read，写方法→write，其余→dangerous）；
// - 非法值 → 最保守的 dangerous（fail-closed）。
// 手编/损坏的 catalog.json 可能出现这两种情况，不能默认按 read 放行
// （否则 DELETE 缺 risk 字段会无需确认直接发出）。
func AssessRisk(op *openapi.CatalogOperation) RiskLevel {
	if op == nil {
		return RiskDangerous
	}
	trimmed := strings.ToLower(strings.TrimSpace(op.Risk))
	switch trimmed {
	case "read", "write", "dangerous":
		level, _ := ParseRiskLevel(trimmed)
		return level
	case "":
		return inferMethodRisk(op.Method)
	default:
		return RiskDangerous
	}
}

func inferMethodRisk(method string) RiskLevel {
	switch strings.ToUpper(method) {
	case "GET", "HEAD", "OPTIONS":
		return RiskRead
	case "POST", "PUT", "PATCH", "DELETE":
		return RiskWrite
	default:
		return RiskDangerous
	}
}

// Confirmable 判断风险等级是否需要确认。
func Confirmable(level RiskLevel) bool { return level >= RiskWrite }

// ConfirmationRequiredError 是 fail-closed 确认门禁的结构化错误。
type ConfirmationRequiredError struct {
	System    string `json:"system"`
	Operation string `json:"operation"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Risk      string `json:"risk"`
	Reason    string `json:"reason"`
	Hint      string `json:"hint"`
}

func (e *ConfirmationRequiredError) Error() string {
	return fmt.Sprintf("操作 %s %s %s（风险 %s）需要确认：%s", e.Method, e.Path, e.Operation, e.Risk, e.Reason)
}

// NewConfirmationRequired 构造确认门禁错误。
func NewConfirmationRequired(system, operation, method, path string, risk RiskLevel, reason string) *ConfirmationRequiredError {
	return &ConfirmationRequiredError{
		System: system, Operation: operation, Method: method, Path: path,
		Risk: risk.String(), Reason: reason,
		Hint: "确认无误后追加 --yes 重新执行",
	}
}
