// Package cron provides a generic cron adapter for scheduled job execution
package cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/handler"
)

// Scheduler is the interface that any cron library must implement
// This allows plugging in any cron library (robfig/cron, go-co-op/gocron, etc.)
type Scheduler interface {
	// AddFunc adds a job with cron expression
	AddFunc(spec string, cmd func()) error
	// Start starts the scheduler
	Start()
	// Stop stops the scheduler
	Stop()
}

// Config for cron adapter
type Config struct {
	// Location for cron expressions (default: Local)
	Location *time.Location
	// RecoverPanic if true, recover from panics in jobs
	RecoverPanic bool
	// Logger for cron events
	Logger Logger
}

// Logger interface for cron logging
type Logger interface {
	Info(msg string, fields ...any)
	Error(msg string, fields ...any)
}

// Job represents a scheduled job
type Job struct {
	Name     string
	Schedule string
	Handler  *handler.Handler
}

// Adapter handles cron trigger execution
type Adapter struct {
	scheduler  Scheduler
	config     *Config
	jobs       []*Job
	registry   *handler.Registry
	ctxFactory func(context.Context) *ucontext.Context
	mu         sync.RWMutex
	running    bool
	stopCh     chan struct{}
}

// New creates a new cron adapter
// scheduler can be any implementation of Scheduler interface
func New(registry *handler.Registry, scheduler Scheduler, config *Config) *Adapter {
	if config == nil {
		config = &Config{
			Location:     time.Local,
			RecoverPanic: true,
		}
	}

	return &Adapter{
		scheduler: scheduler,
		config:    config,
		registry:  registry,
		jobs:      make([]*Job, 0),
		stopCh:    make(chan struct{}),
	}
}

// SetContextFactory sets the function to create unicorn context
func (a *Adapter) SetContextFactory(factory func(context.Context) *ucontext.Context) {
	a.ctxFactory = factory
}

// Start starts the cron scheduler
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return nil
	}
	a.running = true
	a.mu.Unlock()

	// Register all cron handlers from registry
	handlers := a.registry.GetCronHandlers()
	for _, h := range handlers {
		for _, trigger := range h.Triggers() {
			if cronTrigger, ok := trigger.(*handler.CronTrigger); ok {
				if err := a.addJob(h, cronTrigger.Schedule); err != nil {
					return fmt.Errorf("failed to register cron job %s: %w", h.Name, err)
				}
			}
		}
	}

	// Start scheduler
	a.scheduler.Start()

	if a.config.Logger != nil {
		a.config.Logger.Info("cron scheduler started", "jobs", len(a.jobs))
	}

	// Wait for context cancellation
	select {
	case <-ctx.Done():
		return a.Stop()
	case <-a.stopCh:
		return nil
	}
}

// Stop stops the cron scheduler
func (a *Adapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	a.scheduler.Stop()
	a.running = false

	select {
	case <-a.stopCh:
	default:
		close(a.stopCh)
	}

	if a.config.Logger != nil {
		a.config.Logger.Info("cron scheduler stopped")
	}

	return nil
}

// addJob adds a job to the scheduler
func (a *Adapter) addJob(h *handler.Handler, schedule string) error {
	job := &Job{
		Name:     h.Name,
		Schedule: schedule,
		Handler:  h,
	}

	// Create the job function
	jobFunc := a.createJobFunc(job)

	// Add to scheduler
	if err := a.scheduler.AddFunc(schedule, jobFunc); err != nil {
		return err
	}

	a.jobs = append(a.jobs, job)
	return nil
}

