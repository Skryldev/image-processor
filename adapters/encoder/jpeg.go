package encoder

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"

	"github.com/Skryldev/image-processor/core"
	apperrors "github.com/Skryldev/image-processor/errors"
)

// JPEG encodes images to JPEG format.
type JPEG struct {
	DefaultQuality int // used when EncodeOptions.Quality == 0
}

func NewJPEG(defaultQuality int) *JPEG {
	if defaultQuality <= 0 {
		defaultQuality = 85
	}
	return &JPEG{DefaultQuality: defaultQuality}
}

func (j *JPEG) CanEncode(format core.Format) bool {
	return format == core.FormatJPEG
}

func (j *JPEG) Encode(ctx context.Context, img *core.ImageData, opts core.EncodeOptions) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryEncode, "jpeg.encode", err)
	}

	src, ok := img.Image.(image.Image)
	if !ok || src == nil {
		return nil, apperrors.New(apperrors.CategoryEncode, "jpeg.encode", apperrors.ErrEmptyInput)
	}

	quality := opts.Quality
	if quality <= 0 {
		quality = j.DefaultQuality
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: quality}); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryEncode, "jpeg.encode", err)
	}
	return buf.Bytes(), nil
}