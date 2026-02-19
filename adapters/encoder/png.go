package encoder

import (
	"bytes"
	"context"
	"image"
	"image/png"

	"github.com/Skryldev/image-processor/core"
	apperrors "github.com/Skryldev/image-processor/errors"
)

// PNG encodes images to PNG format.
type PNG struct{}

func NewPNG() *PNG { return &PNG{} }

func (p *PNG) CanEncode(format core.Format) bool { return format == core.FormatPNG }

func (p *PNG) Encode(ctx context.Context, img *core.ImageData, opts core.EncodeOptions) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryEncode, "png.encode", err)
	}

	src, ok := img.Image.(image.Image)
	if !ok || src == nil {
		return nil, apperrors.New(apperrors.CategoryEncode, "png.encode", apperrors.ErrEmptyInput)
	}

	enc := &png.Encoder{}
	if opts.Lossless {
		enc.CompressionLevel = png.BestCompression
	} else {
		enc.CompressionLevel = png.DefaultCompression
	}
	if opts.Interlaced {
		enc.CompressionLevel = png.BestCompression // closest approximation
	}

	var buf bytes.Buffer
	if err := enc.Encode(&buf, src); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryEncode, "png.encode", err)
	}
	return buf.Bytes(), nil
}