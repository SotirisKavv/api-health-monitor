package probe

import (
	"container/heap"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
)

func newTestScheduler() *Scheduler {
	h := make(PriorityHeap, 0)
	heap.Init(&h)
	return &Scheduler{
		tasks:     &h,
		taskChan:  make(chan Task, 10),
		workers:   0,
		wakeUp:    make(chan struct{}, 1),
		scheduled: make(map[string]Task),
	}
}

func TestPriorityHeapOrdersTasksByExecutionTime(t *testing.T) {
	h := make(PriorityHeap, 0)
	heap.Init(&h)
	later := Task{target: models.Target{ID: "later"}, ExecuteAt: time.Now().Add(2 * time.Second)}
	soon := Task{target: models.Target{ID: "soon"}, ExecuteAt: time.Now().Add(1 * time.Second)}

	heap.Push(&h, later)
	heap.Push(&h, soon)

	if h.Len() != 2 {
		t.Fatalf("expected heap length 2, got %d", h.Len())
	}
	if !h.Less(0, 1) {
		t.Fatalf("expected first task to execute sooner than second task")
	}
	if h.Peek() == nil || h.Peek().target.ID != "soon" {
		t.Fatalf("expected Peek() to return the earliest task")
	}
	h.Swap(0, 1)
	h.Swap(0, 1)
	if task := heap.Pop(&h).(Task); task.target.ID != "soon" {
		t.Fatalf("expected heap.Pop() to return the earliest task, got %s", task.target.ID)
	}
}

func TestScheduler_SubmitRemoveAndDispatchReadyItems(t *testing.T) {
	s := newTestScheduler()
	task := Task{target: models.Target{ID: "target-1"}, ExecuteAt: time.Now().Add(-time.Second)}

	s.Submit(task)
	s.Submit(task)
	if got := s.ScheduledCount(); got != 1 {
		t.Fatalf("expected duplicate submit to be ignored, got %d tasks", got)
	}

	s.dispatchReadyItems()
	select {
	case got := <-s.taskChan:
		if got.target.ID != task.target.ID {
			t.Fatalf("expected dispatched task %s, got %s", task.target.ID, got.target.ID)
		}
	default:
		t.Fatalf("expected ready task to be dispatched")
	}

	s.Submit(Task{target: models.Target{ID: "target-2"}, ExecuteAt: time.Now().Add(time.Minute)})
	s.Remove(models.Target{ID: "target-2"})
	if got := s.ScheduledCount(); got != 0 {
		t.Fatalf("expected removed task to disappear, got %d tasks", got)
	}

	s.Stop()
	s.Submit(Task{target: models.Target{ID: "closed"}, ExecuteAt: time.Now()})
	if got := s.ScheduledCount(); got != 0 {
		t.Fatalf("expected closed scheduler to reject new tasks, got %d tasks", got)
	}
}

func TestScheduler_ResetTimerAndWakeUp(t *testing.T) {
	s := newTestScheduler()
	timer := s.resetTimer(nil, 50*time.Millisecond)
	if timer == nil {
		t.Fatalf("expected resetTimer() to create a timer")
	}

	start := time.Now()
	s.waitForTimerOrWakeUp(timer)
	if time.Since(start) < 40*time.Millisecond {
		t.Fatalf("expected waitForTimerOrWakeUp() to wait for the timer")
	}

	timer = s.resetTimer(timer, time.Second)
	s.Notify()
	start = time.Now()
	s.waitForTimerOrWakeUp(timer)
	if time.Since(start) > 300*time.Millisecond {
		t.Fatalf("expected wake-up notification to interrupt timer wait")
	}
}

func TestScheduler_StartWorkerResubmitsAfterExecution(t *testing.T) {
	s := newTestScheduler()
	go s.startWorker(1)

	task := Task{
		target:    models.Target{ID: "worker-target", Name: "worker-target", Interval: 1},
		ExecuteAt: time.Now(),
		Timeout:   200 * time.Millisecond,
		ExecFunc: func(context.Context, models.Target) error {
			return errors.New("boom")
		},
	}
	s.taskChan <- task

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if s.ScheduledCount() == 1 {
			close(s.taskChan)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(s.taskChan)
	t.Fatalf("expected worker to resubmit the task after execution")
}

func TestScheduler_LoopClosesTaskChannelWhenStopped(t *testing.T) {
	s := newTestScheduler()
	done := make(chan struct{})

	go func() {
		s.Loop()
		close(done)
	}()

	s.Stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("expected Loop() to return after Stop()")
	}

	_, ok := <-s.taskChan
	if ok {
		t.Fatalf("expected task channel to be closed when loop exits")
	}
}
