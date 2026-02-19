package imageprocessor_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"sync"
	"testing"
	"time"

	imageprocessor "github.com/Skryldev/image-processor"
	"github.com/Skryldev/image-processor/config"
	"github.com/Skryldev/image-processor/core"
	"github.com/Skryldev/image-processor/hooks"
	"github.com/Skryldev/image-processor/pipeline"
	"github.com/Skryldev/image-processor/utils"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

func newRedJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 50, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	return buf.Bytes()
}

func newRedPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 50, G: 50, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func newProc(t *testing.T) *imageprocessor.Processor {
	t.Helper()
	cfg := imageprocessor.DefaultConfig()
	cfg.WorkerCount = 2
	cfg.QueueSize = 16
	p := imageprocessor.New(cfg)
	p.Start()
	t.Cleanup(p.Stop)
	return p
}

// ── Unit tests ────────────────────────────────────────────────────────────────

func TestProcess_JPEG_Resize(t *testing.T) {
	proc := newProc(t)
	raw := newRedJPEG(t, 800, 600)

	result, err := proc.Process(context.Background(),
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
		imageprocessor.Resize(400, 0),
		imageprocessor.EncodeWith(proc.Inner().Registry(), core.EncodeOptions{Quality: 80}),
	)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	got := result.Primary
	if got.Meta.Width != 400 {
		t.Errorf("width: got %d, want 400", got.Meta.Width)
	}
	// Aspect ratio: 800x600 → 400x300
	if got.Meta.Height != 300 {
		t.Errorf("height: got %d, want 300", got.Meta.Height)
	}
	if len(got.Data) == 0 {
		t.Error("encoded data is empty")
	}
}

func TestProcess_PNG_Decode(t *testing.T) {
	proc := newProc(t)
	raw := newRedPNG(t, 100, 100)

	result, err := proc.Process(context.Background(),
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
	)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if result.Primary.Meta.Format != core.FormatPNG {
		t.Errorf("format: got %s, want png", result.Primary.Meta.Format)
	}
}

func TestProcess_FormatConversion_JPEG_to_PNG(t *testing.T) {
	proc := newProc(t)
	raw := newRedJPEG(t, 200, 200)

	result, err := proc.Process(context.Background(),
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
		imageprocessor.ConvertFormat(imageprocessor.PNG),
		imageprocessor.EncodeWith(proc.Inner().Registry(), core.EncodeOptions{}),
	)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if result.Primary.Format != core.FormatPNG {
		t.Errorf("output format: got %s, want png", result.Primary.Format)
	}
}

func TestProcess_Thumbnail(t *testing.T) {
	proc := newProc(t)
	raw := newRedJPEG(t, 800, 400) // wide landscape

	result, err := proc.Process(context.Background(),
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
		imageprocessor.Thumbnail(100),
		imageprocessor.EncodeWith(proc.Inner().Registry(), core.EncodeOptions{Quality: 80}),
	)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if result.Primary.Meta.Width != 100 || result.Primary.Meta.Height != 100 {
		t.Errorf("thumbnail dimensions: %dx%d, want 100x100",
			result.Primary.Meta.Width, result.Primary.Meta.Height)
	}
}

func TestProcess_StripEXIF(t *testing.T) {
	proc := newProc(t)
	raw := newRedJPEG(t, 100, 100)

	// Inject fake EXIF data.
	result, err := proc.Process(context.Background(),
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
		imageprocessor.StripEXIF(),
	)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if result.Primary.Meta.EXIF != nil {
		t.Error("EXIF not stripped")
	}
}

func TestProcess_Grayscale(t *testing.T) {
	proc := newProc(t)
	raw := newRedJPEG(t, 50, 50)

	result, err := proc.Process(context.Background(),
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
		imageprocessor.Grayscale(),
	)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if result.Primary.Meta.ColorSpace != core.ColorSpaceGray {
		t.Errorf("color space: got %s, want gray", result.Primary.Meta.ColorSpace)
	}
}

func TestProcess_ContextCancel(t *testing.T) {
	proc := newProc(t)
	raw := newRedJPEG(t, 100, 100)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := proc.Process(ctx,
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
	)
	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}

// ── Table-driven tests ────────────────────────────────────────────────────────

func TestScaleDimensions(t *testing.T) {
	tests := []struct {
		srcW, srcH, targetW, targetH int
		wantW, wantH                 int
	}{
		{800, 600, 400, 0, 400, 300},
		{800, 600, 0, 300, 400, 300},
		{800, 600, 200, 200, 200, 200},
		{800, 600, 0, 0, 800, 600},
	}
	for _, tc := range tests {
		gotW, gotH := utils.ScaleDimensions(tc.srcW, tc.srcH, tc.targetW, tc.targetH)
		if gotW != tc.wantW || gotH != tc.wantH {
			t.Errorf("ScaleDimensions(%d,%d,%d,%d) = %d,%d; want %d,%d",
				tc.srcW, tc.srcH, tc.targetW, tc.targetH, gotW, gotH, tc.wantW, tc.wantH)
		}
	}
}

// ── Concurrency tests ─────────────────────────────────────────────────────────

func TestProcess_ConcurrentSafety(t *testing.T) {
	proc := newProc(t)
	raw := newRedJPEG(t, 200, 200)

	const goroutines = 20
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = proc.Process(context.Background(),
				imageprocessor.FromReader(bytes.NewReader(raw)),
				&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
				imageprocessor.Resize(100, 0),
				imageprocessor.EncodeWith(proc.Inner().Registry(), core.EncodeOptions{Quality: 80}),
			)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
}

// ── Batch test ────────────────────────────────────────────────────────────────

