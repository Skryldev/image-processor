<div dir="rtl">

<div align="center">

# 🖼️ imageprocessor

**ماژول پردازش تصویر enterprise-grade برای Go**

یک کتابخانه‌ی production-ready، ماژولار، و قابل توسعه برای پردازش تصاویر در حجم بالا  
طراحی‌شده با اصول Clean Architecture — بدون global state، با libvips backend، با کارایی واقعی

---

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)
[![Architecture](https://img.shields.io/badge/arch-Clean%20Architecture-blue?style=flat-square)]()
[![Backend](https://img.shields.io/badge/backend-libvips-orange?style=flat-square)]()

</div>

---

## 📋 فهرست مطالب

- [🖼️ imageprocessor](#️-imageprocessor)
  - [📋 فهرست مطالب](#-فهرست-مطالب)
  - [🤔 چرا این ماژول؟](#-چرا-این-ماژول)
    - [مشکل واقعی](#مشکل-واقعی)
    - [راه‌حل این ماژول](#راهحل-این-ماژول)
  - [✨ ویژگی‌ها](#-ویژگیها)
    - [پردازش تصویر](#پردازش-تصویر)
    - [معماری و کارایی](#معماری-و-کارایی)
  - [📦 نصب و راه‌اندازی](#-نصب-و-راهاندازی)
    - [پیش‌نیازها](#پیشنیازها)
    - [نصب libvips](#نصب-libvips)
    - [نصب ماژول](#نصب-ماژول)
    - [Docker (توصیه‌شده برای production)](#docker-توصیهشده-برای-production)
  - [⚡ شروع سریع](#-شروع-سریع)
  - [📚 راهنمای کامل استفاده](#-راهنمای-کامل-استفاده)
    - [۱. ساخت و پیکربندی Processor](#۱-ساخت-و-پیکربندی-processor)
    - [۲. تغییر اندازه (Resize)](#۲-تغییر-اندازه-resize)
    - [۳. برش (Crop)](#۳-برش-crop)
    - [۴. تبدیل فرمت](#۴-تبدیل-فرمت)
    - [۵. تولید Thumbnail](#۵-تولید-thumbnail)
    - [۶. کنترل کیفیت و Adaptive Compression](#۶-کنترل-کیفیت-و-adaptive-compression)
    - [۷. حذف EXIF + Auto Rotate](#۷-حذف-exif--auto-rotate)
    - [۸. پردازش موازی چند خروجی (Variants)](#۸-پردازش-موازی-چند-خروجی-variants)
    - [۹. پردازش دسته‌ای (Batch)](#۹-پردازش-دستهای-batch)
    - [۱۰. پردازش غیرهمزمان (Async Worker Pool)](#۱۰-پردازش-غیرهمزمان-async-worker-pool)
    - [۱۱. ذخیره‌سازی](#۱۱-ذخیرهسازی)
      - [Local Storage](#local-storage)
      - [S3 / MinIO](#s3--minio)
    - [۱۲. Observability: لاگ و متریک](#۱۲-observability-لاگ-و-متریک)
    - [۱۳. Step سفارشی](#۱۳-step-سفارشی)
  - [🌐 استفاده در HTTP Handler](#-استفاده-در-http-handler)
  - [⚠️ مدیریت خطا](#️-مدیریت-خطا)
  - [🚀 کارایی و بهینه‌سازی](#-کارایی-و-بهینهسازی)
    - [چرا libvips؟](#چرا-libvips)
    - [نتایج بنچمارک (Apple M2)](#نتایج-بنچمارک-apple-m2)
    - [Worker Pool و Backpressure](#worker-pool-و-backpressure)
  - [🧪 تست و بنچمارک](#-تست-و-بنچمارک)
  - [📊 مقایسه با رقبا](#-مقایسه-با-رقبا)
  - [📦 وابستگی‌ها](#-وابستگیها)
  - [🗺️ نقشه راه](#️-نقشه-راه)
  - [🤝 مشارکت در پروژه](#-مشارکت-در-پروژه)
  - [📄 لایسنس](#-لایسنس)

---

## 🤔 چرا این ماژول؟

### مشکل واقعی

وقتی یک سرویس backend نیاز به پردازش تصویر دارد، اغلب با این چالش‌ها روبرو می‌شود:

- کتابخانه‌های موجود یا **بیش از حد ساده** هستند (فقط resize) یا **بیش از حد وابسته** (نیاز به CGO، libvips، ImageMagick)
- **Global state** و singleton‌های پنهان که تست را غیرممکن می‌کنند
- **هیچ pipeline قابل توسعه‌ای** وجود ندارد — نمی‌توان یک step سفارشی اضافه کرد
- **کنترل ضعیف memory** — هر request یک buffer جدید allocate می‌کند
- **بدون worker pool** — در بار بالا goroutine explosion رخ می‌دهد
- **بدون مکانیزم retry** برای خطاهای گذرا

### راه‌حل این ماژول

```
✅ Clean Architecture بدون وابستگی بین لایه‌ها
✅ Zero global state — هر Processor مستقل است
✅ Plugable pipeline — هر step قابل جایگزینی یا افزودن
✅ libvips backend — 8× سریع‌تر از Go stdlib، 18× کمتر RAM
✅ Buffer pool با sync.Pool — حداقل GC pressure
✅ Worker pool با backpressure — کنترل کامل بار
✅ Retry هوشمند فقط برای خطاهای transient
✅ Context-aware — هر عملیات قابل لغو است
```

---

## ✨ ویژگی‌ها

### پردازش تصویر

| قابلیت | جزئیات |
|--------|---------|
| **فرمت‌های پشتیبانی‌شده** | JPEG، PNG، WebP (decode و encode واقعی) |
| **تغییر اندازه (Resize)** | Lanczos3 با shrink-on-load، دو محور مستقل |
| **برش (Crop)** | برش دقیق با مختصات مشخص |
| **Thumbnail** | vips_thumbnail — سریع‌ترین مسیر ممکن |
| **تبدیل فرمت** | JPEG↔PNG↔WebP در یک pipeline |
| **کنترل کیفیت** | Quality 1-100 قابل تنظیم per-step |
| **Adaptive Compression** | کاهش خودکار کیفیت تا رسیدن به حجم هدف |
| **حذف EXIF** | پاک‌سازی کامل metadata با libvips |
| **Auto Rotate** | اعمال EXIF orientation بدون pixel copy |
| **Grayscale** | تبدیل به تصویر سیاه و سفید |
| **Watermark** | افزودن لایه شفاف روی تصویر |

### معماری و کارایی

| قابلیت | جزئیات |
|--------|---------|
| **libvips Backend** | SIMD/AVX2، demand-driven pipeline، tile streaming |
| **Worker Pool** | تعداد worker قابل تنظیم، backpressure داخلی |
| **Batch Processing** | پردازش موازی چندین تصویر |
| **Async Queue** | ارسال job و دریافت نتیجه از channel |
| **Multi-Variant** | تولید چند نسخه از یک تصویر به صورت موازی |
| **Buffer Pool** | sync.Pool برای صفر allocation تکراری |
| **Context Aware** | لغو عملیات از طریق context.Context |
| **Retry مکانیزم** | تلاش مجدد فقط برای خطاهای transient |

---

## 📦 نصب و راه‌اندازی

### پیش‌نیازها

- **Go 1.22** یا بالاتر
- **libvips 8.x** نصب‌شده روی سیستم

### نصب libvips

```bash
# macOS
brew install vips

# Ubuntu / Debian
apt-get install libvips-dev

# Alpine Linux (Docker)
apk add --no-cache vips-dev build-base

# تأیید نصب
vips --version
```

### نصب ماژول

```bash
go get github.com/Skryldev/image-processor
```

### Docker (توصیه‌شده برای production)

```dockerfile
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache vips-dev build-base
WORKDIR /app
COPY . .
RUN CGO_ENABLED=1 go build -o server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache vips
COPY --from=builder /app/server /server
ENTRYPOINT ["/server"]
```

---

## ⚡ شروع سریع

```go
package main

import (
    "bytes"
    "context"
    "fmt"
    "os"

    imageprocessor "github.com/Skryldev/image-processor"
    "github.com/Skryldev/image-processor/adapters/vips"
    "github.com/Skryldev/image-processor/core"
    "github.com/Skryldev/image-processor/pipeline"
)

func main() {
    // ۱. ساخت processor
    proc := imageprocessor.New(imageprocessor.DefaultConfig())

    // ۲. فعال‌سازی libvips backend
    backend := vips.NewBackend(vips.BackendConfig{DefaultQuality: 85})
    defer backend.Shutdown()
    vips.RegisterVipsBackend(proc.Inner().Registry(), backend)

    proc.Start()
    defer proc.Stop()

    // ۳. خواندن فایل تصویر
    file, _ := os.Open("photo.jpg")
    defer file.Close()

    reg := proc.Inner().Registry()

    // ۴. پردازش: decode → resize → WebP → encode
    result, err := proc.Process(
        context.Background(),
        imageprocessor.FromReader(file),
        &pipeline.DecodeStep{Registry: reg},
        &vips.VipsResizeStep{Width: 1024}, // Lanczos3 + shrink-on-load
        imageprocessor.ConvertFormat(imageprocessor.WebP),
        imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85}),
    )
    if err != nil {
        panic(err)
    }

    // ۵. ذخیره خروجی
    os.WriteFile("output.webp", result.Primary.Data, 0644)

    fmt.Printf("✅ %dx%d — %d bytes — %.1f ms\n",
        result.Primary.Meta.Width,
        result.Primary.Meta.Height,
        len(result.Primary.Data),
        float64(result.ProcessingTime.Microseconds())/1000,
    )
}
```

---

## 📚 راهنمای کامل استفاده

### ۱. ساخت و پیکربندی Processor

```go
import (
    imageprocessor "github.com/alienrobotninja/imageprocessor"
    "github.com/alienrobotninja/imageprocessor/adapters/vips"
    "github.com/alienrobotninja/imageprocessor/config"
)

cfg := config.Default()
cfg.WorkerCount    = 8
cfg.QueueSize      = 512
cfg.DefaultQuality = 85
cfg.MaxRetries     = 3
cfg.RetryDelay     = 100 * time.Millisecond
cfg.JobTimeout     = 30 * time.Second
cfg.MaxImageBytes  = 20 * 1024 * 1024 // 20MB

proc := imageprocessor.New(cfg)

backend := vips.NewBackend(vips.BackendConfig{
    DefaultQuality: 85,
    MaxWorkers:     cfg.WorkerCount,
    MaxCacheSize:   100, // MB
})
defer backend.Shutdown()
vips.RegisterVipsBackend(proc.Inner().Registry(), backend)

proc.Start()
defer proc.Stop()
```

---

### ۲. تغییر اندازه (Resize)

از `VipsResizeStep` به جای `ResizeStep` استفاده کنید تا از shrink-on-load بهره ببرید:

```go
reg := proc.Inner().Registry()

// عرض ۸۰۰ — ارتفاع خودکار (Lanczos3)
result, _ := proc.Process(ctx, src,
    &pipeline.DecodeStep{Registry: reg},
    &vips.VipsResizeStep{Width: 800},
    imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85}),
)

// هر دو بُعد مشخص
result, _ = proc.Process(ctx, src,
    &pipeline.DecodeStep{Registry: reg},
    &vips.VipsResizeStep{Width: 800, Height: 600},
    imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85}),
)
```

---

### ۳. برش (Crop)

```go
// Crop(x, y, width, height)
result, _ := proc.Process(ctx, src,
    &pipeline.DecodeStep{Registry: reg},
    imageprocessor.Crop(100, 50, 400, 300),
    imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 90}),
)
```

---

### ۴. تبدیل فرمت

```go
// JPEG به WebP واقعی (libwebp)
result, _ := proc.Process(ctx, src,
    &pipeline.DecodeStep{Registry: reg},
    imageprocessor.ConvertFormat(imageprocessor.WebP),
    imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 80}),
)

// PNG lossless
result, _ = proc.Process(ctx, src,
    &pipeline.DecodeStep{Registry: reg},
    imageprocessor.ConvertFormat(imageprocessor.PNG),
    imageprocessor.EncodeWith(reg, core.EncodeOptions{Lossless: true}),
)
```

---

### ۵. تولید Thumbnail

`VipsThumbnailStep` مستقیم از bytes کار می‌کند — نیازی به DecodeStep ندارد:

```go
// shrink-on-load + Lanczos3 + centre crop — در یک C function call
result, _ := proc.Process(ctx, src,
    &vips.VipsThumbnailStep{Size: 256}, // نه DecodeStep نیاز است
    imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 75}),
)
```

---

### ۶. کنترل کیفیت و Adaptive Compression

```go
// کیفیت ثابت
result, _ := proc.Process(ctx, src,
    &pipeline.DecodeStep{Registry: reg},
    imageprocessor.Quality(70),
    imageprocessor.EncodeWith(reg, core.EncodeOptions{}),
)

// Adaptive Compression: کاهش خودکار کیفیت تا حجم هدف
result, _ = proc.Process(ctx, src,
    &pipeline.DecodeStep{Registry: reg},
    imageprocessor.AdaptiveCompress(
        reg,
        100 * 1024, // هدف: ۱۰۰ کیلوبایت
        30,         // حداقل کیفیت
        92,         // شروع از کیفیت ۹۲
    ),
)
```

---

### ۷. حذف EXIF + Auto Rotate

```go
result, _ := proc.Process(ctx, src,
    &pipeline.DecodeStep{Registry: reg},
    &vips.VipsAutoRotateStep{},  // اعمال EXIF orientation
    &vips.VipsStripEXIFStep{},   // حذف تمام metadata
    imageprocessor.EncodeWith(reg, core.EncodeOptions{
        StripEXIF: true,
        Quality:   85,
    }),
)
```

---

### ۸. پردازش موازی چند خروجی (Variants)

```go
result, err := proc.ProcessVariants(
    ctx,
    imageprocessor.FromReader(uploadedFile),

    // مرحله پایه: یک بار اجرا می‌شود
    []core.Step{
        &pipeline.DecodeStep{Registry: reg},
        &vips.VipsStripEXIFStep{},
    },

    // نسخه‌های موازی
    []core.VariantDefinition{
        {Name: "original", Steps: []core.Step{
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

for name, variant := range result.Variants {
    fmt.Printf("variant[%-10s]: %dx%d — %d bytes\n",
        name, variant.Meta.Width, variant.Meta.Height, len(variant.Data))
}
```

---

### ۹. پردازش دسته‌ای (Batch)

```go
sources := make([]core.Source, len(files))
for i, f := range files {
    sources[i] = imageprocessor.FromReader(f)
}

results, errs := proc.Batch(ctx, sources,
    &pipeline.DecodeStep{Registry: reg},
    &vips.VipsResizeStep{Width: 800},
    imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85}),
)
```

---

### ۱۰. پردازش غیرهمزمان (Async Worker Pool)

```go
resultCh := make(chan core.JobResult, 1)

err := proc.Submit(core.Job{
    ID:     "upload-" + uploadID,
    Ctx:    context.Background(),
    Source: imageprocessor.FromReader(uploadedFile),
    Steps: []core.Step{
        &pipeline.DecodeStep{Registry: reg},
        &vips.VipsStripEXIFStep{},
        &vips.VipsResizeStep{Width: 1200},
        imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85}),
    },
    ResultCh: resultCh,
})
if err != nil {
    // صف پر است — ErrWorkerPoolFull → HTTP 429
    return http.StatusTooManyRequests
}

go func() {
    select {
    case res := <-resultCh:
        if res.Err != nil {
            log.Printf("job failed: %v", res.Err)
            return
        }
        saveToStorage(res.JobID, res.Result.Primary.Data)
    case <-time.After(60 * time.Second):
        log.Println("job timed out")
    }
}()
```

---

### ۱۱. ذخیره‌سازی

#### Local Storage

```go
import "github.com/alienrobotninja/imageprocessor/adapters/storage"

localStorage, _ := storage.NewLocal("/var/data/images", 0644)

key := core.StorageKey{Bucket: "uploads", Path: "2024/01/photo.jpg"}
localStorage.Put(ctx, key, bytes.NewReader(result.Primary.Data), map[string]string{
    "width":  strconv.Itoa(result.Primary.Meta.Width),
    "format": string(result.Primary.Format),
})
```

#### S3 / MinIO

```go
s3Adapter, _ := storage.NewS3(&myS3Client{client: awsClient}, "my-bucket")
key := core.StorageKey{Path: "processed/photo.webp"}
s3Adapter.Put(ctx, key, bytes.NewReader(data), nil)
```

---

### ۱۲. Observability: لاگ و متریک

```go
import (
    "log/slog"
    "github.com/alienrobotninja/imageprocessor/hooks"
)

logger := hooks.NewSlogLogger(
    slog.New(slog.NewJSONHandler(os.Stdout, nil)),
)
proc.SetLogger(logger)
proc.AddHook(hooks.NewLoggingHook(logger))

metrics := hooks.NewInMemoryMetrics()
proc.AddHook(hooks.NewMetricsHook(metrics))

// --- بعد از پردازش ---
snap := metrics.Snapshot()
for step, count := range snap.StepCalls {
    fmt.Printf("step=%-22s calls=%-3d  avg=%.1fms\n",
        step, count,
        float64(snap.StepDurationsMs[step])/float64(count),
    )
}

processed, errCount := proc.Stats()
fmt.Printf("Processed: %d  Errors: %d\n", processed, errCount)
```

---

### ۱۳. Step سفارشی

```go
type BrightnessStep struct {
    Factor float64
}

func (s *BrightnessStep) Name() string { return "brightness" }

func (s *BrightnessStep) Execute(ctx context.Context, img *core.ImageData) (*core.ImageData, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    src := img.Image.(image.Image)
    bounds := src.Bounds()
    dst := image.NewRGBA(bounds)
    for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
        for x := bounds.Min.X; x < bounds.Max.X; x++ {
            r, g, b, a := src.At(x, y).RGBA()
            dst.SetRGBA(x, y, color.RGBA{
                R: clamp(float64(r>>8) * s.Factor),
                G: clamp(float64(g>>8) * s.Factor),
                B: clamp(float64(b>>8) * s.Factor),
                A: uint8(a >> 8),
            })
        }
    }
    out := *img
    out.Image = dst
    return &out, nil
}

// استفاده
result, _ := proc.Process(ctx, src,
    &pipeline.DecodeStep{Registry: reg},
    &BrightnessStep{Factor: 1.3},
    imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85}),
)
```

---

## 🌐 استفاده در HTTP Handler

```go
func (h *ImageHandler) Upload(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, 20<<20)

    if err := r.ParseMultipartForm(10 << 20); err != nil {
        http.Error(w, "فایل خیلی بزرگ است", http.StatusBadRequest)
        return
    }
    file, header, err := r.FormFile("image")
    if err != nil {
        http.Error(w, "فایل یافت نشد", http.StatusBadRequest)
        return
    }
    defer file.Close()

    reg := h.proc.Inner().Registry()

    result, err := h.proc.ProcessVariants(
        r.Context(),
        imageprocessor.FromReaderWithMeta(
            file, header.Size,
            header.Header.Get("Content-Type"),
            header.Filename,
        ),
        []core.Step{
            &pipeline.DecodeStep{Registry: reg},
            &vips.VipsAutoRotateStep{},
            &vips.VipsStripEXIFStep{},
        },
        []core.VariantDefinition{
            {Name: "original", Steps: []core.Step{
                imageprocessor.AdaptiveCompress(reg, 500*1024, 60, 92),
            }},
            {Name: "medium", Steps: []core.Step{
                &vips.VipsResizeStep{Width: 800},
                imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 85}),
            }},
            {Name: "thumb", Steps: []core.Step{
                &vips.VipsThumbnailStep{Size: 256},
                imageprocessor.EncodeWith(reg, core.EncodeOptions{Quality: 75}),
            }},
        },
    )
    if err != nil {
        http.Error(w, "خطا در پردازش تصویر", http.StatusInternalServerError)
        return
    }

    imageID := generateID()
    urls := map[string]string{}
    for name, variant := range result.Variants {
        key := core.StorageKey{Bucket: "images", Path: imageID + "/" + name + ".jpg"}
        h.storage.Put(r.Context(), key, bytes.NewReader(variant.Data), nil)
        urls[name] = "/images/" + key.Path
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "id":              imageID,
        "urls":            urls,
        "processing_time": result.ProcessingTime.String(),
    })
}
```

---

## ⚠️ مدیریت خطا

```go
import apperrors "github.com/alienrobotninja/imageprocessor/errors"

result, err := proc.Process(ctx, src, steps...)
if err != nil {
    switch {
    case apperrors.IsCategory(err, apperrors.CategoryDecode):
        http.Error(w, "فایل تصویر معتبر نیست", http.StatusBadRequest)

    case apperrors.IsCategory(err, apperrors.CategoryStorage):
        log.Printf("storage error: %v", err)
        http.Error(w, "خطای ذخیره‌سازی", http.StatusInternalServerError)

    case apperrors.IsRetryable(err):
        http.Error(w, "سرویس موقتاً در دسترس نیست", http.StatusServiceUnavailable)

    default:
        log.Printf("unexpected error: %v", err)
        http.Error(w, "خطای داخلی", http.StatusInternalServerError)
    }
    return
}
```

| Category | توضیح |
|----------|-------|
| `CategoryDecode` | فرمت ناشناخته، فایل خراب |
| `CategoryEncode` | خطای رمزگذاری خروجی |
| `CategoryPipeline` | خطای step در pipeline |
| `CategoryStorage` | خطای I/O ذخیره‌سازی |
| `CategoryTransient` | خطای گذرا (retryable) |
| `CategoryInput` | ورودی نامعتبر |

---

## 🚀 کارایی و بهینه‌سازی

### چرا libvips؟

libvips در سه سطح از Go stdlib سریع‌تر است:

**SIMD/AVX2** — libjpeg-turbo 32 پیکسل را در یک CPU cycle پردازش می‌کند در مقابل 1 پیکسل در Go.

**Shrink-On-Load** — برای thumbnail یک تصویر 4K، libvips به libjpeg-turbo می‌گوید در 1/8 رزولوشن decode کن. نتیجه: 64× کمتر داده قبل از اینکه Go اجرا شود.

**Tile Streaming** — libvips هیچ‌وقت کل bitmap را در RAM نگه نمی‌دارد. Peak RAM برای 1920×1080: Go stdlib = 11MB، libvips = 600KB.

### نتایج بنچمارک (Apple M2)

| عملیات | Go stdlib | libvips | بهبود |
|---------|-----------|---------|-------|
| Decode JPEG 1920×1080 | ~28 ms | ~3.8 ms | **7.5×** |
| Resize 1920→960 | ~8 ms | ~0.9 ms | **9×** |
| Thumbnail 4K → 256px | ~91 ms | ~9 ms | **10×** |
| Encode WebP 800×600 | — | ~1.8 ms | واقعی |
| Pipeline کامل | ~68 ms | ~8.5 ms | **8×** |
| Peak RAM (1920×1080) | ~11 MB | ~0.6 MB | **18×** کمتر |

```bash
# اجرای بنچمارک مقایسه‌ای
go test -bench=. -benchmem -count=3 ./adapters/vips/
```

### Worker Pool و Backpressure

```
بدون pool (100 request همزمان):   100 goroutine → احتمال OOM
با pool (WorkerCount=8):           8 goroutine فعال + 92 در صف → ثبات کامل
```

```go
err := proc.Submit(job)
if err == apperrors.ErrWorkerPoolFull {
    // HTTP 429 یا ذخیره در Redis queue
}
```

---

## 🧪 تست و بنچمارک

```bash
# تمام تست‌ها
go test ./...

# با race detector
go test -race ./...

# تست‌های خاص
go test -run TestProcess_JPEG_Resize -v ./...
go test -run TestWorkerPool_Async -v ./...
go test -run TestBatch -v ./...

# بنچمارک Go stdlib vs libvips
go test -bench=. -benchmem -count=3 ./adapters/vips/

# بنچمارک کل ماژول
go test -bench=. -benchmem ./...

# پوشش کد
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**نمونه خروجی بنچمارک:**
```
BenchmarkDecode_Stdlib_1920x1080-8     40    28400000 ns/op   8388608 B/op    12 allocs/op
BenchmarkDecode_Vips_1920x1080-8      300     3800000 ns/op    204800 B/op     4 allocs/op
BenchmarkThumbnail_Stdlib_4K-8         10    91000000 ns/op  22020096 B/op    18 allocs/op
BenchmarkThumbnail_Vips_4K-8          200     9200000 ns/op    409600 B/op     5 allocs/op
BenchmarkPipeline_Stdlib-8             15    68000000 ns/op  11534336 B/op    42 allocs/op
BenchmarkPipeline_Vips-8              120     8500000 ns/op    614400 B/op     8 allocs/op
```

---

## 📊 مقایسه با رقبا

| ویژگی | imageprocessor | disintegration/imaging | h2non/bimg | davidbyttow/govips |
|-------|:-:|:-:|:-:|:-:|
| Clean Architecture | ✅ | ❌ | ❌ | ❌ |
| Pipeline قابل توسعه | ✅ | ❌ | ❌ | ❌ |
| Worker Pool داخلی | ✅ | ❌ | ❌ | ❌ |
| Adaptive Compression | ✅ | ❌ | ❌ | ❌ |
| Multi-Variant موازی | ✅ | ❌ | ❌ | ❌ |
| Async Queue | ✅ | ❌ | ❌ | ❌ |
| Custom Storage | ✅ | ❌ | ❌ | ❌ |
| Hook/Metrics | ✅ | ❌ | ❌ | ❌ |
| libvips Backend | ✅ | ❌ | ✅ | ✅ |
| سرعت خام | **بالا** | کم | بسیار بالا | بالا |

---

## 📦 وابستگی‌ها

| پکیج | نسخه | دلیل |
|------|------|------|
| `github.com/davidbyttow/govips/v2` | v2.14.0 | libvips binding — decode، encode، resize، thumbnail |
| `golang.org/x/image` | v0.18.0 | resampling با کیفیت بالا برای Go stdlib steps |

---

## 🗺️ نقشه راه

- [x] libvips backend (VipsResizeStep، VipsThumbnailStep، VipsStripEXIFStep، VipsAutoRotateStep)
- [x] WebP encode واقعی با libwebp
- [ ] پشتیبانی از AVIF و HEIC
- [ ] ادپتر Google Cloud Storage
- [ ] ادپتر Azure Blob Storage
- [ ] پشتیبانی از GIF
- [ ] Redis queue برای async job‌ها
- [ ] Prometheus metrics exporter
- [ ] OpenTelemetry tracing
- [ ] CLI برای پردازش دسته‌ای از command line

---

## 🤝 مشارکت در پروژه

```bash
git clone https://github.com/alienrobotninja/imageprocessor.git
cd imageprocessor
git checkout -b feature/your-feature

go test -race ./...
go vet ./...

git commit -m "feat: add your feature"
git push origin feature/your-feature
```

**قوانین:**
- هر feature باید unit test داشته باشد
- رعایت اصل zero global state
- کامنت‌گذاری به انگلیسی برای سازگاری با Go community

---

## 📄 لایسنس

MIT License — آزاد برای استفاده تجاری و شخصی

---

<div align="center">

ساخته‌شده با ❤️ برای جامعه‌ی Go

</div>

</div>