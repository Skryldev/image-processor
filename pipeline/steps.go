// Package pipeline provides built-in pipeline steps and the extensible Step API.
package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/Skryldev/image-processor/core"
	apperrors "github.com/Skryldev/image-processor/errors"
	"github.com/Skryldev/image-processor/utils"
	xdraw "golang.org/x/image/draw"
)

// ── Resize ────────────────────────────────────────────────────────────────────

// ResizeStep resizes the image to the given dimensions, preserving aspect ratio
// when one axis is 0.
type ResizeStep struct {
	Width, Height int
	// Resampler controls quality vs speed.  Defaults to draw.BiLinear.
	Resampler xdraw.Interpolator
}

func (s *ResizeStep) Name() string { return "resize" }

func (s *ResizeStep) Execute(ctx context.Context, img *core.ImageData) (*core.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryPipeline, s.Name(), err)
	}

	src, ok := img.Image.(image.Image)
	if !ok || src == nil {
		return nil, apperrors.New(apperrors.CategoryPipeline, s.Name(), apperrors.ErrEmptyInput)
	}

	srcB := src.Bounds()
	dstW, dstH := utils.ScaleDimensions(srcB.Dx(), srcB.Dy(), s.Width, s.Height)

	if dstW == srcB.Dx() && dstH == srcB.Dy() {
		return img, nil // nothing to do
	}
	if dstW <= 0 || dstH <= 0 {
		return nil, apperrors.New(apperrors.CategoryPipeline, s.Name(), apperrors.ErrInvalidDimensions)
	}

	sampler := s.Resampler
	if sampler == nil {
		sampler = xdraw.BiLinear
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	sampler.Scale(dst, dst.Bounds(), src, srcB, xdraw.Over, nil)

	out := *img
	out.Image = dst
	out.Meta.Width = dstW
	out.Meta.Height = dstH
	return &out, nil
}

// ── Crop ──────────────────────────────────────────────────────────────────────

// CropStep crops a rectangle from the image.
type CropStep struct {
	X, Y, Width, Height int
}

func (s *CropStep) Name() string { return "crop" }

func (s *CropStep) Execute(ctx context.Context, img *core.ImageData) (*core.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryPipeline, s.Name(), err)
	}

	src, ok := img.Image.(image.Image)
	if !ok || src == nil {
		return nil, apperrors.New(apperrors.CategoryPipeline, s.Name(), apperrors.ErrEmptyInput)
	}

	rect := image.Rect(s.X, s.Y, s.X+s.Width, s.Y+s.Height)
	if !rect.In(src.Bounds()) {
		return nil, apperrors.New(apperrors.CategoryPipeline, s.Name(),
			fmt.Errorf("crop rect %v exceeds image bounds %v", rect, src.Bounds()))
	}

	dst := image.NewRGBA(image.Rect(0, 0, s.Width, s.Height))
	draw.Draw(dst, dst.Bounds(), src, rect.Min, draw.Src)

	out := *img
	out.Image = dst
	out.Meta.Width = s.Width
	out.Meta.Height = s.Height
	return &out, nil
}

// ── Format conversion ─────────────────────────────────────────────────────────

// FormatStep converts the image to a new format (sets img.Format for the
// subsequent encode step to pick up).
type FormatStep struct {
	Format core.Format
}

func (s *FormatStep) Name() string { return "format" }

func (s *FormatStep) Execute(_ context.Context, img *core.ImageData) (*core.ImageData, error) {
	out := *img
	out.Format = s.Format
	out.Meta.Format = s.Format
	return &out, nil
}

// ── Quality ───────────────────────────────────────────────────────────────────

// QualityStep records the desired encode quality.  The actual quality is
// consumed by EncodeStep.
type QualityStep struct {
	Quality int
}

func (s *QualityStep) Name() string { return "quality" }