func TestBatch(t *testing.T) {
	proc := newProc(t)
	raw := newRedJPEG(t, 100, 100)

	sources := make([]core.Source, 5)
	for i := range sources {
		sources[i] = imageprocessor.FromReader(bytes.NewReader(raw))
	}

	results, errs := proc.Batch(context.Background(), sources,
		&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
		imageprocessor.Resize(50, 50),
	)

	for i, err := range errs {
		if err != nil {
			t.Errorf("batch[%d]: %v", i, err)
		}
		if results[i] == nil {
			t.Errorf("batch[%d]: nil result", i)
		}
	}
}

// ── Async worker pool test ────────────────────────────────────────────────────

func TestWorkerPool_Async(t *testing.T) {
	proc := newProc(t)
	raw := newRedJPEG(t, 100, 100)

	resultCh := make(chan core.JobResult, 1)
	job := core.Job{
		ID:  "test-job-1",
		Ctx: context.Background(),
		Source: imageprocessor.FromReader(bytes.NewReader(raw)),
		Steps: []core.Step{
			&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
			imageprocessor.Resize(50, 0),
		},
		ResultCh: resultCh,
	}

	if err := proc.Submit(job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case res := <-resultCh:
		if res.Err != nil {
			t.Fatalf("async job error: %v", res.Err)
		}
		if res.Result.Primary.Meta.Width != 50 {
			t.Errorf("async width: got %d, want 50", res.Result.Primary.Meta.Width)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("async job timed out")
	}
}

// ── Hooks /Metrics test ──────────────────────────────────────────────────────

func TestMetricsHook(t *testing.T) {
	m := hooks.NewInMemoryMetrics()
	hook := hooks.NewMetricsHook(m)

	proc := newProc(t)
	proc.AddHook(hook)

	raw := newRedJPEG(t, 100, 100)
	_, err := proc.Process(context.Background(),
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
		imageprocessor.Resize(50, 0),
	)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	snap := m.Snapshot()
	if snap.StepCalls["resize"] == 0 {
		t.Error("resize step was not recorded in metrics")
	}
}

// ── Custom step test ──────────────────────────────────────────────────────────

// brightenStep is a custom pipeline step for testing extensibility.
type brightenStep struct{ delta uint8 }

func (b *brightenStep) Name() string { return "brighten" }
func (b *brightenStep) Execute(_ context.Context, img *core.ImageData) (*core.ImageData, error) {
	src, ok := img.Image.(image.Image)
	if !ok {
		return img, nil
	}
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, bv, a := src.At(x, y).RGBA()
			dst.SetRGBA(x, y, color.RGBA{
				R: clampAdd(uint8(r>>8), b.delta),
				G: clampAdd(uint8(g>>8), b.delta),
				B: clampAdd(uint8(bv>>8), b.delta),
				A: uint8(a >> 8),
			})
		}
	}
	out := *img
	out.Image = dst
	return &out, nil
}

func clampAdd(a, b uint8) uint8 {
	if int(a)+int(b) > 255 {
		return 255
	}
	return a + b
}

func TestCustomStep(t *testing.T) {
	proc := newProc(t)
	raw := newRedJPEG(t, 50, 50)

	_, err := proc.Process(context.Background(),
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: proc.Inner().Registry()},
		&brightenStep{delta: 10},
		imageprocessor.EncodeWith(proc.Inner().Registry(), core.EncodeOptions{Quality: 80}),
	)
	if err != nil {
		t.Fatalf("Process with custom step: %v", err)
	}
}

// ── Config validation test ────────────────────────────────────────────────────

func TestConfigValidation(t *testing.T) {
	cfg := config.Default()
	cfg.DefaultQuality = 0 // invalid
	if err := config.Validate(cfg); err == nil {
		t.Error("expected validation error for quality=0")
	}
}

// ── Benchmarks ────────────────────────────────────────────────────────────────

func BenchmarkProcess_Resize_JPEG(b *testing.B) {
	cfg := imageprocessor.DefaultConfig()
	proc := imageprocessor.New(cfg)
	proc.Start()
	defer proc.Stop()

	raw := makeRedJPEGBench(b, 1920, 1080)
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&pipeline.DecodeStep{Registry: reg},
			imageprocessor.Resize(960, 0),
			imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85}),
		)
		if err != nil {
			b.Fatalf("Process: %v", err)
		}
	}
}

func BenchmarkProcess_Thumbnail(b *testing.B) {
	proc := imageprocessor.New(imageprocessor.DefaultConfig())
	proc.Start()
	defer proc.Stop()

	raw := makeRedJPEGBench(b, 1024, 768)
	reg := proc.Inner().Registry()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := proc.Process(context.Background(),
			imageprocessor.FromReader(bytes.NewReader(raw)),
			&pipeline.DecodeStep{Registry: reg},
			imageprocessor.Thumbnail(150),
			imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 80}),
		)
		if err != nil {
			b.Fatalf("Process: %v", err)
		}
	}
}

func BenchmarkBatch_Parallel(b *testing.B) {
	proc := imageprocessor.New(imageprocessor.DefaultConfig())
	proc.Start()
	defer proc.Stop()

	raw := makeRedJPEGBench(b, 800, 600)
	reg := proc.Inner().Registry()

	const batchSize = 10
	sources := make([]core.Source, batchSize)
	for i := range sources {
		sources[i] = imageprocessor.FromReader(bytes.NewReader(raw))
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		proc.Batch(context.Background(), sources,
			&pipeline.DecodeStep{Registry: reg},
			imageprocessor.Resize(400, 0),
		)
	}
}

func makeRedJPEGBench(b *testing.B, w, h int) []byte {
	b.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 50, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}
