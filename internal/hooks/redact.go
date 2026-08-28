package hooks

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

const maxStringLen = 512

func RedactPayload(raw []byte, fields []string) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		// 非JSON：存储截断后的字符串
		s := truncateUTF8(string(raw), maxStringLen)
		return map[string]any{"_raw": s}, nil
	}
	redactMap(data, fields)
	// 迭代式截断字符串，代替递归以降低深层嵌套时的分配
	truncateStringsIter(data, maxStringLen)
	return data, nil
}

// truncateUTF8 按 rune 截断到 max 字节以内，不切断多字节 UTF-8 序列
// （按字节硬切会产生无效序列，入库/上报后变为 U+FFFD 乱码）。
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// 回退到完整 rune 边界
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…[truncated]"
}

func redactMap(data map[string]any, fields []string) {
	for _, f := range fields {
		parts := strings.Split(f, ".")
		redactPath(data, parts)
	}
}

func redactPath(cur map[string]any, parts []string) {
	if len(parts) == 0 {
		return
	}
	key := parts[0]
	if len(parts) == 1 {
		if _, ok := cur[key]; ok {
			cur[key] = "[redacted]"
		}
		return
	}
	next, ok := cur[key]
	if !ok {
		return
	}
	switch v := next.(type) {
	case map[string]any:
		redactPath(v, parts[1:])
	case map[interface{}]interface{}:
		// YAML解析可能产生 map[interface{}]interface{}，转为 map[string]any。
		// 不使用 sync.Pool：池化的 map 被赋值给 cur[key] 后再 Put 回池，
		// 后续 Get 会复用同一 map 并清空它，导致调用方仍引用的结果被静默清空。
		m := make(map[string]any, len(v))
		for k, val := range v {
			if ks, ok := k.(string); ok {
				m[ks] = val
			}
		}
		redactPath(m, parts[1:])
		cur[key] = m
	}
}

// truncateStringsIter 使用显式栈迭代遍历嵌套 map，避免深层递归带来的栈帧分配。
func truncateStringsIter(root map[string]any, max int) {
	// 栈元素：当前 map 及其待遍历 key 的索引
	type frame struct {
		m    map[string]any
		keys []string
		i    int
	}
	stack := []frame{{m: root}}

	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.keys == nil {
			// 收集当前 map 的所有 key
			top.keys = make([]string, 0, len(top.m))
			for k := range top.m {
				top.keys = append(top.keys, k)
			}
		}
		if top.i >= len(top.keys) {
			// 当前 map 遍历完成
			stack = stack[:len(stack)-1]
			continue
		}
		k := top.keys[top.i]
		top.i++
		v := top.m[k]
		switch t := v.(type) {
		case string:
			if len(t) > max {
				top.m[k] = truncateUTF8(t, max)
			}
		case map[string]any:
			stack = append(stack, frame{m: t})
		}
	}
}
