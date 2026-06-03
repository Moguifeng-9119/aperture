package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Span struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Attrs     map[string]string
	Events    []SpanEvent
	Parent    *Span
	Children  []*Span
}

type SpanEvent struct {
	Name      string
	Timestamp time.Time
	Attrs     map[string]string
}

type Tracer struct {
	spans []*Span
	log   bool
}

func New(logSpans bool) *Tracer {
	return &Tracer{log: logSpans}
}

func (t *Tracer) Start(ctx context.Context, name string) (context.Context, *Span) {
	parent := SpanFromContext(ctx)
	s := &Span{
		Name:      name,
		StartTime: time.Now(),
		Attrs:     make(map[string]string),
		Parent:    parent,
	}
	if parent != nil {
		parent.Children = append(parent.Children, s)
	}
	t.spans = append(t.spans, s)
	return ContextWithSpan(ctx, s), s
}

func (s *Span) SetAttr(key, value string) {
	if s == nil {
		return
	}
	s.Attrs[key] = value
}

func (s *Span) AddEvent(name string, attrs map[string]string) {
	if s == nil {
		return
	}
	s.Events = append(s.Events, SpanEvent{
		Name:      name,
		Timestamp: time.Now(),
		Attrs:     attrs,
	})
}

func (s *Span) Finish() {
	if s == nil {
		return
	}
	s.EndTime = time.Now()
}

func (s *Span) FinishWithError(err error) {
	if s == nil {
		return
	}
	s.EndTime = time.Now()
	if err != nil {
		s.SetAttr("error", "true")
		s.SetAttr("error.message", err.Error())
	}
}

func (s *Span) Duration() time.Duration {
	if s == nil {
		return 0
	}
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

func (t *Tracer) Log(span *Span) {
	if !t.log || span == nil {
		return
	}

	attrs := make([]any, 0, len(span.Attrs)*2)
	for k, v := range span.Attrs {
		attrs = append(attrs, k, v)
	}

	msg := fmt.Sprintf("span[%s] %v", span.Name, span.Duration())
	slog.Info(msg, attrs...)

	for _, evt := range span.Events {
		evtAttrs := make([]any, 0, len(evt.Attrs)*2)
		for k, v := range evt.Attrs {
			evtAttrs = append(evtAttrs, k, v)
		}
		slog.Debug("  event: "+evt.Name, evtAttrs...)
	}

	for _, child := range span.Children {
		t.Log(child)
	}
}

type contextKey struct{}

func ContextWithSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, contextKey{}, span)
}

func SpanFromContext(ctx context.Context) *Span {
	s, _ := ctx.Value(contextKey{}).(*Span)
	return s
}
