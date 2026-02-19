package core

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Skryldev/image-processor/utils"
	"github.com/Skryldev/image-processor/config"
	apperrors "github.com/Skryldev/image-processor/errors"
)

// PipelineRunner is a minimal interface over pipeline.Pipeline so that core
// does not import the pipeline package (avoiding a circular dependency).
type PipelineRunner interface {
	Run(ctx context.Context, img *ImageData) (*ImageData, map[string]time.Duration, error)
	Clone() PipelineRunner
}

// Processor is the central orchestrator.  It is safe for concurrent use.
type Processor struct {
	cfg      config.Config
	registry Registry
	hooks    []Hook
	logger   Logger
	metrics  MetricsCollector

	// Worker pool.
	jobQueue chan Job
	wg       sync.WaitGroup
	once     sync.Once
	shutdown chan struct{}

	// Atomic counters for lightweight internal metrics.
	processedCount int64
	errorCount     int64
}

// New creates a Processor with the given config.  Call Start() before
// submitting jobs; call Stop() when done.
func New(cfg config.Config, reg Registry) *Processor {
	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}
	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 256
	}
	return &Processor{
		cfg:      cfg,
		registry: reg,
		jobQueue: make(chan Job, queueSize),
		shutdown: make(chan struct{}),
	}
}

// SetLogger attaches a structured logger.
func (p *Processor) SetLogger(l Logger) { p.logger = l }

// SetMetrics attaches a metrics collector.
func (p *Processor) SetMetrics(m MetricsCollector) { p.metrics = m }

// AddHook registers a pipeline hook.
func (p *Processor) AddHook(h Hook) { p.hooks = append(p.hooks, h) }

// Registry returns the underlying registry so callers can register
// encoders/decoders after construction.
func (p *Processor) Registry() Registry { return p.registry }

// Start launches the worker pool.  It is idempotent.
func (p *Processor) Start() {
	p.once.Do(func() {
		workerCount := p.cfg.WorkerCount
		if workerCount <= 0 {
			workerCount = runtime.NumCPU()
		}
		for i := 0; i < workerCount; i++ {
			p.wg.Add(1)
			go p.worker()
		}
	})
}

// Stop drains the queue and shuts down all workers.
func (p *Processor) Stop() {
	close(p.shutdown)
	p.wg.Wait()
}

// Process is the primary synchronous API.  It reads from src, runs steps, and
// returns a ProcessingResult.
func (p *Processor) Process(ctx context.Context, src Source, steps ...Step) (*ProcessingResult, error) {
	if len(steps) == 0 {
		return nil, apperrors.New(apperrors.CategoryPipeline, "process", apperrors.ErrEmptyInput)
	}

	start := time.Now()

	// --- 1. Drain source into memory (respecting max size limit) -------------
	var limitedR = src.Reader
	if p.cfg.MaxImageBytes > 0 {
		limitedR = &utils.LimitedReader{R: src.Reader, Max: p.cfg.MaxImageBytes}
	}

	buf, err := utils.DrainReader(ctx, limitedR, p.cfg.ChunkSize)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryDecode, "process.drain", err)
	}
	rawBytes := utils.CloneBytes(buf.Bytes())
	utils.ReleaseBuffer(buf)

	// --- 2. Detect format ----------------------------------------------------
	format := Format(utils.DetectFormat(rawBytes))
	if src.ContentType != "" {
		format = contentTypeToFormat(src.ContentType)
	}

	img := &ImageData{
		Data:         rawBytes,
		Format:       format,
		OriginalSize: int64(len(rawBytes)),
	}

	// --- 3. Run steps --------------------------------------------------------
	timings := make(map[string]time.Duration, len(steps))
	current := img
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			atomic.AddInt64(&p.errorCount, 1)
			return nil, apperrors.Wrap(apperrors.CategoryPipeline, step.Name(), err)
		}
		p.notifyBefore(ctx, step.Name(), current)
		t := time.Now()
		next, stepErr := p.runWithRetry(ctx, step, current)
		elapsed := time.Since(t)
		timings[step.Name()] = elapsed
		p.notifyAfter(ctx, step.Name(), next, elapsed, stepErr)
		if stepErr != nil {
			atomic.AddInt64(&p.errorCount, 1)
			return nil, stepErr
		}
		current = next
	}

	atomic.AddInt64(&p.processedCount, 1)

	total := time.Since(start)
	return &ProcessingResult{
		Primary:        current,
		ProcessingTime: total,
		StepTimings:    timings,
	}, nil
}

