package manifest

import "testing"

func TestValidateID(t *testing.T) {
	valid := []string{"a", "dev-kit", "code-review", "my.skill", "under_score", "v1.0", "a1b2c3", "trailing-"}
	for _, s := range valid {
		if err := ValidateID(s); err != nil {
			t.Fatalf("expected %q valid, got %v", s, err)
		}
	}

	invalid := []string{
		"",
		"../escape",
		"../../../../tmp/target",
		"a/b",
		`a\b`,
		"a b",
		"a\"b",
		"$(curl evil)",
		"a`b",
		"-leading",
	}
	for _, s := range invalid {
		if err := ValidateID(s); err == nil {
			t.Fatalf("expected %q invalid, got nil", s)
		}
	}
}
