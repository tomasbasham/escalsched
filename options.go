package escalsched

import "time"

// Options holds configuration options for the [Scheduler].
type Options[T any] struct {
	FairnessWindow time.Duration
	Metrics        MetricsHook[T]
}

// Option is a function that configures [Options].
type Option[T any] func(*Options[T])

// WithFairnessWindow sets the fairness window duration for the [Scheduler].
func WithFairnessWindow[T any](d time.Duration) Option[T] {
	return func(o *Options[T]) {
		o.FairnessWindow = d
	}
}

// WithMetricsHook sets the metrics hook for the [Scheduler].
func WithMetricsHook[T any](hook MetricsHook[T]) Option[T] {
	return func(o *Options[T]) {
		o.Metrics = hook
	}
}
