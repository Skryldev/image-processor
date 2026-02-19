package core

import (
	"context"
	"io"
	"time"
)

// Format identifies an image codec.
type Format string

const (
	FormatJPEG    Format = "jpeg"
	FormatPNG     Format = "png"
	FormatWebP    Format = "webp"
	FormatUnknown Format = "unknown"
)

// ColorSpace represents the image colour model.
type ColorSpace string

const (
	ColorSpaceRGB  ColorSpace = "rgb"
	ColorSpaceRGBA ColorSpace = "rgba"
	ColorSpaceCMYK ColorSpace = "cmyk"
	ColorSpaceGray ColorSpace = "gray"
)

// Metadata holds extracted image information without loading pixel data.
type Metadata struct {
	Width       int
	Height      int
	Format      Format
	ColorSpace  ColorSpace
	HasAlpha    bool
	SizeBytes   int64
	EXIF        map[string]string // nil when stripped or absent
	HasEXIF     bool
	Orientation int // EXIF orientation tag (1-8)
}

// ImageData is the in-memory representation passed through a pipeline.
// Data holds encoded bytes; Image holds the decoded pixel buffer when needed.
type ImageData struct {
	// Encoded bytes — non-nil when the image has been encoded or is raw input.
	Data   []byte
	Format Format

	// Decoded pixel buffer — populated lazily by decode steps only when needed.
	// Using image.Image keeps us CGO-free; libvips adapters can use unsafe pointers
	// wrapped in their own types and satisfy the Processor interface directly.
	Image interface{} // actual type: image.Image or vips.Image depending on backend

	// Metadata extracted during decode.
	Meta Metadata

	// Size of the original raw input for adaptive compression decisions.
	OriginalSize int64
}

// ProcessingResult is returned to the caller after the full pipeline completes.
type ProcessingResult struct {
	Primary  *ImageData
	Variants map[string]*ImageData // keyed by variant name

	// Observability.
	ProcessingTime time.Duration
	StepTimings    map[string]time.Duration
	MemoryUsedB    int64
}

// Source abstracts where raw bytes come from (reader, file path, URL, etc.).
type Source struct {
	Reader      io.Reader
	ContentType string // optional hint
	Name        string // optional logical name / filename
	Size        int64  // -1 if unknown
}

// Job encapsulates a single unit of work for the worker pool.
type Job struct {
	ID      string
	Ctx     context.Context //nolint:containedctx // intentional for async jobs
	Source  Source
	Steps   []Step
	Options JobOptions
	// Result channel; nil for fire-and-forget.
	ResultCh chan<- JobResult
}

// JobOptions controls per-job behaviour.
type JobOptions struct {
	MaxRetries  int
	RetryDelay  time.Duration
	VariantDefs []VariantDefinition
}

// VariantDefinition instructs the pipeline to produce a named output variant.
type VariantDefinition struct {
	Name  string
	Steps []Step
}

// JobResult wraps the outcome of an async job.
type JobResult struct {
	JobID  string
	Result *ProcessingResult
	Err    error
}

// Step is the fundamental pipeline building block.  Each Step transforms an
// *ImageData value and must be safe for concurrent use across goroutines.
type Step interface {
	Name() string
	Execute(ctx context.Context, img *ImageData) (*ImageData, error)
}

// Hook is an optional observer invoked around pipeline steps.
type Hook interface {
	BeforeStep(ctx context.Context, stepName string, img *ImageData)
	AfterStep(ctx context.Context, stepName string, img *ImageData, d time.Duration, err error)
}

// StorageKey uniquely identifies a stored image.
type StorageKey struct {
	Bucket string
	Path   string
}