package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// controllableDeadlineContext lets pipe-drain tests begin the command only after
// its background descendant has inherited the output pipes, then
// deterministically deliver the DeadlineExceeded signal that RunCapture
// reports to callers. A fixed short timeout races process start under
// parallel CI load. It is used by both the Unix (runner_test.go) and
// Windows (runner_pipe_drain_windows_test.go) pipe-drain tests.
type controllableDeadlineContext struct {
	done chan struct{}
	once sync.Once
}

func newControllableDeadlineContext() *controllableDeadlineContext {
	return &controllableDeadlineContext{done: make(chan struct{})}
}

func (c *controllableDeadlineContext) Deadline() (time.Time, bool) { return time.Time{}, false }

func (c *controllableDeadlineContext) Done() <-chan struct{} { return c.done }

func (c *controllableDeadlineContext) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func (c *controllableDeadlineContext) Value(any) any { return nil }

func (c *controllableDeadlineContext) expire() {
	c.once.Do(func() { close(c.done) })
}

func awaitFixtureFile(path string, limit time.Duration) error {
	timer := time.NewTimer(limit)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		select {
		case <-timer.C:
			return fmt.Errorf("timed out after %s", limit)
		case <-ticker.C:
		}
	}
}
