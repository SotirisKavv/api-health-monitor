package probe

import (
	"container/heap"
	"context"
	"log"
	"sync"
	"time"

	"github.com/SotirisKavv/api-health-monitor/internal/models"
)

type Task struct {
	target    models.Target
	ExecuteAt time.Time
	ExecFunc  func(context.Context, models.Target) error
	Timeout   time.Duration
}

type PriorityHeap []Task

func (h PriorityHeap) Len() int           { return len(h) }
func (h PriorityHeap) Less(i, j int) bool { return h[i].ExecuteAt.Before(h[j].ExecuteAt) }
func (h PriorityHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h PriorityHeap) Peek() *Task {
	if len(h) == 0 {
		return nil
	}
	return &h[0]
}

func (h *PriorityHeap) Push(x any) {
	*h = append(*h, x.(Task))
}

func (h *PriorityHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

type Scheduler struct {
	tasks     *PriorityHeap
	scheduled map[string]Task
	taskChan  chan Task
	workers   int
	wakeUp    chan struct{}
	closed    bool
	mu        sync.Mutex
}

func NewScheduler(workers int) *Scheduler {
	h := make(PriorityHeap, 0)
	heap.Init(&h)
	s := &Scheduler{
		tasks:     &h,
		taskChan:  make(chan Task, 100),
		workers:   workers,
		wakeUp:    make(chan struct{}, 1),
		scheduled: make(map[string]Task),
		closed:    false,
	}
	go s.Loop()
	go s.adjustWorkers()
	s.Start()
	return s
}

func (s *Scheduler) Start() {
	for i := 0; i < s.workers; i++ {
		go s.startWorker(i)
	}
}

func (s *Scheduler) Submit(task Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	if _, exists := s.scheduled[task.target.ID]; !exists {
		heap.Push(s.tasks, task)
		s.scheduled[task.target.ID] = task
		s.Notify()
	}

}

func (s *Scheduler) Remove(target models.Target) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.scheduled[target.ID]; exists {
		delete(s.scheduled, target.ID)
		for i, task := range *s.tasks {
			if task.target.ID == target.ID {
				heap.Remove(s.tasks, i)
				break
			}
		}
	}
}

func (s *Scheduler) ScheduledCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tasks.Len()
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.Notify()
}

func (s *Scheduler) Notify() {
	select {
	case s.wakeUp <- struct{}{}:
	default:
	}
}

func (s *Scheduler) Loop() {
	var timer *time.Timer

	for {
		closed, empty, next := s.getQueueState()

		if closed && empty {
			close(s.taskChan)
			return
		}

		if empty {
			s.waitForWakeUp()
			continue
		}

		delay := time.Until(next)
		if delay <= 0 {
			s.dispatchReadyItems()
			continue
		}

		timer = s.resetTimer(timer, delay)
		s.waitForTimerOrWakeUp(timer)
	}
}

func (s *Scheduler) dispatchReadyItems() {
	now := time.Now()
	for {
		s.mu.Lock()
		head := s.tasks.Peek()
		if head == nil || head.ExecuteAt.After(now) {
			s.mu.Unlock()
			break
		}
		task := heap.Pop(s.tasks).(Task)
		delete(s.scheduled, task.target.ID)
		s.mu.Unlock()
		s.taskChan <- task
	}
}

func (s *Scheduler) getQueueState() (closed, empty bool, next time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	empty = s.tasks.Len() == 0
	closed = s.closed
	if !empty {
		next = s.tasks.Peek().ExecuteAt
	}
	return
}

func (s *Scheduler) waitForWakeUp() {
	<-s.wakeUp
}

func (s *Scheduler) adjustWorkers() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		taskLen := s.tasks.Len()
		s.mu.Unlock()
		idealWorkers := (taskLen / 10) + 1 // 1 worker for 10 tasks
		if idealWorkers > s.workers {
			for i := s.workers; i < idealWorkers; i++ {
				go s.startWorker(i)
			}
			s.workers = idealWorkers
		}
	}
}

func (s *Scheduler) resetTimer(timer *time.Timer, delay time.Duration) *time.Timer {
	if timer == nil {
		return time.NewTimer(delay)
	}

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(delay)
	return timer
}

func (s *Scheduler) waitForTimerOrWakeUp(timer *time.Timer) {
	select {
	case <-timer.C:
	case <-s.wakeUp:
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
}

func (s *Scheduler) startWorker(workerId int) {
	for task := range s.taskChan {
		ctx, cancel := context.WithTimeout(context.Background(), task.Timeout)
		done := make(chan error, 1)
		go func() {
			done <- task.ExecFunc(ctx, task.target)
		}()
		select {
		case err := <-done:
			cancel()
			if err != nil {
				log.Printf("Worker %d: Task execution failed for target %s: %v", workerId, task.target.Name, err)
			}
			s.Submit(Task{
				target:    task.target,
				ExecuteAt: time.Now().Add(time.Duration(task.target.Interval) * time.Second),
				ExecFunc:  task.ExecFunc,
				Timeout:   task.Timeout,
			})
		case <-ctx.Done():
			cancel()
			log.Printf("Worker %d: Task execution timed out for target %s", workerId, task.target.Name)
			s.Submit(Task{
				target:    task.target,
				ExecuteAt: time.Now().Add(task.Timeout),
				ExecFunc:  task.ExecFunc,
				Timeout:   task.Timeout,
			})
		}
	}
}
