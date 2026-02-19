package core

import (
	"context"
	"io"
)

// Decoder converts raw bytes / a reader into an in-memory ImageData.
// Implementations live in adapters/decoder/.
type Decoder interface {
	// Decode reads from r and returns a decoded ImageData.
	Decode(ctx context.Context, r io.Reader) (*ImageData, error)
	// CanDecode reports whether this decoder handles the given format hint.
	CanDecode(format Format) bool
}

// Encoder serialises an ImageData to bytes in a target format.
// Implementations live in adapters/encoder/.
type Encoder interface {
	Encode(ctx context.Context, img *ImageData, opts EncodeOptions) ([]byte, error)
	CanEncode(format Format) bool
}

// EncodeOptions carries format-specific encoding parameters.
type EncodeOptions struct {
	Quality    int  // 1-100; 0 = use encoder default
	Lossless   bool // WebP / PNG lossless mode
	StripEXIF  bool
	Interlaced bool // progressive JPEG / interlaced PNG
}

// StorageAdapter persists processed images and retrieves them later.
// Implementations live in adapters/storage/.
type StorageAdapter interface {
	Put(ctx context.Context, key StorageKey, r io.Reader, meta map[string]string) error
	Get(ctx context.Context, key StorageKey) (io.ReadCloser, error)
	Delete(ctx context.Context, key StorageKey) error
	Exists(ctx context.Context, key StorageKey) (bool, error)
}

// MetricsCollector receives performance observations from the pipeline.
type MetricsCollector interface {
	RecordProcessingTime(stepName string, d interface{ Seconds() float64 })
	RecordThroughput(bytes int64)
	RecordMemory(bytes int64)
	RecordError(stepName string, category string)
}

// Logger is a minimal structured logging interface.
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// Registry maps Format values to Decoder/Encoder implementations.
type Registry interface {
	DecoderFor(format Format) (Decoder, bool)
	EncoderFor(format Format) (Encoder, bool)
	RegisterDecoder(format Format, d Decoder)
	RegisterEncoder(format Format, e Encoder)
}