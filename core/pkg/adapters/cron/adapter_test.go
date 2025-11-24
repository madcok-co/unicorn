package cron

import (
	"testing"
	"time"
)

func TestSimpleScheduler(t *testing.T) {
	t.Run("AddFunc with @every syntax", func(t *testing.T) {
		scheduler := NewSimpleScheduler()

		err := scheduler.AddFunc("@every 1s", func() {})
		if err != nil {
			t.Errorf("AddFunc failed: %v", err)
		}
	})

	t.Run("AddFunc with invalid syntax", func(t *testing.T) {
		scheduler := NewSimpleScheduler()

		err := scheduler.AddFunc("0 * * * *", func() {})
		if err == nil {
			t.Error("expected error for non-@every syntax")
		}
	})

	t.Run("Start and Stop", func(t *testing.T) {
		scheduler := NewSimpleScheduler()
		counter := 0

		scheduler.AddFunc("@every 50ms", func() {
			counter++
		})

		scheduler.Start()
		time.Sleep(150 * time.Millisecond)
		scheduler.Stop()

		// Should have run 2-3 times
		if counter < 2 {
			t.Errorf("expected at least 2 executions, got %d", counter)
		}
	})

	t.Run("Stop before start", func(t *testing.T) {
		scheduler := NewSimpleScheduler()
		// Should not panic
		scheduler.Stop()
	})

	t.Run("Multiple jobs", func(t *testing.T) {
		scheduler := NewSimpleScheduler()
		counter1 := 0
		counter2 := 0

		scheduler.AddFunc("@every 50ms", func() {
			counter1++
		})
		scheduler.AddFunc("@every 100ms", func() {
			counter2++
		})

		scheduler.Start()
		time.Sleep(250 * time.Millisecond)
		scheduler.Stop()

		if counter1 < 4 {
			t.Errorf("expected at least 4 executions for job1, got %d", counter1)
		}
		if counter2 < 2 {
			t.Errorf("expected at least 2 executions for job2, got %d", counter2)
		}
	})
}

func TestParseSimpleSpec(t *testing.T) {
	tests := []struct {
		spec     string
		expected time.Duration
		hasError bool
	}{
		{"@every 1s", time.Second, false},
		{"@every 5m", 5 * time.Minute, false},
		{"@every 1h", time.Hour, false},
		{"@every 500ms", 500 * time.Millisecond, false},
		{"0 * * * *", 0, true},
		{"invalid", 0, true},
		{"@every invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			duration, err := parseSimpleSpec(tt.spec)

			if tt.hasError && err == nil {
				t.Error("expected error")
			}
			if !tt.hasError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.hasError && duration != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, duration)
			}
		})
	}
}

func TestRobfigCronWrapper(t *testing.T) {
	// Mock robfig cron
	mock := &mockRobfigCron{
		jobs: make([]func(), 0),
	}

	wrapper := WrapRobfigCron(mock)

	t.Run("AddFunc", func(t *testing.T) {
		err := wrapper.AddFunc("@every 1s", func() {})
		if err != nil {
			t.Errorf("AddFunc failed: %v", err)
		}

		if len(mock.jobs) != 1 {
			t.Errorf("expected 1 job, got %d", len(mock.jobs))
		}
	})

	t.Run("Start", func(t *testing.T) {
		wrapper.Start()
		if !mock.started {
			t.Error("expected Start to be called")
		}
	})

	t.Run("Stop", func(t *testing.T) {
		wrapper.Stop()
		if !mock.stopped {
			t.Error("expected Stop to be called")
		}
	})
}

// Mock implementation of robfig/cron interface
type mockRobfigCron struct {
	jobs    []func()
	started bool
	stopped bool
}

func (m *mockRobfigCron) AddFunc(spec string, cmd func()) (int, error) {
	m.jobs = append(m.jobs, cmd)
	return len(m.jobs), nil
}

func (m *mockRobfigCron) Start() {
	m.started = true
}

func (m *mockRobfigCron) Stop() {
	m.stopped = true
}
