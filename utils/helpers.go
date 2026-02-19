package utils

import (
	"bytes"
	"net/http"
)

const (
	formatJPEG    = "jpeg"
	formatPNG     = "png"
	formatWebP    = "webp"
	formatUnknown = "unknown"
)

// DetectFormat sniffs the first 512 bytes of data and returns the image format.
func DetectFormat(data []byte) string {
	if len(data) < 4 {
		return formatUnknown
	}
	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return formatJPEG
	}
	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return formatPNG
	}
	// WebP: RIFF....WEBP
	if len(data) >= 12 &&
		data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P' {
		return formatWebP
	}
	// Fallback to net/http sniffing.
	ct := http.DetectContentType(data)
	switch ct {
	case "image/jpeg":
		return formatJPEG
	case "image/png":
		return formatPNG
	case "image/webp":
		return formatWebP
	}
	return formatUnknown
}

// ScaleDimensions computes output (w, h) preserving aspect ratio.
// Pass 0 for either axis to calculate it from the other.
func ScaleDimensions(srcW, srcH, targetW, targetH int) (int, int) {
	if targetW == 0 && targetH == 0 {
		return srcW, srcH
	}
	if targetW == 0 {
		ratio := float64(targetH) / float64(srcH)
		return int(float64(srcW) * ratio), targetH
	}
	if targetH == 0 {
		ratio := float64(targetW) / float64(srcW)
		return targetW, int(float64(srcH) * ratio)
	}
	return targetW, targetH
}

// PeekReader reads up to n bytes without consuming them (returns a new reader
// containing the peeked bytes followed by the rest of orig).
func PeekReader(orig []byte, n int) (peek []byte, rest []byte) {
	if n > len(orig) {
		n = len(orig)
	}
	return orig[:n], orig
}

// CloneBytes returns a copy of b (safe for use after the source buffer is released).
func CloneBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// BytesReader creates an io.Reader backed by b without allocation.
func BytesReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}
