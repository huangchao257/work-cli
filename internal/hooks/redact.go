package hooks

import (
	"encoding/json"
	"strings"
)

const maxStringLen = 512

func RedactPayload(raw []byte, fields []string) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		// non-JSON: store truncated string
		s := string(raw)
		if len(s) > maxStringLen {
			s = s[:maxStringLen] + "…[truncated]"
		}
		return map[string]any{"_raw": s}, nil
	}
	redactMap(data, fields)
	truncateStrings(data, maxStringLen)
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
		m := map[string]any{}
		for k, val := range v {
			if ks, ok := k.(string); ok {
				m[ks] = val
			}
		}
		redactPath(m, parts[1:])
		cur[key] = m
	}
}

func truncateStrings(data map[string]any, max int) {
	for k, v := range data {
		switch t := v.(type) {
		case string:
			if len(t) > max {
				data[k] = t[:max] + "…[truncated]"
			}
		case map[string]any:
			truncateStrings(t, max)
		}
	}
}
