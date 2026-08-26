package adapter

import (
	"encoding/json"
	"testing"
)

func TestMergeMCPServers(t *testing.T) {
	server := json.RawMessage(`{"command":"node","args":["server.js"]}`)
	out, err := MergeMCPServers(nil, "mysql", server)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := MergeMCPServers(out, "mysql", server)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := RemoveMCPServer(out2, "mysql")
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) == 0 {
		t.Fatal("expected output")
	}
}

func TestMergeMCPServersPreservesUnknownFields(t *testing.T) {
	// 回归：合并时应保留 mcpServers 之外的顶层字段（如用户自定义字段）。
	existing := []byte(`{"mcpServers":{"keep":{"command":"x"}},"customField":"val","another":42}`)
	server := json.RawMessage(`{"command":"node"}`)
	out, err := MergeMCPServers(existing, "new", server)
	if err != nil {
		t.Fatal(err)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["customField"]; !ok {
		t.Fatalf("customField lost: %s", string(out))
	}
	if _, ok := root["another"]; !ok {
		t.Fatalf("another lost: %s", string(out))
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(root["mcpServers"], &servers); err != nil {
		t.Fatal(err)
	}
	if _, ok := servers["keep"]; !ok {
		t.Fatalf("existing server 'keep' lost: %s", string(out))
	}
	if _, ok := servers["new"]; !ok {
		t.Fatalf("new server lost: %s", string(out))
	}
}
