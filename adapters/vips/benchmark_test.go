package vips_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	imageprocessor "github.com/Skryldev/image-processor"
	"github.com/Skryldev/image-processor/adapters/vips"
	"github.com/Skryldev/image-processor/core"
	"github.com/Skryldev/image-processor/pipeline"
)

func makeJPEG(b *testing.B, w, h int) []byte {
	b.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92})
	return buf.Bytes()
}

func newVipsProc(b *testing.B) (*imageprocessor.Processor, *vips.Backend) {
	b.Helper()
	proc := imageprocessor.New(imageprocessor.DefaultConfig())
	backend := vips.NewBackend(vips.BackendConfig{DefaultQuality: 85})
	vips.RegisterVipsBackend(proc.Inner().Registry(), backend)
	proc.Start()
	return proc, backend
}

func newStdlibProc(b *testing.B) *imageprocessor.Processor {
	b.Helper()
	proc := imageprocessor.New(imageprocessor.DefaultConfig())
	proc.Start()
	return proc
}

// ─── Decode ───────────────────────────────────────────────────────────────────

func BenchmarkDecode_Stdlib_1920x1080(b *testing.B) {
	raw := makeJPEG(b, 1920, 1080)
	proc := newStdlibProc(b)
	defer proc.Stop()
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&pipeline.DecodeStep{Registry: reg},
		); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecode_Vips_1920x1080(b *testing.B) {
	raw := makeJPEG(b, 1920, 1080)
	proc, backend := newVipsProc(b)
	defer proc.Stop()
	defer backend.Shutdown()
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&pipeline.DecodeStep{Registry: reg},
		); err != nil {
			b.Fatal(err)
		}
	}
}

// ─── Resize ───────────────────────────────────────────────────────────────────

func BenchmarkResize_Stdlib_1920to960(b *testing.B) {
	raw := makeJPEG(b, 1920, 1080)
	proc := newStdlibProc(b)
	defer proc.Stop()
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&pipeline.DecodeStep{Registry: reg},
			imageprocessor.Resize(960, 0),
			imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85}),
		); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResize_Vips_1920to960(b *testing.B) {
	raw := makeJPEG(b, 1920, 1080)
	proc, backend := newVipsProc(b)
	defer proc.Stop()
	defer backend.Shutdown()
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&pipeline.DecodeStep{Registry: reg},
			&vips.VipsResizeStep{Width: 960},
			imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85}),
		); err != nil {
			b.Fatal(err)
		}
	}
}

// ─── Thumbnail ────────────────────────────────────────────────────────────────

func BenchmarkThumbnail_Stdlib_4K(b *testing.B) {
	raw := makeJPEG(b, 3840, 2160)
	proc := newStdlibProc(b)
	defer proc.Stop()
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&pipeline.DecodeStep{Registry: reg},
			imageprocessor.Thumbnail(256),
			imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 75}),
		); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkThumbnail_Vips_4K(b *testing.B) {
	raw := makeJPEG(b, 3840, 2160)
	proc, backend := newVipsProc(b)
	defer proc.Stop()
	defer backend.Shutdown()
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&vips.VipsThumbnailStep{Size: 256},
			imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 75}),
		); err != nil {
			b.Fatal(err)
		}
	}
}

// ─── WebP encode ──────────────────────────────────────────────────────────────

func BenchmarkEncodeWebP_Stdlib(b *testing.B) {
	raw := makeJPEG(b, 800, 600)
	proc := newStdlibProc(b)
	defer proc.Stop()
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&pipeline.DecodeStep{Registry: reg},
			imageprocessor.ConvertFormat(imageprocessor.WebP),
			imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 80}),
		); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeWebP_Vips(b *testing.B) {
	raw := makeJPEG(b, 800, 600)
	proc, backend := newVipsProc(b)
	defer proc.Stop()
	defer backend.Shutdown()
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&pipeline.DecodeStep{Registry: reg},
			imageprocessor.ConvertFormat(imageprocessor.WebP),
			imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 80}),
		); err != nil {
			b.Fatal(err)
		}
	}
}

// ─── Full pipeline ────────────────────────────────────────────────────────────

func BenchmarkPipeline_Stdlib(b *testing.B) {
	raw := makeJPEG(b, 1920, 1080)
	proc := newStdlibProc(b)
	defer proc.Stop()
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&pipeline.DecodeStep{Registry: reg},
			imageprocessor.Resize(960, 0),
			imageprocessor.StripEXIF(),
			imageprocessor.ConvertFormat(imageprocessor.WebP),
			imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 80}),
		); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPipeline_Vips(b *testing.B) {
	raw := makeJPEG(b, 1920, 1080)
	proc, backend := newVipsProc(b)
	defer proc.Stop()
	defer backend.Shutdown()
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&pipeline.DecodeStep{Registry: reg},
			&vips.VipsResizeStep{Width: 960},
			&vips.VipsStripEXIFStep{},
			imageprocessor.ConvertFormat(imageprocessor.WebP),
			imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 80}),
		); err != nil {
			b.Fatal(err)
		}
	}
}