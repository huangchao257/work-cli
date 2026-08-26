package usage

import (
	"errors"
	"testing"
)

func TestWrapfNoVerbOnly(t *testing.T) {
	// 无 %w：等价 Newf，仅格式化字符串。
	e := Wrapf("参数非法: %d", 42)
	if e.Error() != "参数非法: 42" {
		t.Fatalf("got %q, want %q", e.Error(), "参数非法: 42")
	}
	if e.err != nil {
		t.Fatalf("expected nil inner error, got %v", e.err)
	}
}

func TestWrapNoDuplicate(t *testing.T) {
	// Wrap 用 msg + inner 组成单条消息，不重复。
	inner := errors.New("boom")
	e := Wrap(inner, "操作失败")
	if e.Error() != "操作失败: boom" {
		t.Fatalf("got %q, want %q", e.Error(), "操作失败: boom")
	}
	if !errors.Is(e, inner) {
		t.Fatal("expected errors.Is to match inner error")
	}
}

func TestAsError(t *testing.T) {
	// 非 usage 错误：AsError 包成 usage error，消息不重复。
	inner := errors.New("plain")
	e := AsError(inner)
	if !Is(e) {
		t.Fatal("expected Is(AsError(plain)) to be true")
	}
	if e.Error() != "plain" {
		t.Fatalf("got %q, want %q", e.Error(), "plain")
	}

	// 已是 usage 错误：原样返回。
	ue := New("already usage")
	if AsError(ue) != ue {
		t.Fatal("expected AsError to return the same *Error")
	}
}
