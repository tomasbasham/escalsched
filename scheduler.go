package escalsched

import (
	"container/heap"
	"context"
	"iter"
	"sync"
	"time"
)

// Ensure Scheduledr implements [heap.Interface].
var _ heap.Interface = (*Scheduler[any])(nil)

// EscalationStep defines a single step in an escalation policy.
type EscalationStep struct {
	After  time.Duration
	Target Priority
}

// EscalationPolicy defines a series of escalation steps.
type EscalationPolicy []EscalationStep

// MetricsHook defines hooks for monitoring schedule, unschedule, and escalation
// events.
type MetricsHook[T any] interface {
	OnSchedule(task *Task[T])
	OnUnschedule(task *Task[T])
	OnEscalate(task *Task[T], from, to Priority)
}

// Scheduler is a priority queue that supports the following operations:
//
//   - Schedule with priority and optional escalation policy
//   - Unschedule, blocking until a task is available
//   - Manual escalation of a task
//   - Fairness window to prevent starvation
//   - Metrics hooks for schedule, unschedule, and escalation events
//
// Tasks with higher priority are unscheduled first. If two tasks have the same
// priority, they are unscheduled in FIFO order. If a fairness window is set,
// and the last unschedule was longer ago than the fairness window, tasks are
// unscheduled in FIFO order regardless of priority.
type Scheduler[T any] struct {
	mu      sync.Mutex
	metrics MetricsHook[T]

	tasks []*Task[T]
	seqNo int64

	// Time based escalation.
	fairnessWindow time.Duration
	lastDequeueAt  time.Time

	notifyCh chan struct{}
}

// New creates a new [Scheduler] with the given options.
func New[T any](opts ...Option[T]) *Scheduler[T] {
	o := &Options[T]{}
	for _, opt := range opts {
		opt(o)
	}

	s := &Scheduler[T]{
		tasks:          make([]*Task[T], 0),
		notifyCh:       make(chan struct{}, 1),
		fairnessWindow: o.FairnessWindow,
		metrics:        o.Metrics,
	}

	heap.Init(s)
	return s
}

// Schedule adds a new [Task] with the given value and priority to the
// scheduler.
func (s *Scheduler[T]) Schedule(value T, priority Priority) *Task[T] {
	return s.schedule(value, priority, nil)
}

// ScheduleWithPolicy adds a new [Task] with the given value, priority, and
// escalation policy to the scheduler.
func (s *Scheduler[T]) ScheduleWithPolicy(value T, priority Priority, policy EscalationPolicy) *Task[T] {
	return s.schedule(value, priority, policy)
}

func (s *Scheduler[T]) schedule(value T, priority Priority, policy EscalationPolicy) *Task[T] {
	task := &Task[T]{
		Value:       value,
		Priority:    priority,
		index:       -1,
		escalatedCh: make(chan struct{}),
		doneCh:      make(chan struct{}),
	}

	s.mu.Lock()
	task.seqNo = s.seqNo
	task.enqueueAt = time.Now()
	s.seqNo++
	heap.Push(s, task)
	s.mu.Unlock()

	if s.metrics != nil {
		s.metrics.OnSchedule(task)
	}

	s.notify()

	if policy != nil {
		s.applyPolicy(task, policy)
	}

	return task
}

func (s *Scheduler[T]) applyPolicy(task *Task[T], policy EscalationPolicy) {
	for _, step := range policy {
		go func() {
			timer := time.NewTimer(step.After)
			defer timer.Stop()

			select {
			case <-timer.C:
				s.Escalate(task, step.Target)
			case <-task.doneCh:
				return
			}
		}()
	}
}

// Escalate increases the priority of the given [Task] to the target priority if
// the target priority is higher than the current priority.
func (s *Scheduler[T]) Escalate(task *Task[T], target Priority) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Task has already been dequeued, or does not exist in the heap.
	if task.index < 0 {
		return
	}

	task.mu.Lock()

	// No escalation needed.
	if task.Priority.priority >= target.priority {
		return
	}

	old := task.Priority
	task.Priority = target
	task.mu.Unlock()

	select {
	case <-task.escalatedCh:
	default:
		close(task.escalatedCh)
	}

	// Reorder the heap since the task's priority has changed.
	heap.Fix(s, task.index)

	if s.metrics != nil {
		s.metrics.OnEscalate(task, old, target)
	}

	s.notify()
}

