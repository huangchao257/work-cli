package hooks

import (
	"encoding/json"
	"strings"
	"sync"
)

const maxStringLen = 512

// mapPool 用于 redactPath 中 map[interface{}]interface{} 转 map[string]any 时复用。
// 大容量 map 不归还池以避免内存泄漏。
var mapPool = sync.Pool{
	New: func() any { return make(map[string]any, 8) },
}

func RedactPayload(raw []byte, fields []string) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		// 非JSON：存储截断后的字符串
		s := string(raw)
		if len(s) > maxStringLen {
			s = s[:maxStringLen] + "…[truncated]"
		}
		return map[string]any{"_raw": s}, nil
	}
	redactMap(data, fields)
	// 迭代式截断字符串，代替递归以降低深层嵌套时的分配
	truncateStringsIter(data, maxStringLen)
	return data, nil
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
		// YAML解析可能产生 map[interface{}]interface{}，转为 map[string]any
		m := mapPool.Get().(map[string]any)
		// 清空复用 map
		for k := range m {
			delete(m, k)
		}
		for k, val := range v {
			if ks, ok := k.(string); ok {
				m[ks] = val
			}
		}
		redactPath(m, parts[1:])
		cur[key] = m
		// 小 map 归还池，大 map 不归还避免内存占留
		if len(m) <= 64 {
			mapPool.Put(m)
		}
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
				top.m[k] = t[:max] + "…[truncated]"
			}
		case map[string]any:
			stack = append(stack, frame{m: t})
		}
	}
}
