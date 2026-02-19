package core

import "sync"

// ── Registry ──────────────────────────────────────────────────────────────────

// DefaultRegistry is a thread-safe implementation of Registry.
type DefaultRegistry struct {
	mu       sync.RWMutex
	decoders map[Format]Decoder
	encoders map[Format]Encoder
}

// NewRegistry returns an empty DefaultRegistry.
func NewRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		decoders: make(map[Format]Decoder),
		encoders: make(map[Format]Encoder),
	}
}

func (r *DefaultRegistry) RegisterDecoder(f Format, d Decoder) {
	r.mu.Lock()
	r.decoders[f] = d
	r.mu.Unlock()
}

func (r *DefaultRegistry) RegisterEncoder(f Format, e Encoder) {
	r.mu.Lock()
	r.encoders[f] = e
	r.mu.Unlock()
}

func (r *DefaultRegistry) DecoderFor(f Format) (Decoder, bool) {
	r.mu.RLock()
	d, ok := r.decoders[f]
	r.mu.RUnlock()
	return d, ok
}

func (r *DefaultRegistry) EncoderFor(f Format) (Encoder, bool) {
	r.mu.RLock()
	e, ok := r.encoders[f]
	r.mu.RUnlock()
	return e, ok
}