func (s *Scheduler[T]) notify() {
	select {
	case s.notifyCh <- struct{}{}:
	default:
	}
}

// TasksIterator defines an iterator over scheduled tasks.
type TasksIterator[T any] iter.Seq[*Task[T]]

// Tasks returns an iterator over scheduled tasks. The iterator yields the
// highest priority [Task] from the scheduler, blocking until tasks are
// available or the context is cancelled.
func (s *Scheduler[T]) Tasks(ctx context.Context) TasksIterator[T] {
	return func(yield func(*Task[T]) bool) {
		for {
			next := s.Next(ctx)
			if next == nil {
				return
			}

			if !yield(next) {
				return
			}
		}
	}
}

// Next removes and returns the highest priority [Task] from the scheduler. If
// the scheduler has no tasks, Next blocks until a [Task] is available or the
// context is cancelled.
func (s *Scheduler[T]) Next(ctx context.Context) *Task[T] {
	for {
		s.mu.Lock()
		if len(s.tasks) > 0 {
			task := heap.Pop(s).(*Task[T])
			close(task.doneCh)
			s.lastDequeueAt = time.Now()
			s.mu.Unlock()

			if s.metrics != nil {
				s.metrics.OnUnschedule(task)
			}

			return task
		}
		s.mu.Unlock()

		select {
		case <-s.notifyCh:
		case <-ctx.Done():
			return nil
		}
	}
}

// UnscheduleTask unschedules the given [Task] from the scheduler. This is
// useful for cancelling a [Task] that is no longer needed, or for consuming a
// [Task] immediately without going through the normal process.
func (s *Scheduler[T]) UnscheduleTask(task *Task[T]) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.index < 0 {
		return
	}

	heap.Remove(s, task.index)
	close(task.doneCh)
	s.lastDequeueAt = time.Now()

	if s.metrics != nil {
		s.metrics.OnUnschedule(task)
	}
}

// Len returns the number of tasks currently scheduled.
func (s *Scheduler[T]) Len() int {
	return len(s.tasks)
}

// Less determines if the [Task] at i less than the [Task] at j. This is used by
// the heap to order tasks. It is without side effects and may be called
// directly.
func (s *Scheduler[T]) Less(i, j int) bool {
	a, b := s.tasks[i], s.tasks[j]

	// Duration of time since last dequeue. This is used to enforce fairness.
	sinceLastDequeue := time.Since(s.lastDequeueAt)

	// If fairness window is set, and the last dequeue was longer ago than the
	// fairness window, prioritize by enqueue time to ensure fairness.
	if s.fairnessWindow > 0 && !s.lastDequeueAt.IsZero() && sinceLastDequeue > s.fairnessWindow {
		return a.enqueueAt.Before(b.enqueueAt)
	}

	aPriority, bPriority := a.GetPriority(), b.GetPriority()
	if aPriority.priority != bPriority.priority {
		return aPriority.priority > bPriority.priority
	}
	return a.seqNo < b.seqNo
}

// Swap swaps the tasks at indices i and j. This is used by the heap to reorder
// tasks. It should not be called directly.
func (s *Scheduler[T]) Swap(i, j int) {
	s.tasks[i], s.tasks[j] = s.tasks[j], s.tasks[i]
	s.tasks[i].index = i
	s.tasks[j].index = j
}

// Push adds a new [Task] to the scheduler. This is used by the heap to add new
// tasks. It should not be called directly.
func (s *Scheduler[T]) Push(x any) {
	task := x.(*Task[T])
	task.index = len(s.tasks)
	s.tasks = append(s.tasks, task)
}

// Pop removes and returns the highest priority [Task] from the scheduler. This
// is used by the heap to remove tasks. It should not be called directly.
func (s *Scheduler[T]) Pop() any {
	old := s.tasks
	n := len(old)
	task := old[n-1]
	old[n-1] = nil  // avoid memory leak
	task.index = -1 // for safety
	s.tasks = old[0 : n-1]
	return task
}

// Peek returns the highest priority [Task] from the scheduler without
// removing it. If the scheduler has no tasks, Peek returns nil.
func (s *Scheduler[T]) Peek() *Task[T] {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.tasks) == 0 {
		return nil
	}
	return s.tasks[0]
}
