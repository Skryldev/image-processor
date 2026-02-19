package imageprocessor

import "github.com/Skryldev/image-processor/core"

// Inner exposes the underlying core.Processor for advanced use (e.g., direct
// registry access in tests).  Prefer the high-level API for normal usage.
func (p *Processor) Inner() *core.Processor { return p.inner }