// Escalation simulates a task scheduling system that processes tasks with
// different priorities and automatic escalation.
package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/tomasbasham/escalsched"
)

const (
	numWorkers      = 2
	numInitialTasks = 20 // Pre-fill queue to show priority ordering.
	burstInterval   = 3 * time.Second
	burstSize       = 10
	processTime     = 500 * time.Millisecond
)

var (
	policy = escalsched.EscalationPolicy{
		{After: 1 * time.Second, Target: escalsched.Priorities.Normal},
		{After: 2 * time.Second, Target: escalsched.Priorities.High},
	}
)

func main() {
	s := escalsched.New[string]()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Pre-fill queue with mixed priority tasks.
	fmt.Println("Scheduling initial tasks...")
	for i := 1; i <= numInitialTasks; i++ {
		priority := choosePriority(i)
		taskName := fmt.Sprintf("Task-%d (%s)", i, priority)

		if priority == escalsched.Priorities.Low {
			s.ScheduleWithPolicy(taskName, priority, policy)
			fmt.Printf("  Scheduled %s with escalation policy\n", taskName)
		} else {
			s.Schedule(taskName, priority)
			fmt.Printf("  Scheduled %s\n", taskName)
		}
	}

	// Continue scheduling new tasks occasionally.
	go scheduleTasks(ctx, s)

	// Use WaitGroup to know when all workers are done.
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	fmt.Printf("\nStarting %d workers (processing takes ~%s per task)...\n\n", numWorkers, processTime)

	worker := func(id int) {
		defer wg.Done()
		for task := range s.Tasks(ctx) {
			fmt.Printf("Worker %d: Processing %s (escalated %t)\n", id, task.Value, task.Escalated())
			time.Sleep(processTime) // slow processing to allow queue buildup.
		}
	}

	// Start workers.
	for i := range numWorkers {
		go worker(i + 1)
	}

	<-ctx.Done()
	fmt.Printf("\n>>> Draining...\n\n")

	wg.Wait() // wait for all workers to finish.
}

// Simulate scheduling bursts of new tasks with different priorities and
// escalation policies.
func scheduleTasks(ctx context.Context, s *escalsched.Scheduler[string]) {
	ticker := time.NewTicker(burstInterval)
	defer ticker.Stop()

	i := numInitialTasks + 1
	for {
		select {
		case <-ticker.C:
			fmt.Printf("\n>>> Burst: Scheduling 10 new tasks\n")

			// Schedule a burst of tasks to create backlog.
			for range burstSize {
				priority := []escalsched.Priority{
					escalsched.Priorities.Normal,
					escalsched.Priorities.Low,
				}[rand.IntN(2)]

				taskName := fmt.Sprintf("Task-%d (%s)", i, priority)

				if priority == escalsched.Priorities.Low {
					s.ScheduleWithPolicy(taskName, priority, policy)
				} else {
					s.Schedule(taskName, priority)
				}
				i++
			}
		case <-ctx.Done():
			return
		}
	}
}

func choosePriority(i int) escalsched.Priority {
	switch {
	case i%15 == 0:
		return escalsched.Priorities.High
	case i%5 == 0:
		return escalsched.Priorities.Normal
	default:
		return escalsched.Priorities.Low
	}
}
