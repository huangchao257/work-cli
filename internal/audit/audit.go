// Package audit 提供 hooks 事件的本地审计引擎。
//
// 对本地 queue.jsonl 中的 hooks 事件按策略规则匹配，输出违规清单。
// 纯离线旁路分析，不阻断 IDE。
package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/huangchao257/work-cli/internal/hooks"
	"gopkg.in/yaml.v3"
)

// Severity 表示违规严重等级。
type Severity string

const (
	Low    Severity = "low"
	Medium Severity = "medium"
	High   Severity = "high"
)

// Rule 是一条审计规则。
type Rule struct {
	ID        string   `yaml:"id"`
	Event     string   `yaml:"event"`
	Match     string   `yaml:"match"`
	PathRegex string   `yaml:"path_regex"`
	Severity  Severity `yaml:"severity"`
}

// Policy 是审计策略文件结构。
type Policy struct {
	Rules []Rule `yaml:"rules"`
}

// CompiledPolicy 是预编译后的策略，缓存正则避免重复编译。
type CompiledPolicy struct {
	Rules []compiledRule
}

// Compile 预编译策略中所有规则的正则表达式。应先通过 LoadPolicy 加载并校验。
func (p Policy) Compile() CompiledPolicy {
	compiled := make([]compiledRule, 0, len(p.Rules))
	for _, r := range p.Rules {
		cr := compiledRule{rule: r}
		if r.Match != "" {
			cr.matchRe = regexp.MustCompile(r.Match)
		}
		if r.PathRegex != "" {
			cr.pathRe = regexp.MustCompile(r.PathRegex)
		}
		if cr.rule.Severity == "" {
			cr.rule.Severity = Medium
		}
		compiled = append(compiled, cr)
	}
	return CompiledPolicy{Rules: compiled}
}

// Violation 是一条违规记录。
type Violation struct {
	RuleID    string   `json:"rule_id"`
	EventID   string   `json:"event_id"`
	Severity  Severity `json:"severity"`
	Detail    string   `json:"detail"`
	Timestamp string   `json:"timestamp"`
}

// EventRecord 是 hooks.EventRecord 的兼容别名，便于审计解码与测试。
// 直接引用 hooks 包已导出的结构体，不重复定义、不修改 hooks 包。
type EventRecord = hooks.EventRecord

// queueLine 对应 queue.jsonl 每行结构：{"event":{...EventRecord...},"uploaded_at":...}
type queueLine struct {
	Event      EventRecord `json:"event"`
	UploadedAt *time.Time  `json:"uploaded_at"`
}

// LoadPolicy 从路径读取并校验审计策略文件。
// 校验：每条规则 id 必填；match 与 path_regex 至少一个；正则可编译；
// severity 空值默认 medium。
func LoadPolicy(path string) (Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, err
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Policy{}, fmt.Errorf("解析审计策略失败: %w", err)
	}
	for i, r := range p.Rules {
		if r.ID == "" {
			return Policy{}, fmt.Errorf("规则 #%d 缺少 id", i+1)
		}
		if r.Match == "" && r.PathRegex == "" {
			return Policy{}, fmt.Errorf("规则 %s 缺少 match 或 path_regex", r.ID)
		}
		if r.Match != "" {
			if _, err := regexp.Compile(r.Match); err != nil {
				return Policy{}, fmt.Errorf("规则 %s match 正则非法: %w", r.ID, err)
			}
		}
		if r.PathRegex != "" {
			if _, err := regexp.Compile(r.PathRegex); err != nil {
				return Policy{}, fmt.Errorf("规则 %s path_regex 正则非法: %w", r.ID, err)
			}
		}
		if r.Severity == "" {
			p.Rules[i].Severity = Medium
		}
	}
	return p, nil
}

// ReadEvents 读取 queue.jsonl，逐行解码为 EventRecord。
// now 与 since 用于时间过滤：since > 0 时仅保留 now-since 之后的事件。
// 解码失败的行跳过并计入 warnings。文件不存在返回 (nil,nil,nil)。
func ReadEvents(path string, now time.Time, since time.Duration) ([]EventRecord, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	defer f.Close()

	var events []EventRecord
	var warnings []string
	sc := bufio.NewScanner(f)
	// 提高单行缓冲上限，避免长 payload 被截断
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ql queueLine
		if err := json.Unmarshal(line, &ql); err != nil {
			warnings = append(warnings, fmt.Sprintf("跳过无法解码的行: %v", err))
			continue
		}
		if since > 0 {
			ts := ql.Event.Timestamp
			if ts.IsZero() || ts.Before(now.Add(-since)) {
				continue
			}
		}
		events = append(events, ql.Event)
	}
	if err := sc.Err(); err != nil {
		return events, warnings, err
	}
	return events, warnings, nil
}

// Evaluate 是纯函数：对事件按策略匹配，返回违规清单。
// 时间过滤应在调用前由 ReadEvents 完成，Evaluate 不做额外过滤。
func Evaluate(events []EventRecord, cp CompiledPolicy) []Violation {
	var violations []Violation
	for _, ev := range events {
		ptext := payloadText(ev.Payload)
		paths := extractPaths(ev.Payload)
		for _, cr := range cp.Rules {
			// event 过滤：规则限定事件名时不匹配则跳过
			if cr.rule.Event != "" && cr.rule.Event != ev.AbstractEvent {
				continue
			}
			hit, detail := cr.match(ptext, paths)
			if hit {
				violations = append(violations, Violation{
					RuleID:    cr.rule.ID,
					EventID:   ev.EventID,
					Severity:  cr.rule.Severity,
					Detail:    detail,
					Timestamp: ev.Timestamp.Format(time.RFC3339),
				})
			}
		}
	}
	return violations
}

// compiledRule 是预编译后的规则，缓存正则以加速匹配。
type compiledRule struct {
	rule    Rule
	matchRe *regexp.Regexp
	pathRe  *regexp.Regexp
}

// match 应用 match 与 path_regex，任一命中即违规。
func (cr compiledRule) match(ptext string, paths []string) (bool, string) {
	if cr.matchRe != nil && ptext != "" {
		if m := cr.matchRe.FindString(ptext); m != "" {
			return true, fmt.Sprintf("match 命中: %s", m)
		}
	}
	if cr.pathRe != nil {
		for _, pf := range paths {
			if cr.pathRe.MatchString(pf) {
				return true, fmt.Sprintf("path_regex 命中: %s", pf)
			}
		}
		// 无显式路径字段时回退到整段文本匹配
		if len(paths) == 0 && ptext != "" {
			if m := cr.pathRe.FindString(ptext); m != "" {
				return true, fmt.Sprintf("path_regex 命中: %s", m)
			}
		}
	}
	return false, ""
}

// payloadText 将 payload 序列化为文本，优先 JSON，失败回退 fmt.Sprintf。
func payloadText(payload map[string]any) string {
	if len(payload) == 0 {
		return ""
	}
	if b, err := json.Marshal(payload); err == nil {
		return string(b)
	}
	return fmt.Sprintf("%v", payload)
}

// extractPaths 从 payload 中提取常见路径字段值。
func extractPaths(payload map[string]any) []string {
	var paths []string
	for _, k := range []string{"path", "file", "file_path", "filepath", "filePath"} {
		if v, ok := payload[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				paths = append(paths, s)
			}
		}
	}
	return paths
}
