package escalsched

import "time"

// Task represents an item to be scheduled. It holds the value and its priority.
type Task[T any] struct {
	Value     T
	Priority  Priority
	enqueueAt time.Time
	index     int

	// The seqNo is used to maitain the order of tasks with the same priority. It
	// is incremented each time a new [Task] is scheduled and is immutable. In
	// theory it could overflow, but even if 1 million tasks are enqueued per
	// second, it would take over 292434 years to overflow.
	seqNo int64

	escalatedCh chan struct{} // closed when escalated.
	doneCh      chan struct{} // closed when dequeued.
}

// Escalated returns true if the [Task] has been escalated in priority.
func (e *Task[T]) Escalated() bool {
	select {
	case <-e.escalatedCh:
		return true
	default:
		return false
	}
}
