package adapter

import (
	"encoding/json"
	"fmt"
)

// mcpFile 是 MCP 配置文件的顶层结构，仅供 ExtractMCPServer 解析 mcpServers 使用。
type mcpFile struct {
	MCPServers map[string]json.RawMessage `json:"mcpServers"`
}

// mergeMCPServers 在保留未知顶层字段的前提下合并 mcpServers。
// 直接 Unmarshal 到 mcpFile 会丢弃 mcpServers 之外的顶层字段；这里解析到
// map[string]json.RawMessage，仅替换 mcpServers 键，其余键原样保留再整体序列化
// （json.MarshalIndent 对 map 键按字典序排序，输出确定性）。
func mergeMCPServers(existing []byte, mutate func(map[string]json.RawMessage) error) ([]byte, error) {
	root := map[string]json.RawMessage{}
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &root); err != nil {
			return nil, fmt.Errorf("解析 MCP 配置失败: %w", err)
		}
	}

	var servers map[string]json.RawMessage
	if raw, ok := root["mcpServers"]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return nil, fmt.Errorf("解析 mcpServers 失败: %w", err)
		}
	}
	if servers == nil {
		servers = map[string]json.RawMessage{}
	}
	if err := mutate(servers); err != nil {
		return nil, err
	}

	out, err := json.Marshal(servers)
	if err != nil {
		return nil, err
	}
	root["mcpServers"] = out
	return json.MarshalIndent(root, "", "  ")
}

func MergeMCPServers(existing []byte, serverID string, serverJSON json.RawMessage) ([]byte, error) {
	return mergeMCPServers(existing, func(servers map[string]json.RawMessage) error {
		servers[serverID] = serverJSON
		return nil
	})
}

func RemoveMCPServer(existing []byte, serverID string) ([]byte, error) {
	if len(existing) == 0 {
		return existing, nil
	}
	return mergeMCPServers(existing, func(servers map[string]json.RawMessage) error {
		delete(servers, serverID)
		return nil
	})
}

func ExtractMCPServer(existing []byte, serverID string) (json.RawMessage, error) {
	cfg := mcpFile{}
	if len(existing) == 0 {
		return nil, fmt.Errorf("server %s not found", serverID)
	}
	if err := json.Unmarshal(existing, &cfg); err != nil {
		return nil, fmt.Errorf("解析 MCP 配置失败: %w", err)
	}
	raw, ok := cfg.MCPServers[serverID]
	if !ok {
		return nil, fmt.Errorf("server %s not found", serverID)
	}
	return raw, nil
}
