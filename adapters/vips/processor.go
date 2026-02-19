package vips

import (
	"context"
	"fmt"
	"io"
	"runtime"

	govips "github.com/davidbyttow/govips/v2/vips"

	"github.com/Skryldev/image-processor/core"
	apperrors "github.com/Skryldev/image-processor/errors"
	"github.com/Skryldev/image-processor/utils"
)

// BackendConfig configures the libvips backend.
type BackendConfig struct {
	DefaultQuality int
	MaxCacheSize   int
	MaxWorkers     int
	ReportLeaks    bool
}

// Backend is a unified libvips-powered Decoder and Encoder.
// Safe for concurrent use across goroutines.
type Backend struct {
	cfg BackendConfig
}

// NewBackend initialises libvips and returns a ready Backend.
// Call Shutdown() when the process exits.
func NewBackend(cfg BackendConfig) *Backend {
	if cfg.DefaultQuality <= 0 {
		cfg.DefaultQuality = 85
	}
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = runtime.NumCPU()
	}
	govips.Startup(&govips.Config{
		ConcurrencyLevel: cfg.MaxWorkers,
		MaxCacheSize:     cfg.MaxCacheSize,
		ReportLeaks:      cfg.ReportLeaks,
		CollectStats:     true,
	})
	return &Backend{cfg: cfg}
}

// Shutdown releases all libvips resources. Call once at process exit.
func (b *Backend) Shutdown() {
	govips.Shutdown()
}

// ─── Decoder ──────────────────────────────────────────────────────────────────

func (b *Backend) CanDecode(f core.Format) bool {
	switch f {
	case core.FormatJPEG, core.FormatPNG, core.FormatWebP, core.FormatUnknown:
		return true
	}
	return false
}

func (b *Backend) Decode(ctx context.Context, r io.Reader) (*core.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryDecode, "vips.decode", err)
	}

	buf, err := utils.DrainReader(ctx, r, 32*1024)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryDecode, "vips.decode.drain", err)
	}
	raw := utils.CloneBytes(buf.Bytes())
	utils.ReleaseBuffer(buf)

	ref, err := govips.NewImageFromBuffer(raw)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryDecode, "vips.decode", err)
	}
	runtime.SetFinalizer(ref, func(r *govips.ImageRef) { r.Close() })

	format := vipsFormatToCore(ref.Format())
	meta := core.Metadata{
		Width:       ref.Width(),
		Height:      ref.Height(),
		Format:      format,
		ColorSpace:  vipsInterpretationToColorSpace(ref.Interpretation()),
		HasAlpha:    ref.HasAlpha(),
		Orientation: ref.Orientation(),
	}
	fields := ref.GetFields()
	if len(fields) > 0 {
		exif := make(map[string]string, len(fields))
		for _, field := range fields {
			exif[field] = ref.GetString(field)
		}
		if len(exif) > 0 {
			meta.EXIF = exif
			meta.HasEXIF = true
		}
	}

	return &core.ImageData{
		Data:         raw,
		Format:       format,
		Image:        &VipsImage{ref: ref},
		Meta:         meta,
		OriginalSize: int64(len(raw)),
	}, nil
}

// ─── Encoder ──────────────────────────────────────────────────────────────────

func (b *Backend) CanEncode(f core.Format) bool {
	switch f {
	case core.FormatJPEG, core.FormatPNG, core.FormatWebP:
		return true
	}
	return false
}

func (b *Backend) Encode(ctx context.Context, img *core.ImageData, opts core.EncodeOptions) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryEncode, "vips.encode", err)
	}

	vi, ok := img.Image.(*VipsImage)
	if !ok || vi == nil {
		return nil, apperrors.New(apperrors.CategoryEncode, "vips.encode",
			fmt.Errorf("image must be decoded with the vips backend first"))
	}

	quality := opts.Quality
	if quality <= 0 {
		quality = b.cfg.DefaultQuality
	}

	switch img.Format {
	case core.FormatJPEG:
		ep := govips.NewJpegExportParams()
		ep.Quality = quality
		ep.StripMetadata = opts.StripEXIF
		ep.Interlace = opts.Interlaced
		buf, _, err := vi.ref.ExportJpeg(ep)
		if err != nil {
			return nil, apperrors.Wrap(apperrors.CategoryEncode, "vips.encode.jpeg", err)
		}
		return buf, nil

	case core.FormatPNG:
		ep := govips.NewPngExportParams()
		ep.StripMetadata = opts.StripEXIF
		ep.Interlace = opts.Interlaced
		buf, _, err := vi.ref.ExportPng(ep)
		if err != nil {
			return nil, apperrors.Wrap(apperrors.CategoryEncode, "vips.encode.png", err)
		}
		return buf, nil

	case core.FormatWebP:
		ep := govips.NewWebpExportParams()
		ep.Quality = quality
		ep.Lossless = opts.Lossless
		ep.StripMetadata = opts.StripEXIF
		buf, _, err := vi.ref.ExportWebp(ep)
		if err != nil {
			return nil, apperrors.Wrap(apperrors.CategoryEncode, "vips.encode.webp", err)
		}
		return buf, nil

	default:
		return nil, apperrors.New(apperrors.CategoryEncode, "vips.encode",
			fmt.Errorf("%w: %s", apperrors.ErrUnsupportedFormat, img.Format))
	}
}

// ─── VipsImage ────────────────────────────────────────────────────────────────

// VipsImage wraps a *govips.ImageRef for storage in core.ImageData.Image.
type VipsImage struct {
	ref *govips.ImageRef
}

