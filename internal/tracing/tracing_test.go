package tracing

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSpanLifecycle(t *testing.T) {
	tracer := New(false)
	ctx := context.Background()

	ctx, span := tracer.Start(ctx, "test-span")
	span.SetAttr("key", "value")
	span.AddEvent("step-1", map[string]string{"detail": "started"})
	span.Finish()

	if span.Duration() < 0 {
		t.Error("expected non-negative duration")
	}
	if span.Attrs["key"] != "value" {
		t.Error("expected attr to be set")
	}
	if len(span.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(span.Events))
	}

	_ = ctx
}

func TestSpanFinishWithError(t *testing.T) {
	tracer := New(false)
	ctx := context.Background()

	_, span := tracer.Start(ctx, "failing-span")
	span.FinishWithError(errors.New("something went wrong"))

	if span.Attrs["error"] != "true" {
		t.Error("expected error attr to be set")
	}
	if span.Duration() < 0 {
		t.Error("expected non-negative duration")
	}
}

func TestParentChildSpan(t *testing.T) {
	tracer := New(false)

	ctx, parent := tracer.Start(context.Background(), "parent")
	_, child := tracer.Start(ctx, "child")
	child.Finish()
	parent.Finish()

	if child.Parent != parent {
		t.Error("expected child.Parent to be parent span")
	}
	if len(parent.Children) != 1 {
		t.Errorf("expected parent to have 1 child, got %d", len(parent.Children))
	}
	if parent.Children[0] != child {
		t.Error("expected parent.Children[0] to be child span")
	}
	if parent.Duration() < child.Duration() {
		t.Error("parent duration should be >= child duration")
	}
}

func TestNilSpanSafety(t *testing.T) {
	var span *Span
	span.SetAttr("key", "value")
	span.AddEvent("event", nil)
	span.Finish()
	span.FinishWithError(nil)

	if span.Duration() != 0 {
		t.Error("nil span should return zero duration")
	}
}

func TestSpanFromContext(t *testing.T) {
	ctx := context.Background()
	if s := SpanFromContext(ctx); s != nil {
		t.Error("expected nil span from empty context")
	}

	tracer := New(false)
	ctx, span := tracer.Start(ctx, "test")
	retrieved := SpanFromContext(ctx)
	if retrieved != span {
		t.Error("expected same span from context")
	}
}

func TestSpanDurationWhileRunning(t *testing.T) {
	tracer := New(false)
	_, span := tracer.Start(context.Background(), "running")
	d := span.Duration()
	if d < 0 || d > time.Second {
		t.Errorf("expected small non-negative duration while running, got %v", d)
	}
}

func TestTracerLog(t *testing.T) {
	tracer := New(true)
	ctx, parent := tracer.Start(context.Background(), "log-test")
	parent.SetAttr("component", "test")
	_, child := tracer.Start(ctx, "child-span")
	child.Finish()
	parent.Finish()

	// Log should not panic
	tracer.Log(parent)
}
