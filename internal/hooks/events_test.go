package hooks

import "testing"

func TestResolveEventsAuditPreset(t *testing.T) {
	m := &Manifest{Telemetry: TelemetrySpec{Preset: PresetAudit}}
	events, err := ResolveEvents(m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %v", events)
	}
}

func TestBindingsForIDEQoderSessionSkipped(t *testing.T) {
	_, warnings := BindingsForIDE("qoder", []string{EventSession})
	if len(warnings) != 1 {
		t.Fatalf("expected warning for qoder session, got %v", warnings)
	}
}

func TestBindingsForIDECursorShell(t *testing.T) {
	bindings, warnings := BindingsForIDE("cursor", []string{EventShell})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %v", bindings)
	}
}
