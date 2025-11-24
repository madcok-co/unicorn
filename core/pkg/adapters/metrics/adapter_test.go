package metrics

import (
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

func TestMemoryDriver(t *testing.T) {
	driver := NewMemoryDriver()

	t.Run("Counter", func(t *testing.T) {
		counter := driver.Counter("test_counter", []contracts.Tag{
			{Key: "method", Value: "GET"},
		})

		counter.Inc()
		counter.Inc()
		counter.Add(5)

		val := driver.GetCounter("test_counter", contracts.Tag{Key: "method", Value: "GET"})
		if val != 7 {
			t.Errorf("expected 7, got %f", val)
		}
	})

	t.Run("Counter with different tags", func(t *testing.T) {
		counter1 := driver.Counter("http_requests", []contracts.Tag{
			{Key: "method", Value: "GET"},
			{Key: "path", Value: "/users"},
		})
		counter2 := driver.Counter("http_requests", []contracts.Tag{
			{Key: "method", Value: "POST"},
			{Key: "path", Value: "/users"},
		})

		counter1.Add(10)
		counter2.Add(5)

		val1 := driver.GetCounter("http_requests",
			contracts.Tag{Key: "method", Value: "GET"},
			contracts.Tag{Key: "path", Value: "/users"},
		)
		val2 := driver.GetCounter("http_requests",
			contracts.Tag{Key: "method", Value: "POST"},
			contracts.Tag{Key: "path", Value: "/users"},
		)

		if val1 != 10 {
			t.Errorf("expected 10 for GET, got %f", val1)
		}
		if val2 != 5 {
			t.Errorf("expected 5 for POST, got %f", val2)
		}
	})

	t.Run("Gauge", func(t *testing.T) {
		gauge := driver.Gauge("active_connections", nil)

		gauge.Set(100)
		val := driver.GetGauge("active_connections")
		if val != 100 {
			t.Errorf("expected 100, got %f", val)
		}

		gauge.Inc()
		val = driver.GetGauge("active_connections")
		if val != 101 {
			t.Errorf("expected 101, got %f", val)
		}

		gauge.Dec()
		val = driver.GetGauge("active_connections")
		if val != 100 {
			t.Errorf("expected 100, got %f", val)
		}

		gauge.Add(50)
		val = driver.GetGauge("active_connections")
		if val != 150 {
			t.Errorf("expected 150, got %f", val)
		}

		gauge.Sub(30)
		val = driver.GetGauge("active_connections")
		if val != 120 {
			t.Errorf("expected 120, got %f", val)
		}
	})

	t.Run("Histogram", func(t *testing.T) {
		histogram := driver.Histogram("request_duration", nil)

		histogram.Observe(0.1)
		histogram.Observe(0.2)
		histogram.Observe(0.3)
		histogram.Observe(0.5)
		histogram.Observe(1.0)

		count := driver.GetHistogramCount("request_duration")
		if count != 5 {
			t.Errorf("expected 5 observations, got %d", count)
		}
	})

	t.Run("Handler returns nil for memory driver", func(t *testing.T) {
		h := driver.Handler()
		if h != nil {
			t.Error("expected nil handler for memory driver")
		}
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

	t.Run("Counter operations don't panic", func(t *testing.T) {
		counter := driver.Counter("test", nil)
		counter.Inc()
		counter.Add(5)
	})

	t.Run("Gauge operations don't panic", func(t *testing.T) {
		gauge := driver.Gauge("test", nil)
		gauge.Set(100)
		gauge.Inc()
		gauge.Dec()
		gauge.Add(10)
		gauge.Sub(5)
	})

	t.Run("Histogram operations don't panic", func(t *testing.T) {
		histogram := driver.Histogram("test", nil)
		histogram.Observe(1.0)
	})

	t.Run("Handler returns nil", func(t *testing.T) {
		if driver.Handler() != nil {
			t.Error("expected nil handler")
		}
	})

	t.Run("Close returns nil", func(t *testing.T) {
		if driver.Close() != nil {
			t.Error("expected nil error")
		}
	})
}

func TestSameMetricReturned(t *testing.T) {
	driver := NewMemoryDriver()

	t.Run("Same counter for same key", func(t *testing.T) {
		c1 := driver.Counter("test", nil)
		c1.Add(10)

		c2 := driver.Counter("test", nil)
		c2.Add(5)

		val := driver.GetCounter("test")
		if val != 15 {
			t.Errorf("expected 15, got %f", val)
		}
	})

	t.Run("Same gauge for same key", func(t *testing.T) {
		g1 := driver.Gauge("gauge_test", nil)
		g1.Set(100)

		g2 := driver.Gauge("gauge_test", nil)
		g2.Inc()

		val := driver.GetGauge("gauge_test")
		if val != 101 {
			t.Errorf("expected 101, got %f", val)
		}
	})
}
