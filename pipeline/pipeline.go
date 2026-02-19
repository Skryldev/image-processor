// Package pipeline wires steps together, runs hooks, and handles retries.
package pipeline

import (
	"context"
	"time"

	"github.com/Skryldev/image-processor/core"
	apperrors "github.com/Skryldev/image-processor/errors"
)

// Pipeline executes a sequence of Steps with hook and retry support.
type Pipeline struct {
	steps      []core.Step
	hooks      []core.Hook
	maxRetries int
	retryDelay time.Duration
}

// New returns an empty Pipeline.
func New() *Pipeline { return &Pipeline{} }

// Use appends a step to the pipeline.  Returns the same Pipeline for chaining.
func (p *Pipeline) Use(s ...core.Step) *Pipeline {
	p.steps = append(p.steps, s...)
	return p
}

// AddHook registers an observer.
func (p *Pipeline) AddHook(h core.Hook) *Pipeline {
	p.hooks = append(p.hooks, h)
	return p
}

// WithRetry sets the maximum retry count and delay for transient failures.
func (p *Pipeline) WithRetry(maxRetries int, delay time.Duration) *Pipeline {
	p.maxRetries = maxRetries
	p.retryDelay = delay
	return p
}

// Run executes the pipeline on img.  It returns the final ImageData and a map
// of per-step timing observations.
func (p *Pipeline) Run(ctx context.Context, img *core.ImageData) (*core.ImageData, map[string]time.Duration, error) {
	timings := make(map[string]time.Duration, len(p.steps))
	current := img

	for _, step := range p.steps {
		if err := ctx.Err(); err != nil {
			return nil, timings, apperrors.Wrap(apperrors.CategoryPipeline, step.Name(), err)
		}

		result, elapsed, err := p.runStep(ctx, step, current)
		timings[step.Name()] = elapsed
		if err != nil {
			return nil, timings, err
		}
		current = result
	}
	return current, timings, nil
}

// runStep executes a single step, calling hooks and retrying transient errors.
func (p *Pipeline) runStep(ctx context.Context, step core.Step, img *core.ImageData) (*core.ImageData, time.Duration, error) {
	p.callHooksBefore(ctx, step.Name(), img)

	var (
		result  *core.ImageData
		elapsed time.Duration
		err     error
	)

	attempts := p.maxRetries + 1
	for i := 0; i < attempts; i++ {
		start := time.Now()
		result, err = step.Execute(ctx, img)
		elapsed = time.Since(start)

		if err == nil {
			break
		}
		if !apperrors.IsRetryable(err) || i == attempts-1 {
			break
		}
		// Wait before retrying.
		select {
		case <-ctx.Done():
			err = apperrors.Wrap(apperrors.CategoryPipeline, step.Name(), ctx.Err())
			goto done
		case <-time.After(p.retryDelay):
		}
	}

done:
	p.callHooksAfter(ctx, step.Name(), result, elapsed, err)
	return result, elapsed, err
}

func (p *Pipeline) callHooksBefore(ctx context.Context, name string, img *core.ImageData) {
	for _, h := range p.hooks {
		h.BeforeStep(ctx, name, img)
	}
}

func (p *Pipeline) callHooksAfter(ctx context.Context, name string, img *core.ImageData, d time.Duration, err error) {
	for _, h := range p.hooks {
		h.AfterStep(ctx, name, img, d, err)
	}
}

// Clone returns a shallow copy of the pipeline so templates can be reused
// safely across goroutines.
func (p *Pipeline) Clone() *Pipeline {
	cp := &Pipeline{
		steps:      make([]core.Step, len(p.steps)),
		hooks:      make([]core.Hook, len(p.hooks)),
		maxRetries: p.maxRetries,
		retryDelay: p.retryDelay,
	}
	copy(cp.steps, p.steps)
	copy(cp.hooks, p.hooks)
	return cp
}