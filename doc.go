// Package escalsched implements a priority-based task scheduler with time-based
// escalation policies.
//
// Tasks are processed according to their priority levels, whilst escalation
// policies ensure that lower-priority tasks don't experience indefinite
// starvation by automatically promoting them when processing delays exceed
// configured thresholds.
//
// The scheduler provides fairness guarantees through an optional fairness
// window, which temporarily suspends priority-based ordering to process tasks
// in FIFO order when the scheduler has been idle, preventing permanent
// starvation of lower-priority work.
package escalsched
