package escalsched_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/tomasbasham/escalsched"
)

type task struct {
	value    string
	priority escalsched.Priority
}

func TestScheduler_Schedule(t *testing.T) {
	t.Parallel()

	t.Run("basic operations", func(t *testing.T) {
		tests := map[string]struct {
			tasks []task
			want  []string
		}{
			"tasks are unscheduled in priority order": {
				tasks: []task{
					{"low", escalsched.Priorities.Low},
					{"high", escalsched.Priorities.High},
					{"medium", escalsched.Priorities.Normal},
				},
				want: []string{"high", "medium", "low"},
			},
			"tasks with same priority maintain FIFO order": {
				tasks: []task{
					{"first", escalsched.Priorities.Normal},
					{"second", escalsched.Priorities.Normal},
					{"third", escalsched.Priorities.Normal},
				},
				want: []string{"first", "second", "third"},
			},
		}
		for name, tt := range tests {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				s := escalsched.New[string]()
				for _, task := range tt.tasks {
					s.Schedule(task.value, task.priority)
				}

				var got []string
				for range tt.tasks {
					task := s.Next(context.Background())
					got = append(got, task.Value)
				}

				if !slices.Equal(got, tt.want) {
					t.Errorf("mismatch:\n  got:  %#v\n  want: %#v", got, tt.want)
				}
			})
		}
	})
}

func TestScheduler_ScheduleWithPolicy(t *testing.T) {
	t.Parallel()

	t.Run("timed escalation", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			policy := escalsched.EscalationPolicy{
				{After: 100 * time.Millisecond, Target: escalsched.Priorities.High},
			}

			s := escalsched.New[string]()
			s.ScheduleWithPolicy("test", escalsched.Priorities.Low, policy)

			// Ensure the task is scheduled before waiting.
			task := s.Peek()
			if task.GetPriority() != escalsched.Priorities.Low {
				t.Fatalf("expected initial priority: %q, got: %q", escalsched.Priorities.Low, task.Priority)
			}

			// Wait for escalation to occur.
			time.Sleep(150 * time.Millisecond)

			// Unschedule after escalation should be high priority.
			task = s.Next(context.Background())

			got, want := task.GetPriority(), escalsched.Priorities.High
			if got != want {
				t.Errorf("mismatch:\n  got:  %q\n  want: %q", got, want)
			}
			if !task.Escalated() {
				t.Error("expected task to be marked as escalated")
			}
		})
	})

	t.Run("immediate escalation", func(t *testing.T) {
		s := escalsched.New[string]()
		task := s.Schedule("low-task", escalsched.Priorities.Low)

		// Immediately escalate the task to high priority.
		s.Escalate(task, escalsched.Priorities.High)

		// Unschedule after escalation should be high priority.
		task = s.Next(context.Background())

		got, want := task.GetPriority(), escalsched.Priorities.High
		if got != want {
			t.Errorf("mismatch:\n  got:  %q\n  want: %q", got, want)
		}
		if !task.Escalated() {
			t.Error("expected task to be marked as escalated")
		}
	})
}

func TestScheduler_Peek(t *testing.T) {
	t.Parallel()

	s := escalsched.New[string]()
	s.Schedule("first", escalsched.Priorities.Normal)
	s.Schedule("second", escalsched.Priorities.High)

	task := s.Peek()

	got, want := task.Value, "second"
	if got != want {
		t.Errorf("mismatch:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestScheduler_ConcurrentWorkers(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		const numWorkers = 4
		const numTasks = 50

		var wg sync.WaitGroup
		wg.Add(numWorkers)

		processed := make([]int, numWorkers)
		escalated := make([]int, numWorkers)

		s := escalsched.New[int]()

		// Escalation policy: escalate to high priority after short delay.
		policy := escalsched.EscalationPolicy{
			{After: 10 * time.Millisecond, Target: escalsched.Priorities.High},
		}

		// Schedule tasks with policy.
		for i := range numTasks {
			s.ScheduleWithPolicy(i, escalsched.Priorities.Low, policy)
		}

		// Allow some time for escalations to occur.
		time.Sleep(30 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		for w := range numWorkers {
			go func(id int) {
				defer wg.Done()
				for task := range s.Tasks(ctx) {
					processed[id]++
					if task.Escalated() {
						escalated[id]++
					}

					// Last worker to reach numTasks triggers cancellation.
					if processed[id] >= numTasks/numWorkers+1 {
						cancel()
					}
				}
			}(w)
		}

		wg.Wait()

		totalProcessed, totalEscalated := 0, 0
		for i := range numWorkers {
			totalProcessed += processed[i]
			totalEscalated += escalated[i]
		}

		if totalProcessed != numTasks {
			t.Errorf("expected %d tasks processed, got: %d", numTasks, totalProcessed)
		}
		if totalEscalated == 0 {
			t.Error("expected some tasks to be escalated")
		}
	})
}

func BenchmarkScheduler_Throughput(b *testing.B) {
	for numWorkers := 1; numWorkers <= 5; numWorkers++ {
		b.Run(fmt.Sprintf("%d_workers", numWorkers), func(b *testing.B) {
			s := escalsched.New[int]()

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			var wg sync.WaitGroup
			wg.Add(numWorkers)

			worker := func() {
				defer wg.Done()
				for task := range s.Tasks(ctx) {
					_ = task.Value // simulate processing.
				}
			}

			// Reset benchmark timer after setup. This ensures only the following
			// operations are measured.
			b.ReportAllocs()
			b.ResetTimer()

			for range numWorkers {
				go worker()
			}

			for i := range b.N {
				s.Schedule(i, escalsched.Priorities.Normal)
			}

			// Explicitly cancel context to stop workers and eliminate the timeout
			// wait from the benchmark.
			cancel()
			wg.Wait()
		})
	}
}

func BenchmarkScheduler_Escalation(b *testing.B) {
	for numWorkers := 1; numWorkers <= 5; numWorkers++ {
		b.Run(fmt.Sprintf("%d_workers", numWorkers), func(b *testing.B) {
			s := escalsched.New[int]()

			// Escalation policy: small but realistic delay for benchmark.
			policy := escalsched.EscalationPolicy{
				{After: 5 * time.Millisecond, Target: escalsched.Priorities.High},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			var wg sync.WaitGroup
			wg.Add(numWorkers)

			worker := func() {
				defer wg.Done()
				for task := range s.Tasks(ctx) {
					_ = task.Value // simulate processing.
				}
			}

			// Reset benchmark timer after setup. This ensures only the following
			// operations are measured.
			b.ReportAllocs()
			b.ResetTimer()

			for range numWorkers {
				go worker()
			}

			for i := range b.N {
				s.ScheduleWithPolicy(i, escalsched.Priorities.Normal, policy)
			}

			// Explicitly cancel context to stop workers and eliminate the timeout
			// wait from the benchmark.
			cancel()
			wg.Wait()
		})
	}
}
