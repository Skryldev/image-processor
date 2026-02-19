package imageprocessor

import (
	"context"
	"io"

	"github.com/Skryldev/image-processor/adapters/decoder"
	"github.com/Skryldev/image-processor/adapters/encoder"
	"github.com/Skryldev/image-processor/config"
	"github.com/Skryldev/image-processor/core"
	"github.com/Skryldev/image-processor/pipeline"
)

// Re-export Format constants for convenience.
const (
	JPEG = core.FormatJPEG
	PNG  = core.FormatPNG
	WebP = core.FormatWebP
)

// DefaultConfig returns a sensible production configuration.
func DefaultConfig() config.Config { return config.Default() }

// Processor is the primary entry point.
type Processor struct {
	inner *core.Processor
	reg   *core.DefaultRegistry
}

// New creates a fully wired Processor with default JPEG, PNG, and WebP codecs
// registered.  Pass a custom config.Config to override defaults.
func New(cfg config.Config) *Processor {
	reg := core.NewRegistry()
	// Register built-in codecs.
	reg.RegisterDecoder(core.FormatJPEG, decoder.NewJPEG())
	reg.RegisterDecoder(core.FormatPNG, decoder.NewPNG())
	reg.RegisterDecoder(core.FormatWebP, decoder.NewWebP())
	reg.RegisterEncoder(core.FormatJPEG, encoder.NewJPEG(cfg.DefaultQuality))
	reg.RegisterEncoder(core.FormatPNG, encoder.NewPNG())
	reg.RegisterEncoder(core.FormatWebP, encoder.NewWebP(cfg.DefaultQuality))

	inner := core.New(cfg, reg)
	return &Processor{inner: inner, reg: reg}
}

// SetLogger attaches a structured logger.
func (p *Processor) SetLogger(l core.Logger) { p.inner.SetLogger(l) }

// SetMetrics attaches a metrics collector.
func (p *Processor) SetMetrics(m core.MetricsCollector) { p.inner.SetMetrics(m) }

// AddHook registers an observer for pipeline step events.
func (p *Processor) AddHook(h core.Hook) { p.inner.AddHook(h) }

// RegisterDecoder registers a custom decoder for the given format.
func (p *Processor) RegisterDecoder(f core.Format, d core.Decoder) { p.reg.RegisterDecoder(f, d) }

// RegisterEncoder registers a custom encoder for the given format.
func (p *Processor) RegisterEncoder(f core.Format, e core.Encoder) { p.reg.RegisterEncoder(f, e) }

// Start starts the background worker pool.
func (p *Processor) Start() { p.inner.Start() }

// Stop drains and shuts down the worker pool.
func (p *Processor) Stop() { p.inner.Stop() }

// Process executes the provided steps synchronously and returns the result.
func (p *Processor) Process(ctx context.Context, src core.Source, steps ...core.Step) (*core.ProcessingResult, error) {
	return p.inner.Process(ctx, src, steps...)
}

// Batch runs the same steps on multiple sources concurrently.
func (p *Processor) Batch(ctx context.Context, sources []core.Source, steps ...core.Step) ([]*core.ProcessingResult, []error) {
	return p.inner.Batch(ctx, sources, steps...)
}

// ProcessVariants runs base steps and then produces named variants in parallel.
func (p *Processor) ProcessVariants(
	ctx context.Context,
	src core.Source,
	baseSteps []core.Step,
	variants []core.VariantDefinition,
) (*core.ProcessingResult, error) {
	return p.inner.ProcessVariants(ctx, src, baseSteps, variants)
}

// Submit enqueues an async job for the worker pool.
func (p *Processor) Submit(job core.Job) error { return p.inner.Submit(job) }

// NewPipeline creates a reusable, standalone pipeline.
func (p *Processor) NewPipeline(steps ...core.Step) *pipeline.Pipeline {
	pl := pipeline.New()
	pl.Use(steps...)
	return pl
}

// Stats returns lightweight processing statistics.
func (p *Processor) Stats() (processed, errors int64) {
	return p.inner.ProcessedCount(), p.inner.ErrorCount()
}

// ── Source constructors ────────────────────────────────────────────────────────

// FromReader creates a Source from an io.Reader.
func FromReader(r io.Reader) core.Source { return core.Source{Reader: r, Size: -1} }

// FromReaderWithMeta creates a Source with known size and content-type hints.
func FromReaderWithMeta(r io.Reader, size int64, contentType, name string) core.Source {
	return core.Source{Reader: r, Size: size, ContentType: contentType, Name: name}
}

// ── Step constructors ─────────────────────────────────────────────────────────

// Decode returns a step that decodes img.Data → img.Image.
func Decode() core.Step {
	// Lazy singleton registry for the decode step; the full registry is
	// provided by the Processor.  For standalone use, pass the registry
	// explicitly via DecodeWith.
	return &pipeline.DecodeStep{} // registry injected at Process time via wrapper
}

// DecodeWith returns a decode step bound to the given registry.
func DecodeWith(reg core.Registry) core.Step { return &pipeline.DecodeStep{Registry: reg} }

// Resize returns a resize step.  Pass 0 for one axis to preserve aspect ratio.
func Resize(width, height int) core.Step { return &pipeline.ResizeStep{Width: width, Height: height} }

// Crop returns a crop step.
func Crop(x, y, width, height int) core.Step {
	return &pipeline.CropStep{X: x, Y: y, Width: width, Height: height}
}

// Thumbnail returns a square thumbnail step.
func Thumbnail(size int) core.Step { return &pipeline.ThumbnailStep{Size: size} }

// Quality stores the desired encode quality (1-100) for the next Encode step.
func Quality(q int) core.Step { return &pipeline.QualityStep{Quality: q} }

// ConvertFormat instructs subsequent steps to use the given output format.
func ConvertFormat(f core.Format) core.Step { return &pipeline.FormatStep{Format: f} }

// StripEXIF returns a step that removes EXIF metadata.
func StripEXIF() core.Step { return &pipeline.StripEXIFStep{} }

// Grayscale returns a step that converts the image to grayscale.
func Grayscale() core.Step { return &pipeline.GrayscaleStep{} }

// EncodeWith returns an encode step bound to the given registry and options.
func EncodeWith(reg core.Registry, opts core.EncodeOptions) core.Step {
	return &pipeline.EncodeStep{Registry: reg, BaseOptions: opts}
}

// Encode returns an encode step with default options.
// Prefer using the processor's Process method which auto-wires the registry.
func Encode() core.Step { return &pipeline.EncodeStep{} }

// AdaptiveCompress returns a step that iteratively reduces quality to hit a
// target size in bytes.
func AdaptiveCompress(reg core.Registry, targetBytes int64, minQ, maxQ int) core.Step {
	return &pipeline.AdaptiveCompressStep{
		Registry:        reg,
		TargetSizeBytes: targetBytes,
		MinQuality:      minQ,
		MaxQuality:      maxQ,
		StepSize:        5,
	}
}