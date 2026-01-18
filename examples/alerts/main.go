// Alerts simulates a notification system that processes alerts with different
// priorities and automatic escalation.
package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/tomasbasham/escalsched"
)

const (
	numWorkers       = 4
	scheduleInterval = 100 * time.Millisecond
	processTime      = 500 * time.Millisecond
)

var (
	backgroundPolicy = escalsched.EscalationPolicy{
		{After: 1 * time.Second, Target: escalsched.Priorities.High},
	}
	policy = escalsched.EscalationPolicy{
		{After: 500 * time.Millisecond, Target: escalsched.Priorities.High},
	}
)

type stats struct {
	total     int
	escalated int
}

func main() {
	s := escalsched.New(
		escalsched.WithFairnessWindow[string](2*time.Second),
		escalsched.WithMetricsHook(&MyMetrics{}),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Simulate alerts being added to the scheduler in the background.
	go scheduleAlerts(ctx, s)

	// Use WaitGroup to know when all workers are done.
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	fmt.Printf("\nStarting %d workers (processing takes ~%s per alert)...\n\n", numWorkers, processTime)

	worker := func(stats *stats) {
		defer wg.Done()
		for alert := range s.Tasks(ctx) {
			stats.total++
			if alert.Escalated() {
				stats.escalated++
			}
			time.Sleep(processTime) // slow processing to allow queue buildup.
		}
	}

	// Track each worker's stats, and process alerts in the background.
	workerStats := make([]*stats, numWorkers)
	for i := range numWorkers {
		workerStats[i] = &stats{}
		go worker(workerStats[i])
	}

	<-ctx.Done()
	fmt.Printf("\n>>> Draining...\n\n")

	wg.Wait() // wait for all workers to finish.

	fmt.Println("\n--- Worker Stats ---")
	for i, stats := range workerStats {
		fmt.Printf("Worker %d: processed %d alerts, escalated %d\n", i, stats.total, stats.escalated)
	}
}

// Simulate scheduling alers with different priorities and escalation policies.
func scheduleAlerts(ctx context.Context, s *escalsched.Scheduler[string]) {
	ticker := time.NewTicker(scheduleInterval)
	defer ticker.Stop()

	i := 0
	for {
		alertName := fmt.Sprintf("alert-%d", i)

		select {
		case <-ticker.C:
			p := rand.Float64()
			if p < 0.02 {
				// Very low priority notifications without escalation. e.g. system
				// notifications.
				s.Schedule(alertName, escalsched.Priorities.VeryLow)
			} else if p < 0.1 {
				// Background notifications with escalation policy.
				s.ScheduleWithPolicy(alertName, escalsched.Priorities.Low, backgroundPolicy)
			} else if p < 0.3 {
				// Normal priority notifications with automatic escalation.
				s.ScheduleWithPolicy(alertName, escalsched.Priorities.Normal, policy)
			} else {
				// High priority notifications without escalation. e.g. critical alerts.
				s.Schedule(alertName, escalsched.Priorities.High)
			}
			i++
		case <-ctx.Done():
			return
		}
	}
}

// MyMetrics implements escalsched.MetricsHook to log scheduler events.
type MyMetrics struct {
	counter atomic.Int64
}

func (m *MyMetrics) OnSchedule(task *escalsched.Task[string]) {
	// Log every 10th scheduled task to avoid excessive output.
	if m.counter.Add(1)%10 == 0 {
		fmt.Printf("[metrics] Scheduled %s\n", task.Value)
	}
}

func (m *MyMetrics) OnUnschedule(task *escalsched.Task[string]) {
	fmt.Printf("[metrics] Unscheduled %s\n", task.Value)
}

func (m *MyMetrics) OnEscalate(task *escalsched.Task[string], from, to escalsched.Priority) {
	fmt.Printf("[metrics] Escalated %s from %s -> %s\n", task.Value, from, to)
}