// createJobFunc creates the function to execute for a job
func (a *Adapter) createJobFunc(job *Job) func() {
	return func() {
		if a.config.RecoverPanic {
			defer func() {
				if r := recover(); r != nil {
					if a.config.Logger != nil {
						a.config.Logger.Error("cron job panic recovered",
							"job", job.Name,
							"panic", r,
						)
					}
				}
			}()
		}

		// Create context
		ctx := context.Background()
		var uctx *ucontext.Context
		if a.ctxFactory != nil {
			uctx = a.ctxFactory(ctx)
		} else {
			uctx = ucontext.New(ctx)
		}

		// Set request info
		uctx.SetRequest(&ucontext.Request{
			TriggerType: "cron",
			Headers: map[string]string{
				"X-Cron-Job":      job.Name,
				"X-Cron-Schedule": job.Schedule,
			},
		})

		// Execute handler
		start := time.Now()
		executor := handler.NewExecutor(job.Handler)
		err := executor.Execute(uctx)
		duration := time.Since(start)

		if err != nil {
			if a.config.Logger != nil {
				a.config.Logger.Error("cron job failed",
					"job", job.Name,
					"duration", duration,
					"error", err,
				)
			}
		} else {
			if a.config.Logger != nil {
				a.config.Logger.Info("cron job completed",
					"job", job.Name,
					"duration", duration,
				)
			}
		}
	}
}

// Jobs returns all registered jobs
func (a *Adapter) Jobs() []*Job {
	return a.jobs
}

// ============ Built-in Simple Scheduler ============

// SimpleScheduler is a basic scheduler implementation
// For production, use robfig/cron or go-co-op/gocron
type SimpleScheduler struct {
	jobs    []*simpleJob
	running bool
	stopCh  chan struct{}
	mu      sync.Mutex
	wg      sync.WaitGroup
}

type simpleJob struct {
	spec     string
	cmd      func()
	interval time.Duration
}

// NewSimpleScheduler creates a basic scheduler
// Note: This only supports simple intervals like "@every 5m", not full cron expressions
func NewSimpleScheduler() *SimpleScheduler {
	return &SimpleScheduler{
		jobs:   make([]*simpleJob, 0),
		stopCh: make(chan struct{}),
	}
}

// AddFunc adds a job (supports @every syntax only for simple scheduler)
func (s *SimpleScheduler) AddFunc(spec string, cmd func()) error {
	interval, err := parseSimpleSpec(spec)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.jobs = append(s.jobs, &simpleJob{
		spec:     spec,
		cmd:      cmd,
		interval: interval,
	})
	s.mu.Unlock()
	return nil
}

// Start starts the scheduler
func (s *SimpleScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	for _, job := range s.jobs {
		s.wg.Add(1)
		go s.runJob(job)
	}
}

// Stop stops the scheduler
func (s *SimpleScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *SimpleScheduler) runJob(job *simpleJob) {
	defer s.wg.Done()
	ticker := time.NewTicker(job.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			job.cmd()
		}
	}
}

// parseSimpleSpec parses simple interval specs like "@every 5m"
func parseSimpleSpec(spec string) (time.Duration, error) {
	if len(spec) > 7 && spec[:7] == "@every " {
		return time.ParseDuration(spec[7:])
	}
	return 0, fmt.Errorf("simple scheduler only supports @every syntax, got: %s", spec)
}

// ============ Wrapper for robfig/cron ============

// RobfigCronWrapper wraps robfig/cron v3 to implement Scheduler interface
// Usage:
//
//	import "github.com/robfig/cron/v3"
//	c := cron.New()
//	adapter := cronAdapter.New(cronAdapter.WrapRobfigCron(c), registry, config)
type RobfigCronWrapper struct {
	cron RobfigCron
}

// RobfigCron is the interface that robfig/cron implements
type RobfigCron interface {
	AddFunc(spec string, cmd func()) (int, error)
	Start()
	Stop()
}

// WrapRobfigCron wraps a robfig/cron instance
func WrapRobfigCron(c RobfigCron) Scheduler {
	return &RobfigCronWrapper{cron: c}
}

func (w *RobfigCronWrapper) AddFunc(spec string, cmd func()) error {
	_, err := w.cron.AddFunc(spec, cmd)
	return err
}

func (w *RobfigCronWrapper) Start() {
	w.cron.Start()
}

func (w *RobfigCronWrapper) Stop() {
	w.cron.Stop()
}
