package tracer

import (
	"context"
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

func TestMemoryDriver(t *testing.T) {
	driver := NewMemoryDriver()
	ctx := context.Background()

	t.Run("StartSpan creates span", func(t *testing.T) {
		newCtx, span := driver.StartSpan(ctx, "test-operation")

		if span == nil {
			t.Fatal("expected span")
		}
		if newCtx == nil {
			t.Fatal("expected context")
		}

		spanCtx := span.SpanContext()
		if spanCtx.TraceID == "" {
			t.Error("expected trace ID")
		}
		if spanCtx.SpanID == "" {
			t.Error("expected span ID")
		}

		span.End()
	})

	t.Run("Nested spans share trace ID", func(t *testing.T) {
		ctx1, span1 := driver.StartSpan(ctx, "parent")
		ctx2, span2 := driver.StartSpan(ctx1, "child")

		if span1.SpanContext().TraceID != span2.SpanContext().TraceID {
			t.Error("expected same trace ID for parent and child")
		}
		if span1.SpanContext().SpanID == span2.SpanContext().SpanID {
			t.Error("expected different span IDs")
		}

		span2.End()
		span1.End()
		_ = ctx2
	})

	t.Run("SetAttributes", func(t *testing.T) {
		_, span := driver.StartSpan(ctx, "test")
		span.SetAttributes(
			contracts.Attr("key1", "value1"),
			contracts.Attr("key2", 123),
		)
		span.End()
		// No panic is success
	})

	t.Run("AddEvent", func(t *testing.T) {
		_, span := driver.StartSpan(ctx, "test")
		span.AddEvent("something happened", contracts.Attr("detail", "info"))
		span.End()
		// No panic is success
	})

	t.Run("RecordError", func(t *testing.T) {
		_, span := driver.StartSpan(ctx, "test")
		span.RecordError(context.DeadlineExceeded)
		span.End()
		// No panic is success
	})

	t.Run("SetStatus", func(t *testing.T) {
		_, span := driver.StartSpan(ctx, "test")
		span.SetStatus(contracts.SpanStatusError, "something went wrong")
		span.End()
		// No panic is success
	})

	t.Run("SetName", func(t *testing.T) {
		_, span := driver.StartSpan(ctx, "original")
		span.SetName("renamed")
		span.End()
		// No panic is success
	})

	t.Run("IsRecording", func(t *testing.T) {
		_, span := driver.StartSpan(ctx, "test")
		if !span.IsRecording() {
			t.Error("expected span to be recording")
		}
		span.End()
	})

	t.Run("GetSpans returns all spans", func(t *testing.T) {
		d := NewMemoryDriver()
		_, s1 := d.StartSpan(ctx, "span1")
		s1.End()
		_, s2 := d.StartSpan(ctx, "span2")
		s2.End()

		spans := d.GetSpans()
		if len(spans) != 2 {
			t.Errorf("expected 2 spans, got %d", len(spans))
		}
	})

	t.Run("Close", func(t *testing.T) {
		err := driver.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})
}

func TestConsoleDriver(t *testing.T) {
	driver := NewConsoleDriver("test-service")
	ctx := context.Background()

	t.Run("StartSpan creates span", func(t *testing.T) {
		newCtx, span := driver.StartSpan(ctx, "console-test")

		if span == nil {
			t.Fatal("expected span")
		}
		if newCtx == nil {
			t.Fatal("expected context")
		}

		span.End()
	})

	t.Run("Nested spans", func(t *testing.T) {
		ctx1, span1 := driver.StartSpan(ctx, "parent")
		_, span2 := driver.StartSpan(ctx1, "child")

		span2.End()
		span1.End()
	})

	t.Run("Close", func(t *testing.T) {
		err := driver.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})
}

func TestNoopDriver(t *testing.T) {
	driver := &NoopDriver{}
	ctx := context.Background()

	t.Run("StartSpan returns noop span", func(t *testing.T) {
		newCtx, span := driver.StartSpan(ctx, "test")

		if newCtx != ctx {
			t.Error("expected same context for noop")
		}

		// All operations should not panic
		span.SetName("new name")
		span.SetStatus(contracts.SpanStatusOK, "ok")
		span.SetAttributes(contracts.Attr("key", "value"))
		span.AddEvent("event")
		span.RecordError(nil)
		span.End()

		if span.IsRecording() {
			t.Error("noop span should not be recording")
		}

		spanCtx := span.SpanContext()
		if spanCtx.IsValid() {
			t.Error("noop span context should not be valid")
		}
	})

	t.Run("Extract returns same context", func(t *testing.T) {
		carrier := contracts.MapCarrier{}
		result := driver.Extract(ctx, carrier)
		if result != ctx {
			t.Error("expected same context")
		}
	})

	t.Run("Inject returns nil", func(t *testing.T) {
		carrier := contracts.MapCarrier{}
		err := driver.Inject(ctx, carrier)
		if err != nil {
			t.Errorf("expected nil error: %v", err)
		}
	})

	t.Run("Close returns nil", func(t *testing.T) {
		err := driver.Close()
		if err != nil {
			t.Errorf("expected nil error: %v", err)
		}
	})
}

func TestContextPropagation(t *testing.T) {
	driver := NewMemoryDriver()
	ctx := context.Background()

	t.Run("Inject and Extract", func(t *testing.T) {
		// Start a span
		ctx1, span1 := driver.StartSpan(ctx, "original")

		// Inject into carrier
		carrier := contracts.MapCarrier{}
		err := driver.Inject(ctx1, carrier)
		if err != nil {
			t.Fatalf("Inject failed: %v", err)
		}

		// Should have some headers
		if len(carrier) == 0 {
			t.Error("expected carrier to have headers")
		}

		// Extract from carrier
		extractedCtx := driver.Extract(context.Background(), carrier)

		// Start a new span from extracted context
		_, span2 := driver.StartSpan(extractedCtx, "continued")

		// Should share same trace ID
		if span1.SpanContext().TraceID != span2.SpanContext().TraceID {
			t.Error("expected same trace ID after extract/inject")
		}

		span2.End()
		span1.End()
	})
}
