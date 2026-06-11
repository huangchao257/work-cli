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