func (v *VipsImage) Width() int              { return v.ref.Width() }
func (v *VipsImage) Height() int             { return v.ref.Height() }
func (v *VipsImage) Ref() *govips.ImageRef   { return v.ref }
func (v *VipsImage) Close()                  { v.ref.Close() }

// ─── VipsResizeStep ───────────────────────────────────────────────────────────

// VipsResizeStep resizes using vips_resize() with Lanczos3 kernel.
// For JPEG: triggers shrink-on-load so the full bitmap is never allocated.
type VipsResizeStep struct {
	Width, Height int
}

func (s *VipsResizeStep) Name() string { return "vips.resize" }

func (s *VipsResizeStep) Execute(ctx context.Context, img *core.ImageData) (*core.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryPipeline, s.Name(), err)
	}
	vi, ok := img.Image.(*VipsImage)
	if !ok || vi == nil {
		return nil, apperrors.New(apperrors.CategoryPipeline, s.Name(),
			fmt.Errorf("expected *VipsImage; use vips backend for decode"))
	}
	dstW, dstH := utils.ScaleDimensions(img.Meta.Width, img.Meta.Height, s.Width, s.Height)
	if dstW == img.Meta.Width && dstH == img.Meta.Height {
		return img, nil
	}
	scale := float64(dstW) / float64(img.Meta.Width)
	if err := vi.ref.Resize(scale, govips.KernelLanczos3); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryPipeline, s.Name(), err)
	}
	out := *img
	out.Meta.Width = vi.ref.Width()
	out.Meta.Height = vi.ref.Height()
	return &out, nil
}

// ─── VipsThumbnailStep ────────────────────────────────────────────────────────

// VipsThumbnailStep generates a square thumbnail using vips_thumbnail().
// Operates directly on encoded bytes — no separate decode step required.
type VipsThumbnailStep struct {
	Size int
}

func (s *VipsThumbnailStep) Name() string { return "vips.thumbnail" }

func (s *VipsThumbnailStep) Execute(ctx context.Context, img *core.ImageData) (*core.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryPipeline, s.Name(), err)
	}
	if len(img.Data) == 0 {
		return nil, apperrors.New(apperrors.CategoryPipeline, s.Name(), apperrors.ErrEmptyInput)
	}
	ref, err := govips.NewThumbnailFromBuffer(img.Data, s.Size, s.Size, govips.InterestingCentre)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryPipeline, s.Name(), err)
	}
	runtime.SetFinalizer(ref, func(r *govips.ImageRef) { r.Close() })
	out := *img
	out.Image = &VipsImage{ref: ref}
	out.Meta.Width = ref.Width()
	out.Meta.Height = ref.Height()
	return &out, nil
}

// ─── VipsStripEXIFStep ────────────────────────────────────────────────────────

// VipsStripEXIFStep removes all EXIF/XMP/IPTC metadata in-place.
type VipsStripEXIFStep struct{}

func (s *VipsStripEXIFStep) Name() string { return "vips.strip_exif" }

func (s *VipsStripEXIFStep) Execute(_ context.Context, img *core.ImageData) (*core.ImageData, error) {
	if vi, ok := img.Image.(*VipsImage); ok && vi != nil {
		vi.ref.RemoveMetadata()
	}
	out := *img
	out.Meta.EXIF = nil
	out.Meta.HasEXIF = false
	out.Meta.Orientation = 0
	return &out, nil
}

// ─── VipsAutoRotateStep ───────────────────────────────────────────────────────

// VipsAutoRotateStep applies the EXIF orientation tag then strips it.
type VipsAutoRotateStep struct{}

func (s *VipsAutoRotateStep) Name() string { return "vips.auto_rotate" }

func(s *VipsAutoRotateStep) Execute(_ context.Context, img *core.ImageData) (*core.ImageData, error) {
	vi, ok := img.Image.(*VipsImage)
	if !ok || vi == nil {
		return img, nil
	}
	if err := vi.ref.AutoRotate(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryPipeline, s.Name(), err)
	}
	out := *img
	out.Meta.Width = vi.ref.Width()
	out.Meta.Height = vi.ref.Height()
	out.Meta.Orientation = 0
	return &out, nil
}

// ─── RegisterVipsBackend ──────────────────────────────────────────────────────

// RegisterVipsBackend replaces Go stdlib codecs with libvips for all formats.
func RegisterVipsBackend(reg core.Registry, b *Backend) {
	for _, f := range []core.Format{core.FormatJPEG, core.FormatPNG, core.FormatWebP} {
		reg.RegisterDecoder(f, b)
		reg.RegisterEncoder(f, b)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func vipsFormatToCore(f govips.ImageType) core.Format {
	switch f {
	case govips.ImageTypeJPEG:
		return core.FormatJPEG
	case govips.ImageTypePNG:
		return core.FormatPNG
	case govips.ImageTypeWEBP:
		return core.FormatWebP
	default:
		return core.FormatUnknown
	}
}

func vipsInterpretationToColorSpace(i govips.Interpretation) core.ColorSpace {
	switch i {
	case govips.InterpretationSRGB, govips.InterpretationRGB16:
		return core.ColorSpaceRGB
	case govips.InterpretationBW:
		return core.ColorSpaceGray
	case govips.InterpretationCMYK:
		return core.ColorSpaceCMYK
	default:
		return core.ColorSpaceRGB
	}
}

// compile-time interface checks
var _ core.Decoder = (*Backend)(nil)
var _ core.Encoder = (*Backend)(nil)
var _ core.Step   = (*VipsResizeStep)(nil)
var _ core.Step   = (*VipsThumbnailStep)(nil)
var _ core.Step   = (*VipsStripEXIFStep)(nil)
var _ core.Step   = (*VipsAutoRotateStep)(nil)
