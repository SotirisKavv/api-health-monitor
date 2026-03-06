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
	ExecFunc  func(models.Target) error
	Timeout   time.Duration
}

type PriorityHeap []Task

func (h PriorityHeap) Len() int           { return len(h) }
func (h PriorityHeap) Less(i, j int) bool { return h[i].ExecuteAt.Before(h[j].ExecuteAt) }
func (h PriorityHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

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
	tasks    *PriorityHeap
	taskChan chan Task
	workers  int
	stop     chan struct{}
	mu       sync.Mutex
}

func NewScheduler(workers int) *Scheduler {
	h := make(PriorityHeap, 0)
	heap.Init(&h)
	s := &Scheduler{
		tasks:    &h,
		taskChan: make(chan Task, 100),
		workers:  workers,
		stop:     make(chan struct{}),
	}
	go s.dispatch()
	go s.adjustWorkers()
	s.Start()
	return s
}

func (s *Scheduler) dispatch() {
	for {
		s.mu.Lock()
		if s.tasks.Len() > 0 {
			task := heap.Pop(s.tasks).(Task)
			s.mu.Unlock()
			s.taskChan <- task
		} else {
			s.mu.Unlock()
			time.Sleep(100 * time.Millisecond)
		}
		select {
		case <-s.stop:
			return
		default:
		}
	}
}

func (s *Scheduler) Start() {
	for i := 0; i < s.workers; i++ {
		go func(workerId int) {
			for {
				select {
				case task := <-s.taskChan:
					if time.Until(task.ExecuteAt) > 0 {
						time.Sleep(time.Until(task.ExecuteAt))
					}
					ctx, cancel := context.WithTimeout(context.Background(), task.Timeout)
					done := make(chan error, 1)
					go func() {
						done <- task.ExecFunc(task.target)
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
				case <-s.stop:
					return
				}
			}
		}(i)
	}
}

func (s *Scheduler) Submit(task Task) {
	s.mu.Lock()
	heap.Push(s.tasks, task)
	s.mu.Unlock()
}

func (s *Scheduler) Remove(target models.Target) {
	s.mu.Lock()
	for i, task := range *s.tasks {
		if task.target.ID == target.ID {
			heap.Remove(s.tasks, i)
			break
		}
	}
	s.mu.Unlock()
}

func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) adjustWorkers() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			taskLen := s.tasks.Len()
			s.mu.Unlock()
			idealWorkers := (taskLen / 10) + 1 // 1 worker for 10 tasks
			if idealWorkers > s.workers {
				for i := s.workers; i < idealWorkers; i++ {
					go s.startWorker(i)
				}
				s.workers = idealWorkers
			}
		case <-s.stop:
			return
		}
	}
}

func (s *Scheduler) startWorker(workerId int) {
	for {
		select {
		case task := <-s.taskChan:
			if time.Until(task.ExecuteAt) > 0 {
				time.Sleep(time.Until(task.ExecuteAt))
			}
			ctx, cancel := context.WithTimeout(context.Background(), task.Timeout)
			done := make(chan error, 1)
			go func() {
				done <- task.ExecFunc(task.target)
			}()
			select {
			case err := <-done:
				cancel()
				if err != nil {
					log.Printf("Worker %d: Task execution failed for target %s: %v", workerId, task.target.Name, err)
				}
			case <-ctx.Done():
				cancel()
				log.Printf("Worker %d: Task execution timed out for target %s", workerId, task.target.Name)
			}
		case <-s.stop:
			return
		}
	}
}