func (s *QualityStep) Execute(_ context.Context, img *core.ImageData) (*core.ImageData, error) {
	out := *img
	// Store as a tag in Meta so EncodeStep can read it without coupling.
	if out.Meta.EXIF == nil {
		out.Meta.EXIF = make(map[string]string)
	}
	out.Meta.EXIF["_quality"] = fmt.Sprintf("%d", s.Quality)
	return &out, nil
}

// ── EXIF strip ────────────────────────────────────────────────────────────────

// StripEXIFStep removes EXIF metadata from the ImageData.
type StripEXIFStep struct{}

func (s *StripEXIFStep) Name() string { return "strip_exif" }

func (s *StripEXIFStep) Execute(_ context.Context, img *core.ImageData) (*core.ImageData, error) {
	out := *img
	out.Meta.EXIF = nil
	out.Meta.HasEXIF = false
	out.Meta.Orientation = 0
	return &out, nil
}

// ── Thumbnail ────────────────────────────────────────────────────────────────

// ThumbnailStep is a convenience step that combines Resize with square cropping.
type ThumbnailStep struct {
	Size int // square size in pixels
}

func (s *ThumbnailStep) Name() string { return "thumbnail" }

func (s *ThumbnailStep) Execute(ctx context.Context, img *core.ImageData) (*core.ImageData, error) {
	src, ok := img.Image.(image.Image)
	if !ok || src == nil {
		return nil, apperrors.New(apperrors.CategoryPipeline, s.Name(), apperrors.ErrEmptyInput)
	}

	// Step 1: resize so smallest dimension == s.Size.
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	var rw, rh int
	if w < h {
		rw, rh = s.Size, 0
	} else {
		rw, rh = 0, s.Size
	}

	resized, err := (&ResizeStep{Width: rw, Height: rh}).Execute(ctx, img)
	if err != nil {
		return nil, err
	}

	// Step 2: centre-crop to square.
	rb := resized.Image.(image.Image).Bounds()
	ox := (rb.Dx() - s.Size) / 2
	oy := (rb.Dy() - s.Size) / 2
	return (&CropStep{X: ox, Y: oy, Width: s.Size, Height: s.Size}).Execute(ctx, resized)
}

// ── Encode ────────────────────────────────────────────────────────────────────

// EncodeStep serialises the image.Image into encoded bytes using the registry.
type EncodeStep struct {
	Registry    core.Registry
	BaseOptions core.EncodeOptions
}

func (s *EncodeStep) Name() string { return "encode" }

func (s *EncodeStep) Execute(ctx context.Context, img *core.ImageData) (*core.ImageData, error) {
	enc, ok := s.Registry.EncoderFor(img.Format)
	if !ok {
		return nil, apperrors.New(apperrors.CategoryEncode, s.Name(),
			fmt.Errorf("%w: %s", apperrors.ErrUnsupportedFormat, img.Format))
	}

	opts := s.BaseOptions
	// Apply quality override stored by QualityStep.
	if img.Meta.EXIF != nil {
		if qs, found := img.Meta.EXIF["_quality"]; found {
			var q int
			fmt.Sscanf(qs, "%d", &q)
			opts.Quality = q
		}
	}

	data, err := enc.Encode(ctx, img, opts)
	if err != nil {
		return nil, err
	}

	out := *img
	out.Data = data
	out.Meta.SizeBytes = int64(len(data))
	return &out, nil
}

// ── AdaptiveCompress ──────────────────────────────────────────────────────────

// AdaptiveCompressStep iteratively adjusts JPEG/WebP quality to hit a target
// file size.
type AdaptiveCompressStep struct {
	Registry        core.Registry
	TargetSizeBytes int64
	MinQuality      int
	MaxQuality      int
	StepSize        int
}

func (s *AdaptiveCompressStep) Name() string { return "adaptive_compress" }

