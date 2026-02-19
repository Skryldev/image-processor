package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"log/slog"
	"os"
	"time"

	imageprocessor "github.com/Skryldev/image-processor"
	"github.com/Skryldev/image-processor/adapters/vips"
	"github.com/Skryldev/image-processor/config"
	"github.com/Skryldev/image-processor/core"
	"github.com/Skryldev/image-processor/hooks"
	"github.com/Skryldev/image-processor/pipeline"
)

func main() {
	// ── 1. Config ─────────────────────────────────────────────────────────────
	cfg := config.Default()
	cfg.WorkerCount = 4
	cfg.QueueSize = 128
	cfg.DefaultQuality = 85
	cfg.JobTimeout = 30 * time.Second

	// ── 2. Processor + libvips backend ────────────────────────────────────────
	proc := imageprocessor.New(cfg)

	backend := vips.NewBackend(vips.BackendConfig{
		DefaultQuality: 85,
		MaxWorkers:     cfg.WorkerCount,
	})
	defer backend.Shutdown()
	vips.RegisterVipsBackend(proc.Inner().Registry(), backend)

	// ── 3. Observability ──────────────────────────────────────────────────────
	logger := hooks.NewSlogLogger(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	metrics := hooks.NewInMemoryMetrics()
	proc.AddHook(hooks.NewLoggingHook(logger))
	proc.AddHook(hooks.NewMetricsHook(metrics))

	proc.Start()
	defer proc.Stop()

	raw, _ := os.ReadFile("./profile.jpg") // Read Real Source Image
	// raw := os.Args[1] // Read Source Image From CLI
	// raw := makeTestImage(1920, 1080) // Make a Face Image (No Need to open image)
	reg := proc.Inner().Registry()
	ctx := context.Background()

	// ── Example 1: Resize + Convert to WebP ───────────────────────────────────
	fmt.Println("\n── Example 1: Resize + WebP")
	result, err := proc.Process(ctx,
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: reg},
		&vips.VipsResizeStep{Width: 1024},
		imageprocessor.ConvertFormat(imageprocessor.WebP),
		imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 80}),
	)
	mustNoErr(err)
	printResult(result)

	// ── Example 2: Thumbnail ──────────────────────────────────────────────────
	fmt.Println("\n── Example 2: Thumbnail 256px (vips_thumbnail)")
	result, err = proc.Process(ctx,
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&vips.VipsThumbnailStep{Size: 256},
		imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 75}),
	)
	mustNoErr(err)
	printResult(result)

	// ── Example 3: Strip EXIF + Auto Rotate ───────────────────────────────────
	fmt.Println("\n── Example 3: Strip EXIF + Auto Rotate")
	result, err = proc.Process(ctx,
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: reg},
		&vips.VipsAutoRotateStep{},
		&vips.VipsStripEXIFStep{},
		imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85, StripEXIF: true}),
	)
	mustNoErr(err)
	printResult(result)

	// ── Example 4: Adaptive Compression ──────────────────────────────────────
	fmt.Println("\n── Example 4: Adaptive Compression (target 100KB)")
	result, err = proc.Process(ctx,
		imageprocessor.FromReader(bytes.NewReader(raw)),
		&pipeline.DecodeStep{Registry: reg},
		imageprocessor.AdaptiveCompress(reg, 100*1024, 40, 92),
	)
	mustNoErr(err)
	printResult(result)

	// ── Example 5: Multi-Variant (parallel) ───────────────────────────────────
	fmt.Println("\n── Example 5: Multi-Variant")
	varResult, err := proc.ProcessVariants(ctx,
		imageprocessor.FromReader(bytes.NewReader(raw)),
		[]core.Step{
			&pipeline.DecodeStep{Registry: reg},
			&vips.VipsStripEXIFStep{},
		},
		[]core.VariantDefinition{
			{Name: "large", Steps: []core.Step{
				&vips.VipsResizeStep{Width: 1920},
				imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 90}),
			}},
			{Name: "medium", Steps: []core.Step{
				&vips.VipsResizeStep{Width: 800},
				imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85}),
			}},
			{Name: "thumb", Steps: []core.Step{
				&vips.VipsThumbnailStep{Size: 256},
				imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 75}),
			}},
			{Name: "webp", Steps: []core.Step{
				&vips.VipsResizeStep{Width: 800},
				imageprocessor.ConvertFormat(imageprocessor.WebP),
				imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 80}),
			}},
		},
	)
	mustNoErr(err)
	for name, v := range varResult.Variants {
		fmt.Printf("  variant[%-10s]: %dx%d  %s  %d bytes\n",
			name, v.Meta.Width, v.Meta.Height, v.Format, len(v.Data))
	}

	// ── Example 6: Async Worker Pool ──────────────────────────────────────────
	fmt.Println("\n── Example 6: Async Job")
	resultCh := make(chan core.JobResult, 1)
	err = proc.Submit(core.Job{
		ID:  "job-001",
		Ctx: ctx,
		Source: imageprocessor.FromReader(bytes.NewReader(raw)),
		Steps: []core.Step{
			&pipeline.DecodeStep{Registry: reg},
			&vips.VipsResizeStep{Width: 400},
			imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 82}),
		},
		ResultCh: resultCh,
	})
	mustNoErr(err)
	select {
	case res := <-resultCh:
		mustNoErr(res.Err)
		fmt.Printf("  async result: %dx%d  %d bytes\n",
			res.Result.Primary.Meta.Width,
			res.Result.Primary.Meta.Height,
			len(res.Result.Primary.Data),
		)
	case <-time.After(30 * time.Second):
		log.Fatal("async job timed out")
	}

	// ── Metrics ───────────────────────────────────────────────────────────────
	fmt.Println("\n── Metrics")
	snap := metrics.Snapshot()
	for step, calls := range snap.StepCalls {
		avg := float64(snap.StepDurationsMs[step]) / float64(calls)
		fmt.Printf("  %-22s calls=%-3d  avg=%.1fms\n", step, calls, avg)
	}
	processed, errCount := proc.Stats()
	fmt.Printf("\nTotal processed: %d  Errors: %d\n", processed, errCount)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func mustNoErr(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}

func printResult(r *core.ProcessingResult) {
	fmt.Printf("  %dx%d  %s  %d bytes  %.1fms\n",
		r.Primary.Meta.Width,
		r.Primary.Meta.Height,
		r.Primary.Format,
		len(r.Primary.Data),
		float64(r.ProcessingTime.Microseconds())/1000,
	)
}

func makeTestImage(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 255 / w),
				G: uint8(y * 255 / h),
				B: 128, A: 255,
			})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92})
	return buf.Bytes()
}