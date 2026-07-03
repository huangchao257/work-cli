package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/huangchao257/work-cli/internal/usage"
)

func TestExitCodeWithExitError(t *testing.T) {
	ue := usage.New("参数错误")
	ee := exitErr(2, ue)
	if code := ExitCode(ee); code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestExitCodeWithWrappedExitError(t *testing.T) {
	ue := usage.New("参数错误")
	ee := exitErr(2, ue)
	wrapped := fmt.Errorf("额外信息: %w", ee)
	if code := ExitCode(wrapped); code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestExitCodeWithPlainError(t *testing.T) {
	err := errors.New("普通错误")
	if code := ExitCode(err); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestExitCodeWithNil(t *testing.T) {
	if code := ExitCode(nil); code != 1 {
		t.Fatalf("expected exit code 1 for nil, got %d", code)
	}
}

func TestExitCodeWithCustomCodes(t *testing.T) {
	for _, c := range []int{0, 2, 3, 77, 127} {
		ee := exitErr(c, errors.New("test"))
		if code := ExitCode(ee); code != c {
			t.Fatalf("expected exit code %d, got %d", c, code)
		}
	}
}

func TestExitErrorError(t *testing.T) {
	inner := errors.New("内部错误")
	ee := exitErr(2, inner)
	// exitError.Error() 返回内层错误的消息
	if ee.Error() != "内部错误" {
		t.Fatalf("expected '内部错误', got %q", ee.Error())
	}
}

func TestExitErrorUnwrap(t *testing.T) {
	inner := errors.New("内部错误")
	ee := exitErr(2, inner)
	if !errors.Is(ee, inner) {
		t.Fatal("expected errors.Is to match the inner error")
	}
	unwrapped := errors.Unwrap(ee)
	if unwrapped != inner {
		t.Fatal("Unwrap should return the inner error")
	}
}

func TestIsUsageError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		expect  bool
	}{
		{"usage.New", usage.New("参数错误"), true},
		{"usage.Wrap", usage.Wrap(errors.New("inner"), "用法错误"), true},
		{"usage.Newf", usage.Newf("参数 %d 无效", 42), true},
		{"fmt.Errorf wrapped usage", fmt.Errorf("wrapped: %w", usage.New("x")), true},
		{"plain error", errors.New("普通错误"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUsageError(tt.err); got != tt.expect {
				t.Fatalf("IsUsageError(%v) = %v, want %v", tt.err, got, tt.expect)
			}
		})
	}
}

func TestExitUsageErr(t *testing.T) {
	t.Run("usage error maps to exit code 2", func(t *testing.T) {
		ue := usage.New("参数错误")
		out := ExitUsageErr(ue)
		if code := ExitCode(out); code != 2 {
			t.Fatalf("expected exit code 2, got %d", code)
		}
	})

	t.Run("wrapped usage error maps to exit code 2", func(t *testing.T) {
		ue := usage.New("参数错误")
		wrapped := fmt.Errorf("context: %w", ue)
		out := ExitUsageErr(wrapped)
		if code := ExitCode(out); code != 2 {
			t.Fatalf("expected exit code 2, got %d", code)
		}
	})

	t.Run("plain error passes through unchanged", func(t *testing.T) {
		plain := errors.New("普通错误")
		out := ExitUsageErr(plain)
		if out != plain {
			t.Fatal("expected plain error to be passed through unchanged")
		}
	})

	t.Run("nil passes through", func(t *testing.T) {
		out := ExitUsageErr(nil)
		if out != nil {
			t.Fatal("expected nil to pass through")
		}
	})
}
