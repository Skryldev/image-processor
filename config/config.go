package config

import (
	"errors"
	"time"
)

// StorageBackend selects the storage adapter.
type StorageBackend string

const (
	StorageLocal StorageBackend = "local"
	StorageS3    StorageBackend = "s3"
)

// Config is the top-level configuration struct.  All fields have safe defaults
// so callers can start with Config{} and override only what they need.
type Config struct {
	// Worker pool controls.
	WorkerCount   int // default: runtime.NumCPU()
	QueueSize     int // max queued jobs before backpressure; default: 256
	JobTimeout    time.Duration

	// Retry.
	MaxRetries int
	RetryDelay time.Duration

	// Default encode options applied when a pipeline step does not override.
	DefaultQuality int // 1-100; default 85
	DefaultFormat  string

	// Streaming / memory limits.
	MaxImageBytes int64 // 0 = no limit
	ChunkSize     int   // streaming chunk size in bytes; default 32 KiB

	// Storage.
	Storage StorageBackend
	Local   LocalConfig
	S3      S3Config

	// Adaptive compression.
	AdaptiveCompression AdaptiveConfig

	// Logging / metrics.
	LogLevel string // "debug", "info", "warn", "error"
}

// LocalConfig configures the local filesystem storage adapter.
type LocalConfig struct {
	RootDir     string
	Permissions uint32 // default 0644
}

// S3Config configures the AWS S3 storage adapter.
type S3Config struct {
	Bucket          string
	Region          string
	Endpoint        string // optional custom endpoint (MinIO, etc.)
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

// AdaptiveConfig controls the adaptive compression algorithm.
type AdaptiveConfig struct {
	Enabled         bool
	TargetSizeBytes int64 // desired maximum output size
	MinQuality      int   // floor to prevent over-compression; default 30
	MaxQuality      int   // ceiling; default 95
	StepSize        int   // quality decrement per iteration; default 5
}

// Default returns a Config populated with sensible production defaults.
func Default() Config {
	return Config{
		WorkerCount:    0, // resolved at runtime to NumCPU
		QueueSize:      256,
		JobTimeout:     30 * time.Second,
		MaxRetries:     3,
		RetryDelay:     200 * time.Millisecond,
		DefaultQuality: 85,
		ChunkSize:      32 * 1024,
		Storage:        StorageLocal,
		AdaptiveCompression: AdaptiveConfig{
			MinQuality: 30,
			MaxQuality: 95,
			StepSize:   5,
		},
		LogLevel: "info",
	}
}

// Validate returns an error if the configuration is inconsistent.
func Validate(c Config) error {
	if c.DefaultQuality < 1 || c.DefaultQuality > 100 {
		return errors.New("config: DefaultQuality must be between 1 and 100")
	}
	if c.ChunkSize <= 0 {
		return errors.New("config: ChunkSize must be positive")
	}
	if c.AdaptiveCompression.Enabled {
		if c.AdaptiveCompression.MinQuality >= c.AdaptiveCompression.MaxQuality {
			return errors.New("config: AdaptiveCompression.MinQuality must be less than MaxQuality")
		}
	}
	return nil
}