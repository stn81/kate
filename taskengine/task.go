package taskengine

import "context"

// TaskFunc define the task func type
type TaskFunc func(ctx context.Context)

// Run implements the Task interface
func (f TaskFunc) Run(ctx context.Context) {
	f(ctx)
}

// Task define the task interface
type Task interface {
	Run(ctx context.Context)
}
