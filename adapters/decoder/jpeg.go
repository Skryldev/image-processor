// Package decoder provides format-specific image decoders.
package decoder

import (
	"context"
	"image"
	"image/jpeg"
	"io"

	"github.com/Skryldev/image-processor/core"
	apperrors "github.com/Skryldev/image-processor/errors"
)

// JPEG decodes JPEG images using the standard library.
type JPEG struct{}

// NewJPEG returns an initialised JPEG decoder.
func NewJPEG() *JPEG { return &JPEG{} }

func (j *JPEG) CanDecode(format core.Format) bool {
	return format == core.FormatJPEG || format == core.FormatUnknown
}

func (j *JPEG) Decode(ctx context.Context, r io.Reader) (*core.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryDecode, "jpeg.decode", err)
	}

	img, err := jpeg.Decode(r)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryDecode, "jpeg.decode", err)
	}

	bounds := img.Bounds()
	meta := core.Metadata{
		Width:      bounds.Dx(),
		Height:     bounds.Dy(),
		Format:     core.FormatJPEG,
		ColorSpace: colorSpace(img),
		HasAlpha:   hasAlpha(img),
	}

	return &core.ImageData{
		Image:  img,
		Format: core.FormatJPEG,
		Meta:   meta,
	}, nil
}

// colorSpace returns the colour space of an image.Image.
func colorSpace(img image.Image) core.ColorSpace {
	switch img.ColorModel() {
	case nil:
		return core.ColorSpaceRGB
	default:
		switch img.(type) {
		case *image.Gray, *image.Gray16:
			return core.ColorSpaceGray
		case *image.RGBA, *image.NRGBA, *image.RGBA64:
			return core.ColorSpaceRGBA
		case *image.CMYK:
			return core.ColorSpaceCMYK
		}
	}
	return core.ColorSpaceRGB
}

func hasAlpha(img image.Image) bool {
	switch img.(type) {
	case *image.RGBA, *image.NRGBA, *image.RGBA64, *image.NRGBA64:
		return true
	}
	return false
}