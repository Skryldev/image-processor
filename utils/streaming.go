package utils

import (
	"bytes"
	"context"
	"io"
	"sync"
)

// bufPool reuses byte buffers to reduce GC pressure.
var bufPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

// AcquireBuffer returns a reset buffer from the pool.
func AcquireBuffer() *bytes.Buffer {
	b := bufPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

// ReleaseBuffer returns b to the pool.  Callers must not use b after this call.
func ReleaseBuffer(b *bytes.Buffer) {
	// Cap large buffers to avoid pinning excessive memory.
	if b.Cap() > 8*1024*1024 {
		return
	}
	bufPool.Put(b)
}

// DrainReader reads all bytes from r into a pooled buffer and returns them.
// The caller owns the returned slice; pass the buffer back with ReleaseBuffer.
func DrainReader(ctx context.Context, r io.Reader, chunkSize int) (*bytes.Buffer, error) {
	if chunkSize <= 0 {
		chunkSize = 32 * 1024
	}
	buf := AcquireBuffer()
	chunk := make([]byte, chunkSize)
	for {
		if err := ctx.Err(); err != nil {
			ReleaseBuffer(buf)
			return nil, err
		}
		n, err := r.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			ReleaseBuffer(buf)
			return nil, err
		}
	}
	return buf, nil
}

// LimitedReader wraps r and returns an error when more than max bytes are read.
type LimitedReader struct {
	R   io.Reader
	Max int64
	n   int64
}

func (l *LimitedReader) Read(p []byte) (int, error) {
	if l.n >= l.Max && l.Max > 0 {
		return 0, io.ErrUnexpectedEOF
	}
	if l.Max > 0 {
		remain := l.Max - l.n
		if int64(len(p)) > remain {
			p = p[:remain]
		}
	}
	n, err := l.R.Read(p)
	l.n += int64(n)
	return n, err
}

// ChunkedWriter splits writes into fixed-size chunks; useful for streaming uploads.
type ChunkedWriter struct {
	W         io.Writer
	ChunkSize int
}

func (c *ChunkedWriter) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		end := c.ChunkSize
		if end > len(p) {
			end = len(p)
		}
		n, err := c.W.Write(p[:end])
		total += n
		if err != nil {
			return total, err
		}
		p = p[end:]
	}
	return total, nil
}