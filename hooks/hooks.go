// Package hooks provides production-ready Hook and Logger implementations.
package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Skryldev/image-processor/core"
)

// ── Structured logger adapter ─────────────────────────────────────────────────

// SlogLogger wraps the standard library slog.Logger to satisfy core.Logger.
type SlogLogger struct {
	log *slog.Logger
}

// NewSlogLogger creates a logger backed by slog.
func NewSlogLogger(l *slog.Logger) *SlogLogger { return &SlogLogger{log: l} }

func (s *SlogLogger) Debug(msg string, fields ...interface{}) {
	s.log.Debug(msg, toAttrs(fields)...)
}
func (s *SlogLogger) Info(msg string, fields ...interface{}) {
	s.log.Info(msg, toAttrs(fields)...)
}
func (s *SlogLogger) Warn(msg string, fields ...interface{}) {
	s.log.Warn(msg, toAttrs(fields)...)
}
func (s *SlogLogger) Error(msg string, fields ...interface{}) {
	s.log.Error(msg, toAttrs(fields)...)
}

func toAttrs(fields []interface{}) []any { return fields }

// ── Logging hook ──────────────────────────────────────────────────────────────

// LoggingHook logs before/after each pipeline step.
type LoggingHook struct {
	logger core.Logger
}

// NewLoggingHook creates a LoggingHook.
func NewLoggingHook(l core.Logger) *LoggingHook { return &LoggingHook{logger: l} }

func (h *LoggingHook) BeforeStep(_ context.Context, stepName string, img *core.ImageData) {
	h.logger.Debug("pipeline.step.start",
		"step", stepName,
		"format", img.Format,
		"width", img.Meta.Width,
		"height", img.Meta.Height,
	)
}

func (h *LoggingHook) AfterStep(_ context.Context, stepName string, img *core.ImageData, d time.Duration, err error) {
	if err != nil {
		h.logger.Error("pipeline.step.error",
			"step", stepName,
			"duration_ms", d.Milliseconds(),
			"error", err.Error(),
		)
		return
	}
	out := "nil"
	if img != nil {
		out = fmt.Sprintf("%dx%d %s %dB", img.Meta.Width, img.Meta.Height, img.Format, img.Meta.SizeBytes)
	}
	h.logger.Debug("pipeline.step.done",
		"step", stepName,
		"duration_ms", d.Milliseconds(),
		"output", out,
	)
}

// ── In-memory metrics collector ───────────────────────────────────────────────

// InMemoryMetrics accumulates metrics atomically; safe for concurrent use.
type InMemoryMetrics struct {
	mu sync.RWMutex

	stepDurationsMs map[string]int64 // cumulative ms per step
	stepCalls       map[string]int64 // call count per step
	stepErrors      map[string]int64

	totalThroughputB int64
	totalMemoryB     int64
}

// NewInMemoryMetrics creates an empty metrics store.
func NewInMemoryMetrics() *InMemoryMetrics {
	return &InMemoryMetrics{
		stepDurationsMs: make(map[string]int64),
		stepCalls:       make(map[string]int64),
		stepErrors:      make(map[string]int64),
	}
}

func (m *InMemoryMetrics) RecordProcessingTime(stepName string, d interface{ Seconds() float64 }) {
	ms := int64(d.Seconds() * 1000)
	m.mu.Lock()
	m.stepDurationsMs[stepName] += ms
	m.stepCalls[stepName]++
	m.mu.Unlock()
}

func (m *InMemoryMetrics) RecordThroughput(bytes int64) {
	atomic.AddInt64(&m.totalThroughputB, bytes)
}

func (m *InMemoryMetrics) RecordMemory(bytes int64) {
	atomic.AddInt64(&m.totalMemoryB, bytes)
}

func (m *InMemoryMetrics) RecordError(stepName string, _ string) {
	m.mu.Lock()
	m.stepErrors[stepName]++
	m.mu.Unlock()
}

// Snapshot returns a copy of current metrics.
func (m *InMemoryMetrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap := MetricsSnapshot{
		StepDurationsMs: make(map[string]int64, len(m.stepDurationsMs)),
		StepCalls:       make(map[string]int64, len(m.stepCalls)),
		StepErrors:      make(map[string]int64, len(m.stepErrors)),
		TotalThroughputB: atomic.LoadInt64(&m.totalThroughputB),
		TotalMemoryB:     atomic.LoadInt64(&m.totalMemoryB),
	}
	for k, v := range m.stepDurationsMs {
		snap.StepDurationsMs[k] = v
	}
	for k, v := range m.stepCalls {
		snap.StepCalls[k] = v
	}
	for k, v := range m.stepErrors {
		snap.StepErrors[k] = v
	}
	return snap
}

// MetricsSnapshot is an immutable point-in-time copy of metrics.
type MetricsSnapshot struct {StepDurationsMs  map[string]int64
	StepCalls        map[string]int64
	StepErrors       map[string]int64
	TotalThroughputB int64
	TotalMemoryB     int64
}

// ── Metrics hook ──────────────────────────────────────────────────────────────

// MetricsHook feeds pipeline events into a MetricsCollector.
type MetricsHook struct {
	collector core.MetricsCollector
}

// NewMetricsHook creates a MetricsHook.
func NewMetricsHook(c core.MetricsCollector) *MetricsHook { return &MetricsHook{collector: c} }

func (h *MetricsHook) BeforeStep(_ context.Context, _ string, _ *core.ImageData) {}

func (h *MetricsHook) AfterStep(_ context.Context, stepName string, img *core.ImageData, d time.Duration, err error) {
	h.collector.RecordProcessingTime(stepName, d)
	if err != nil {
		h.collector.RecordError(stepName, "pipeline")
	}
	if img != nil {
		h.collector.RecordThroughput(img.Meta.SizeBytes)
	}
}