func (s *AdaptiveCompressStep) Execute(ctx context.Context, img *core.ImageData) (*core.ImageData, error) {
	if s.TargetSizeBytes <= 0 {
		return img, nil
	}
	enc, ok := s.Registry.EncoderFor(img.Format)
	if !ok {
		return img, nil // skip; unsupported format
	}

	quality := s.MaxQuality
	minQ := s.MaxQuality - 1
	if s.MinQuality > 0 {
		minQ = s.MinQuality
	}
	step := s.StepSize
	if step <= 0 {
		step = 5
	}

	var best []byte
	for quality >= minQ {
		if err := ctx.Err(); err != nil {
			return nil, apperrors.Wrap(apperrors.CategoryPipeline, s.Name(), err)
		}
		data, err := enc.Encode(ctx, img, core.EncodeOptions{Quality: quality})
		if err != nil {
			return nil, err
		}
		best = data
		if int64(len(data)) <= s.TargetSizeBytes {
			break
		}
		quality -= step
	}

	out := *img
	out.Data = best
	out.Meta.SizeBytes = int64(len(best))
	return &out, nil
}

// ── Decode ────────────────────────────────────────────────────────────────────

// DecodeStep decodes raw bytes in img.Data into an image.Image.
type DecodeStep struct {
	Registry core.Registry
}

func (s *DecodeStep) Name() string { return "decode" }

func (s *DecodeStep) Execute(ctx context.Context, img *core.ImageData) (*core.ImageData, error) {
	if img.Image != nil {
		return img, nil // already decoded
	}
	if len(img.Data) == 0 {
		return nil, apperrors.New(apperrors.CategoryDecode, s.Name(), apperrors.ErrEmptyInput)
	}
	dec, ok := s.Registry.DecoderFor(img.Format)
	if !ok {
		return nil, apperrors.New(apperrors.CategoryDecode, s.Name(),
			fmt.Errorf("%w: %s", apperrors.ErrUnsupportedFormat, img.Format))
	}

	decoded, err := dec.Decode(ctx, bytes.NewReader(img.Data))
	if err != nil {
		return nil, err
	}

	// Preserve the raw data bytes alongside the decoded representation.
	decoded.Data = img.Data
	decoded.OriginalSize = img.OriginalSize
	return decoded, nil
}

// ── Grayscale ─────────────────────────────────────────────────────────────────

// GrayscaleStep converts the image to grayscale.
type GrayscaleStep struct{}

func (s *GrayscaleStep) Name() string { return "grayscale" }

func (s *GrayscaleStep) Execute(_ context.Context, img *core.ImageData) (*core.ImageData, error) {
	src, ok := img.Image.(image.Image)
	if !ok || src == nil {
		return nil, apperrors.New(apperrors.CategoryPipeline, s.Name(), apperrors.ErrEmptyInput)
	}

	bounds := src.Bounds()
	dst := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x, y, color.GrayModel.Convert(src.At(x, y)))
		}
	}

	out := *img
	out.Image = dst
	out.Meta.ColorSpace = core.ColorSpaceGray
	return &out, nil
}

// ── Watermark ─────────────────────────────────────────────────────────────────

// WatermarkStep composites a watermark image onto the top-left corner.
type WatermarkStep struct {
	Watermark image.Image
	OffsetX   int
	OffsetY   int
}

func (s *WatermarkStep) Name() string { return "watermark" }

func (s *WatermarkStep) Execute(_ context.Context, img *core.ImageData) (*core.ImageData, error) {
	src, ok := img.Image.(image.Image)
	if !ok || src == nil {
		return nil, apperrors.New(apperrors.CategoryPipeline, s.Name(), apperrors.ErrEmptyInput)
	}

	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, image.Point{}, draw.Src)
	offset := image.Point{X: s.OffsetX, Y: s.OffsetY}
	draw.Draw(dst, s.Watermark.Bounds().Add(offset), s.Watermark, image.Point{}, draw.Over)

	out := *img
	out.Image = dst
	return &out, nil
}