// ============================================
// 4. CRON TRIGGER (Scheduled Jobs)
// ============================================
package triggers

import (
	"context"
	"sync"

	"github.com/madcok-co/unicorn"

	"github.com/robfig/cron/v3"
)

type CronTrigger struct {
	cron *cron.Cron
	jobs map[string]cron.EntryID
	mu   sync.RWMutex
}

func NewCronTrigger() *CronTrigger {
	return &CronTrigger{
		cron: cron.New(),
		jobs: make(map[string]cron.EntryID),
	}
}

func (t *CronTrigger) AddJob(name, schedule, serviceName string, request map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Get service handler
	handler, err := unicorn.GetHandler(serviceName)
	if err != nil {
		return err
	}

	// Add cron job
	entryID, err := t.cron.AddFunc(schedule, func() {
		ctx := unicorn.NewContext(context.Background())
		ctx.SetMetadata("cron_job", name)
		ctx.SetMetadata("service_name", serviceName)

		handler.Handle(ctx, request)
	})

	if err != nil {
		return err
	}

	t.jobs[name] = entryID
	return nil
}

func (t *CronTrigger) RemoveJob(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if entryID, ok := t.jobs[name]; ok {
		t.cron.Remove(entryID)
		delete(t.jobs, name)
	}

	return nil
}

func (t *CronTrigger) Start() error {
	t.cron.Start()
	return nil
}

func (t *CronTrigger) Stop() error {
	t.cron.Stop()
	return nil
}
