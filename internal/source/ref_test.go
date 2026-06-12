package source

import "testing"

func TestParseInstallNameRejectsLocalPath(t *testing.T) {
	cases := []string{
		"./examples/dev-kit",
		"/tmp/dev-kit",
		"../examples/dev-kit",
		"git:github.com/org/repo@v1.0",
	}
	for _, raw := range cases {
		if _, err := ParseInstallName(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestParseInstallNameAcceptsRegistryName(t *testing.T) {
	ref, err := ParseInstallName("dev-kit")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Kind != KindRegistry || ref.Name != "dev-kit" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestParseInstallNameRejectsInvalidName(t *testing.T) {
	cases := []string{"Dev-Kit", "dev_kit", "-dev", "dev-"}
	for _, raw := range cases {
		if _, err := ParseInstallName(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}
