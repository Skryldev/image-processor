package decoder

import (
	"context"
	"image/png"
	"io"

	"github.com/Skryldev/image-processor/core"
	apperrors "github.com/Skryldev/image-processor/errors"
)

// PNG decodes PNG images using the standard library.
type PNG struct{}

func NewPNG() *PNG { return &PNG{} }

func (p *PNG) CanDecode(format core.Format) bool {
	return format == core.FormatPNG
}

func (p *PNG) Decode(ctx context.Context, r io.Reader) (*core.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryDecode, "png.decode", err)
	}

	img, err := png.Decode(r)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryDecode, "png.decode", err)
	}

	bounds := img.Bounds()
	meta := core.Metadata{
		Width:      bounds.Dx(),
		Height:     bounds.Dy(),
		Format:     core.FormatPNG,
		ColorSpace: colorSpace(img),
		HasAlpha:   hasAlpha(img),
	}

	return &core.ImageData{
		Image:  img,
		Format: core.FormatPNG,
		Meta:   meta,
	}, nil
}