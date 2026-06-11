package adapter

import (
	"encoding/json"
	"fmt"
)

type mcpFile struct {
	MCPServers map[string]json.RawMessage `json:"mcpServers"`
}

func MergeMCPServers(existing []byte, serverID string, serverJSON json.RawMessage) ([]byte, error) {
	cfg := mcpFile{MCPServers: map[string]json.RawMessage{}}
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &cfg); err != nil {
			return nil, fmt.Errorf("解析 MCP 配置失败: %w", err)
		}
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = map[string]json.RawMessage{}
	}
	cfg.MCPServers[serverID] = serverJSON
	return json.MarshalIndent(cfg, "", "  ")
}

func RemoveMCPServer(existing []byte, serverID string) ([]byte, error) {
	if len(existing) == 0 {
		return existing, nil
	}
	cfg := mcpFile{}
	if err := json.Unmarshal(existing, &cfg); err != nil {
		return nil, fmt.Errorf("解析 MCP 配置失败: %w", err)
	}
	delete(cfg.MCPServers, serverID)
	if len(cfg.MCPServers) == 0 {
		return []byte("{\n  \"mcpServers\": {}\n}\n"), nil
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func ExtractMCPServer(existing []byte, serverID string) (json.RawMessage, error) {
	cfg := mcpFile{}
	if len(existing) == 0 {
		return nil, fmt.Errorf("server %s not found", serverID)
	}
	if err := json.Unmarshal(existing, &cfg); err != nil {
		return nil, err
	}
	raw, ok := cfg.MCPServers[serverID]
	if !ok {
		return nil, fmt.Errorf("server %s not found", serverID)
	}
	return raw, nil
}
