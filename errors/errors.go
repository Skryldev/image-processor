package errors

import (
	"errors"
	"fmt"
)

// Category classifies error types for targeted handling and monitoring.
type Category string

const (
	CategoryDecode    Category = "decode"
	CategoryEncode    Category = "encode"
	CategoryPipeline  Category = "pipeline"
	CategoryStorage   Category = "storage"
	CategoryConfig    Category = "config"
	CategoryTransient Category = "transient"
	CategoryInput     Category = "input"
)

// ProcessingError is the structured error type used throughout the module.
type ProcessingError struct {
	Category Category
	Op       string // operation name
	Err      error
	Retryable bool
}

func (e *ProcessingError) Error() string {
	return fmt.Sprintf("[%s] %s: %v", e.Category, e.Op, e.Err)
}

func (e *ProcessingError) Unwrap() error { return e.Err }

// New creates a non-retryable ProcessingError.
func New(category Category, op string, err error) *ProcessingError {
	return &ProcessingError{Category: category, Op: op, Err: err}
}

// Transient creates a retryable ProcessingError.
func Transient(op string, err error) *ProcessingError {
	return &ProcessingError{Category: CategoryTransient, Op: op, Err: err, Retryable: true}
}

// Wrap wraps an existing error with context.
func Wrap(category Category, op string, err error) error {
	if err == nil {
		return nil
	}
	return New(category, op, err)
}

// IsRetryable reports whether err represents a transient failure.
func IsRetryable(err error) bool {
	var pe *ProcessingError
	if errors.As(err, &pe) {
		return pe.Retryable
	}
	return false
}

// IsCategory reports whether err belongs to the given category.
func IsCategory(err error, cat Category) bool {
	var pe *ProcessingError
	if errors.As(err, &pe) {
		return pe.Category == cat
	}
	return false
}

// Sentinel errors for common failure modes.
var (
	ErrUnsupportedFormat  = errors.New("unsupported image format")
	ErrInvalidDimensions  = errors.New("invalid dimensions")
	ErrEmptyInput         = errors.New("empty input")
	ErrContextCanceled    = errors.New("context canceled")
	ErrWorkerPoolFull     = errors.New("worker pool queue full")
	ErrStorageUnavailable = errors.New("storage unavailable")
)