// Submit enqueues an async job.  Returns ErrWorkerPoolFull if the queue is full.
func (p *Processor) Submit(job Job) error {
	select {
	case p.jobQueue <- job:
		return nil
	default:
		return apperrors.New(apperrors.CategoryPipeline, "submit", apperrors.ErrWorkerPoolFull)
	}
}

// Batch processes multiple sources concurrently (fan-out / fan-in).
func (p *Processor) Batch(ctx context.Context, sources []Source, steps ...Step) ([]*ProcessingResult, []error) {
	results := make([]*ProcessingResult, len(sources))
	errs := make([]error, len(sources))
	var wg sync.WaitGroup

	for i, src := range sources {
		wg.Add(1)
		go func(idx int, s Source) {
			defer wg.Done()
			r, e := p.Process(ctx, s, steps...)
			results[idx] = r
			errs[idx] = e
		}(i, src)
	}
	wg.Wait()
	return results, errs
}

// ProcessVariants runs each VariantDefinition against the decoded image in
// parallel and returns a ProcessingResult with a populated Variants map.
func (p *Processor) ProcessVariants(ctx context.Context, src Source, baseSteps []Step, variants []VariantDefinition) (*ProcessingResult, error) {
	// First run base steps.
	base, err := p.Process(ctx, src, baseSteps...)
	if err != nil {
		return nil, err
	}

	variantResults := make(map[string]*ImageData, len(variants))
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make([]error, 0)

	for _, v := range variants {
		wg.Add(1)
		go func(vd VariantDefinition) {
			defer wg.Done()
			// Clone the base ImageData so variant steps don't mutate each other.
			clone := *base.Primary
			result := &clone
			var stepErr error
			for _, step := range vd.Steps {
				result, stepErr = step.Execute(ctx, result)
				if stepErr != nil {
					mu.Lock()
					errs = append(errs, stepErr)
					mu.Unlock()
					return
				}
			}
			mu.Lock()
			variantResults[vd.Name] = result
			mu.Unlock()
		}(v)
	}
	wg.Wait()

	if len(errs) > 0 {
		return nil, errs[0]
	}
	base.Variants = variantResults
	return base, nil
}

// ── worker pool internals ──────────────────────────────────────────────────────

func (p *Processor) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.shutdown:
			return
		case job, ok := <-p.jobQueue:
			if !ok {
				return
			}
			p.processJob(job)
		}
	}
}

func (p *Processor) processJob(job Job) {
	ctx := job.Ctx
	timeout := p.cfg.JobTimeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	result, err := p.Process(ctx, job.Source, job.Steps...)
	if job.ResultCh != nil {
		job.ResultCh <- JobResult{JobID: job.ID, Result: result, Err: err}
	}
}

func (p *Processor) runWithRetry(ctx context.Context, step Step, img *ImageData) (*ImageData, error) {
	maxRetries := p.cfg.MaxRetries
	delay := p.cfg.RetryDelay

	var (
		result *ImageData
		err    error
	)
	for i := 0; i <= maxRetries; i++ {
		result, err = step.Execute(ctx, img)
		if err == nil || !apperrors.IsRetryable(err) {
			return result, err
		}
		if i < maxRetries {
			select {
			case <-ctx.Done():
				return nil, apperrors.Wrap(apperrors.CategoryPipeline, step.Name(), ctx.Err())
			case <-time.After(delay):
			}
		}
	}
	return result, err
}

func (p *Processor) notifyBefore(ctx context.Context, name string, img *ImageData) {
	for _, h := range p.hooks {
		h.BeforeStep(ctx, name, img)
	}
}

func (p *Processor) notifyAfter(ctx context.Context, name string, img *ImageData, d time.Duration, err error) {
	for _, h := range p.hooks {
		h.AfterStep(ctx, name, img, d, err)
	}
}

// contentTypeToFormat maps MIME types to Format values.
func contentTypeToFormat(ct string) Format {
	switch ct {
	case "image/jpeg", "image/jpg":
		return FormatJPEG
	case "image/png":
		return FormatPNG
	case "image/webp":
		return FormatWebP
	}
	return FormatUnknown
}

// ProcessedCount returns the total number of successfully processed images.
func (p *Processor) ProcessedCount() int64 { return atomic.LoadInt64(&p.processedCount) }

// ErrorCount returns the total number of processing errors.
func (p *Processor) ErrorCount() int64 { return atomic.LoadInt64(&p.errorCount) }
