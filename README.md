# escalsched [![test](https://github.com/tomasbasham/escalsched/actions/workflows/test.yaml/badge.svg?event=push)](https://github.com/tomasbasham/escalsched/actions/workflows/test.yaml)

A Go scheduler that implements priority-based task scheduling with time-based
escalation policies. Tasks are processed according to their priority levels,
whilst escalation policies ensure that lower-priority tasks don't experience
indefinite starvation by automatically promoting them when processing delays
exceed configured thresholds.

The scheduler provides fairness guarantees through an optional fairness window,
which temporarily suspends priority-based ordering to process tasks in FIFO
order when the scheduler has been idle, preventing permanent starvation of
lower-priority work.

## Prerequisites

You will need the following things properly installed on your computer:

- [Go](https://golang.org/): any one of the **three latest major**
  [releases](https://golang.org/doc/devel/release.html)

## Installation

With [Go module](https://go.dev/wiki/Modules) support (Go 1.11+), simply add the
following import

```go
import "github.com/tomasbasham/escalsched"
```

to your code, and then `go [build|run|test]` will automatically fetch the
necessary dependencies.

Otherwise, to install the `escalsched` module, run the following command:

```bash
go get -u github.com/tomasbasham/escalsched
```

## Usage

To use this module, create a scheduler instance and schedule tasks with
priorities. The scheduler processes tasks in priority order, with
higher-priority tasks being unscheduled before lower-priority ones.

### Basic Scheduling

```go
// Create a new scheduler for string tasks.
scheduler := escalsched.New[string]()

// Schedule tasks with different priorities.
scheduler.Schedule("low-priority-task", escalsched.Priorities.Low)
scheduler.Schedule("high-priority-task", escalsched.Priorities.High)
scheduler.Schedule("normal-priority-task", escalsched.Priorities.Normal)

// Retrieve the next task (blocks until available).
ctx := context.Background()
task := scheduler.Next(ctx)
fmt.Println(task.Value) // Outputs: "high-priority-task"
```

### Escalation Policies

Define escalation policies to automatically increase task priority after
specified durations:

<!-- pyml disable md013 -->
```go
scheduler := escalsched.New[string]()

// Define an escalation policy.
policy := escalsched.EscalationPolicy{
    {After: 100 * time.Millisecond, Target: escalsched.Priorities.High},
}

// Schedule a task with the policy.
task := scheduler.ScheduleWithPolicy("important-task", escalsched.Priorities.Low, policy)

// After 100ms, the task will automatically escalate to High priority if it has
// not yet been processed.
```
<!-- pyml enable md013 -->

### Manual Escalation

Tasks can be manually escalated at any time:

```go
scheduler := escalsched.New[string]()
task := scheduler.Schedule("task", escalsched.Priorities.Low)

// Manually escalate to high priority.
scheduler.Escalate(task, escalsched.Priorities.High)
```

### Fairness Window

Configure a fairness window to prevent starvation by temporarily processing
tasks in FIFO order:

```go
scheduler := escalsched.New[string](
    escalsched.WithFairnessWindow(5 * time.Second),
)

// If the scheduler is idle for 5 seconds, the next task will be unscheduled
// in FIFO order regardless of priority.
```

### Metrics Hooks

Monitor scheduler events by implementing the `MetricsHook` interface:

<!-- pyml disable md013 -->
```go
type MyMetrics struct{}

func (m *MyMetrics) OnSchedule(task *escalsched.Task[string]) {
    fmt.Printf("Scheduled: %s\n", task.Value)
}

func (m *MyMetrics) OnUnschedule(task *escalsched.Task[string]) {
    fmt.Printf("Unscheduled: %s\n", task.Value)
}

func (m *MyMetrics) OnEscalate(task *escalsched.Task[string], from, to escalsched.Priority) {
    fmt.Printf("Escalated %s from %s to %s\n", task.Value, from, to)
}

scheduler := escalsched.New[string](
    escalsched.WithMetricsHook(&MyMetrics{}),
)
```
<!-- pyml enable md013 -->

### Concurrent Workers

The scheduler is thread-safe and supports multiple concurrent workers:

```go
scheduler := escalsched.New[int]()

// Schedule tasks.
for i := range 100 {
    scheduler.Schedule(i, escalsched.Priorities.Normal)
}

numWorkers := 4
ctx := context.Background()

// Use WaitGroup to know when all workers are done.
var wg sync.WaitGroup
wg.Add(numWorkers)

worker := func(id int) {
    defer wg.Done()
    for task := range scheduler.Tasks(ctx)
        fmt.Printf("Worker %d: Processing %d\n", w, task.Value)
    }
}

// Start workers.
for i := range numWorkers {
    go worker(i + 1)
}

wg.Wait()
```

### Unscheduling Tasks

Remove tasks from the scheduler without processing them:

```go
scheduler := escalsched.New[string]()
task := scheduler.Schedule("task", escalsched.Priorities.Normal)

// Remove the task from the scheduler.
scheduler.UnscheduleTask(task)
```

## License

This project is licensed under the [MIT License](LICENSE).
