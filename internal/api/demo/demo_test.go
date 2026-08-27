package demo

import (
	"context"
	"testing"

	"github.com/huangchao257/work-cli/internal/api"
)

func TestDemoRegistersToDefaultRegistry(t *testing.T) {
	s, ok := api.DefaultRegistry.ByName("demo")
	if !ok {
		t.Fatal("demo should be registered in DefaultRegistry")
	}
	if s.Manifest().Source != "builtin" {
		t.Fatalf("source = %q", s.Manifest().Source)
	}
	if s.BaseURL() == "" {
		t.Fatal("demo BaseURL should be non-empty (mock transport)")
	}
}

func TestDemoCatalogOperations(t *testing.T) {
	s := New()
	catalog, err := s.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{
		"getStatus": false, "listPets": false, "createPet": false,
		"getPet": false, "deletePet": false,
	}
	for _, op := range catalog.Operations {
		if _, ok := expected[op.ID]; !ok {
			t.Fatalf("unexpected operation %q", op.ID)
		}
		expected[op.ID] = true
	}
	for id, found := range expected {
		if !found {
			t.Fatalf("missing operation %q", id)
		}
	}
	// deletePet 显式 dangerous
	if op, ok := catalog.FindByID("deletePet"); !ok || op.Risk != "dangerous" {
		t.Fatalf("deletePet risk should be dangerous: %#v", op)
	}
}

func TestDemoShortcuts(t *testing.T) {
	s := New()
	shortcuts := s.Shortcuts()
	if len(shortcuts) != 2 {
		t.Fatalf("shortcuts = %d", len(shortcuts))
	}
	var seed, top bool
	for _, sc := range shortcuts {
		switch sc.Name {
		case "+seed":
			seed = sc.Handler != nil
		case "+top":
			top = sc.Target == "listPets"
		}
	}
	if !seed || !top {
		t.Fatalf("unexpected shortcuts: %#v", shortcuts)
	}
}

func TestDemoCallReadOperation(t *testing.T) {
	s := New()
	result, err := api.Call(context.Background(), s, api.CallOptions{
		System: "demo", Operation: "listPets", Params: map[string]string{"query.limit": "2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Status != 200 {
		t.Fatalf("result = %#v", result)
	}
}

func TestDemoCallWriteRequiresYes(t *testing.T) {
	s := New()
	_, err := api.Call(context.Background(), s, api.CallOptions{
		System: "demo", Operation: "createPet", Body: `{"name":"rex"}`,
	})
	if _, ok := err.(*api.ConfirmationRequiredError); !ok {
		t.Fatalf("expected ConfirmationRequiredError, got %v", err)
	}
	result, err := api.Call(context.Background(), s, api.CallOptions{
		System: "demo", Operation: "createPet", Body: `{"name":"rex"}`, Yes: true,
	})
	if err != nil || !result.OK || result.Status != 201 {
		t.Fatalf("createPet --yes: err=%v result=%#v", err, result)
	}
}

func TestDemoCall404(t *testing.T) {
	s := New()
	result, err := api.Call(context.Background(), s, api.CallOptions{
		System: "demo", Operation: "getPet", Params: map[string]string{"path.id": "404"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || result.Status != 404 || result.Error == nil {
		t.Fatalf("result = %#v", result)
	}
}

func TestDemoSeedShortcutHandler(t *testing.T) {
	s := New()
	shortcuts, err := api.BuildShortcuts(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := api.FindShortcut(shortcuts, "seed")
	if !ok {
		t.Fatal("+seed shortcut should exist")
	}
	result, err := api.ExecuteShortcut(context.Background(), s, sc, api.CallOptions{
		System: "demo", Yes: true, DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", result.Data)
	}
	if data["created"] == nil || data["pets"] == nil {
		t.Fatalf("seed should return created+pets: %#v", data)
	}
}
