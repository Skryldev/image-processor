package decoder

import (
	"context"
	"fmt"
	"image"
	"io"

	"github.com/Skryldev/image-processor/core"
	apperrors "github.com/Skryldev/image-processor/errors"
	"github.com/Skryldev/image-processor/utils"
	"golang.org/x/image/webp"
)

// WebP decodes WebP images using golang.org/x/image/webp.
// NOTE: golang.org/x/image/webp only supports lossy WebP decoding.
// For lossless or animated WebP, integrate libvips or google/go-webp.
type WebP struct{}

func NewWebP() *WebP { return &WebP{} }

func (w *WebP) CanDecode(format core.Format) bool {
	return format == core.FormatWebP
}

func (w *WebP) Decode(ctx context.Context, r io.Reader) (*core.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryDecode, "webp.decode", err)
	}

	// Buffer the reader so we can both decode and retain the original bytes.
	buf, err := utils.DrainReader(ctx, r, 32*1024)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryDecode, "webp.drain", err)
	}
	defer utils.ReleaseBuffer(buf)

	img, err := webp.Decode(utils.BytesReader(buf.Bytes()))
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryDecode, "webp.decode", err)
	}

	bounds := img.Bounds()
	meta := core.Metadata{
		Width:      bounds.Dx(),
		Height:     bounds.Dy(),
		Format:     core.FormatWebP,
		ColorSpace: colorSpace(img.(image.Image)),
		HasAlpha:   hasAlpha(img.(image.Image)),
	}

	return &core.ImageData{
		Image:  img,
		Format: core.FormatWebP,
		Meta:   meta,
	}, nil
}

// ensure image.Image is satisfied (webp.Decode returns image.Image).
var _ = fmt.Sprintf // suppress unused import