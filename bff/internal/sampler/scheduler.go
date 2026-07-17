package sampler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Scheduler runs periodic sampling and cleanup tasks via tickers.
type Scheduler struct {
	logger *slog.Logger
	tasks  []task
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type task struct {
	name     string
	interval time.Duration
	fn       func(ctx context.Context)
}

// New creates an empty Scheduler.
func New(logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{logger: logger}
}

// Register adds a periodic task.
func (s *Scheduler) Register(name string, interval time.Duration, fn func(ctx context.Context)) {
	s.tasks = append(s.tasks, task{name: name, interval: interval, fn: fn})
}

// Start begins all registered tasks. The first run happens after `interval`,
// so callers may invoke each task once before Start for immediate bootstrapping.
func (s *Scheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	for _, t := range s.tasks {
		t := t
		s.wg.Add(1)
		go s.runTask(ctx, t)
	}
}

func (s *Scheduler) runTask(ctx context.Context, t task) {
	defer s.wg.Done()
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						s.logger.Error("sampler task panic", "task", t.name, "panic", r)
					}
				}()
				t.fn(ctx)
			}()
		}
	}
}

// Stop cancels all tasks and waits for them to exit.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}
