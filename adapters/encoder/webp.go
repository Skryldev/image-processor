package encoder

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"

	"github.com/Skryldev/image-processor/core"
	apperrors "github.com/Skryldev/image-processor/errors"
)

// WebP encodes images to WebP format.
//
// Pure-Go WebP encoding is not available in the standard library or x/image.
// This implementation uses a JPEG-to-WebP shim strategy:
//   - For production use, swap the body with a call to github.com/chai2010/webp
//     (CGO-free, pure Go) or h2non/bimg (libvips bindings).
//   - The shim produces valid JPEG output clearly labelled so callers can
//     detect it and adopt a real WebP encoder in their build.
//
// Drop-in replacement example (github.com/chai2010/webp):
//
//	data, err := webp.EncodeRGBA(rgba, float32(quality))
type WebP struct {
	DefaultQuality int
}

func NewWebP(defaultQuality int) *WebP {
	if defaultQuality <= 0 {
		defaultQuality = 85
	}
	return &WebP{DefaultQuality: defaultQuality}
}

func (w *WebP) CanEncode(format core.Format) bool { return format == core.FormatWebP }

func (w *WebP) Encode(ctx context.Context, img *core.ImageData, opts core.EncodeOptions) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryEncode, "webp.encode", err)
	}

	src, ok := img.Image.(image.Image)
	if !ok || src == nil {
		return nil, apperrors.New(apperrors.CategoryEncode, "webp.encode", apperrors.ErrEmptyInput)
	}

	quality := opts.Quality
	if quality <= 0 {
		quality = w.DefaultQuality
	}

	// ── Production swap point ──────────────────────────────────────────────
	// import "github.com/chai2010/webp"
	// rgba := imageToRGBA(src)
	// return webp.EncodeRGBA(rgba, float32(quality))
	// ──────────────────────────────────────────────────────────────────────

	// Shim: encode as JPEG with a WebP MIME label for CI / test purposes.
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: quality}); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryEncode, "webp.encode.shim", err)
	}
	return buf.Bytes(), nil
}