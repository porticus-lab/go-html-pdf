# go-html-pdf

Fast, open-source Go library that converts modern HTML + CSS into PDF documents. It renders pages through a headless Chrome browser via the [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/), so every CSS feature your browser supports — flexbox, grid, custom properties, gradients, `@media print`, web fonts — works out of the box.

All dependencies are free and open source.

## Requirements

- **Go 1.24+**
- **Chrome or Chromium** installed and available in `PATH`, **or** use `WithAutoDownload()` to let the library fetch one automatically

## Install

```bash
go get github.com/porticus-lab/go-html-pdf
```

## Quick Start

```go
package main

import (
	"context"
	"log"
	"os"

	htmlpdf "github.com/porticus-lab/go-html-pdf"
)

func main() {
	// Create a converter — starts a headless browser once.
	c, err := htmlpdf.NewConverter()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	html := `<!DOCTYPE html>
<html>
<head><style>
  body { font-family: system-ui, sans-serif; padding: 2rem; }
  h1   { color: #1e40af; }
</style></head>
<body>
  <h1>Hello, PDF!</h1>
  <p>Generated from Go with modern CSS support.</p>
</body>
</html>`

	res, err := c.ConvertHTML(context.Background(), html, nil)
	if err != nil {
		log.Fatal(err)
	}

	res.WriteToFile("hello.pdf", 0o644)
}
```

Pass `nil` as the page config and you get sensible defaults: **A4, portrait, 1 cm margins, scale 1.0, backgrounds enabled**.

## Page Configuration

Use `PageConfig` to control the output:

```go
page := &htmlpdf.PageConfig{
	Size:            htmlpdf.Letter,
	Orientation:     htmlpdf.Landscape,
	Margin:          htmlpdf.Margin{Top: 2, Right: 2.5, Bottom: 2, Left: 2.5},
	Scale:           0.9,
	PrintBackground: true,
}

res, err := c.ConvertHTML(ctx, html, page)
```

### Available Page Sizes

| Name      | Dimensions (cm) |
|-----------|-----------------|
| `A3`      | 29.7 × 42.0     |
| `A4`      | 21.0 × 29.7     |
| `A5`      | 14.8 × 21.0     |
| `Letter`  | 21.59 × 27.94   |
| `Legal`   | 21.59 × 35.56   |
| `Tabloid` | 27.94 × 43.18   |

### PageConfig Fields

| Field                 | Type          | Default     | Description                                          |
|-----------------------|---------------|-------------|------------------------------------------------------|
| `Size`                | `PageSize`    | `A4`        | Paper dimensions                                     |
| `Orientation`         | `Orientation` | `Portrait`  | `Portrait` or `Landscape`                            |
| `Margin`              | `Margin`      | 1 cm all    | Top, Right, Bottom, Left in centimeters              |
| `Scale`               | `float64`     | `1.0`       | Content scale factor (0.1–2.0)                       |
| `PrintBackground`     | `bool`        | `true`      | Include background colors and images                 |
| `DisplayHeaderFooter` | `bool`        | `false`     | Enable header/footer templates                       |
| `HeaderTemplate`      | `string`      | `""`        | HTML template for the page header                    |
| `FooterTemplate`      | `string`      | `""`        | HTML template for the page footer                    |
| `PreferCSSPageSize`   | `bool`        | `false`     | Prefer CSS `@page` size over `Size`                  |

## Input Sources

The library can convert from three input types:

```go
// From an HTML string
res, err := c.ConvertHTML(ctx, "<h1>Hello</h1>", page)

// From a local file
res, err := c.ConvertFile(ctx, "report.html", page)

// From a URL
res, err := c.ConvertURL(ctx, "https://example.com", page)
```

## Converter Options

Configure the `Converter` with functional options:

```go
c, err := htmlpdf.NewConverter(
	htmlpdf.WithTimeout(60 * time.Second),      // conversion timeout (default: 30s)
	htmlpdf.WithChromePath("/usr/bin/chromium"), // custom Chrome path
	htmlpdf.WithNoSandbox(),                    // required when running as root / Docker
	htmlpdf.WithAutoDownload(),                 // auto-download Chromium if not installed
)
```

### Auto-Download

If Chrome is not installed on the host, `WithAutoDownload()` will download and cache a compatible Chromium binary on the first run:

```go
c, err := htmlpdf.NewConverter(htmlpdf.WithAutoDownload())
```

The binary is stored in `~/.cache/rod/browser` (Unix) or `%APPDATA%\rod\browser` (Windows) and reused on subsequent calls. The first invocation may take 10–30 s depending on network speed; after that, the check adds ~1 ms.

This option is ignored when `WithChromePath` is also set.

## One-off Conversions

For single conversions where you don't need to reuse the browser, use the package-level functions:

```go
res, err := htmlpdf.ConvertHTML(ctx, html, page, htmlpdf.WithNoSandbox())
```

These create a temporary `Converter` under the hood. If you're converting more than once, prefer creating a `Converter` directly — it reuses the browser process and is significantly faster.

## Headers and Footers

```go
page := &htmlpdf.PageConfig{
	DisplayHeaderFooter: true,
	HeaderTemplate: `<div style="font-size:10px; text-align:center; width:100%;">
		<span class="title"></span>
	</div>`,
	FooterTemplate: `<div style="font-size:10px; text-align:center; width:100%;">
		Page <span class="pageNumber"></span> of <span class="totalPages"></span>
	</div>`,
}
```

Available template classes: `date`, `title`, `url`, `pageNumber`, `totalPages`.

## Result Object

Every conversion method returns a `*Result` with helpers for different output formats:

```go
res, err := c.ConvertHTML(ctx, html, nil)

res.Bytes()                          // []byte — raw PDF content
res.Base64()                         // string — base64-encoded (RFC 4648)
res.Reader()                         // *bytes.Reader — implements io.Reader + io.Seeker
res.WriteTo(w)                       // writes to any io.Writer (implements io.WriterTo)
res.WriteToFile("out.pdf", 0o644)    // writes directly to disk
res.Len()                            // int — size in bytes
```

## Cloud Storage Upload

The `Result` methods make it straightforward to upload PDFs directly to cloud storage without intermediate files.

### GCP Cloud Storage

```go
import (
	"cloud.google.com/go/storage"
	htmlpdf "github.com/porticus-lab/go-html-pdf"
)

func uploadToGCS(ctx context.Context, c *htmlpdf.Converter, bucket, object string) error {
	res, err := c.ConvertHTML(ctx, "<h1>Invoice #1234</h1>", nil)
	if err != nil {
		return err
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	w := client.Bucket(bucket).Object(object).NewWriter(ctx)
	w.ContentType = "application/pdf"

	if _, err := res.WriteTo(w); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
```

### AWS S3

```go
import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	htmlpdf "github.com/porticus-lab/go-html-pdf"
)

func uploadToS3(ctx context.Context, c *htmlpdf.Converter, client *s3.Client, bucket, key string) error {
	res, err := c.ConvertHTML(ctx, "<h1>Report</h1>", nil)
	if err != nil {
		return err
	}

	contentType := "application/pdf"
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		Body:        res.Reader(),
		ContentType: &contentType,
	})
	return err
}
```

### Base64 in JSON APIs

```go
func handleGeneratePDF(w http.ResponseWriter, r *http.Request) {
	res, err := converter.ConvertHTML(r.Context(), htmlContent, nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"pdf":      res.Base64(),
		"filename": "report.pdf",
	})
}
```

## How It Works

The library has a small surface area — around 250 lines of core code — built on two ideas:

1. **Chrome does the rendering.** Instead of reimplementing a layout engine in Go (which would inevitably lag behind real browser support), the library launches a headless Chrome instance using [`chromedp`](https://github.com/chromedp/chromedp) and talks to it over the Chrome DevTools Protocol. This gives you the exact same rendering fidelity as your browser.

2. **The browser stays alive.** A `Converter` starts Chrome once and reuses it. Each call to `ConvertHTML`/`ConvertURL`/`ConvertFile` opens a new tab, navigates to the content, calls Chrome's `Page.printToPDF` API, and closes the tab. This keeps subsequent conversions fast.

### File Layout

```
├── doc.go            # Package documentation
├── page.go           # PageSize, Orientation, Margin, PageConfig types
├── options.go        # Functional options (WithTimeout, WithChromePath, ...)
├── errors.go         # Sentinel errors
├── result.go         # Result type (Bytes, Base64, Reader, WriteTo, WriteToFile)
├── browser.go        # Auto-download logic via go-rod/rod/lib/launcher
├── converter.go      # Converter struct, conversion methods, convenience functions
├── page_test.go      # Unit tests for page math (no Chrome needed)
├── result_test.go    # Unit tests for Result methods (no Chrome needed)
├── converter_test.go # Integration tests (skipped if Chrome is not installed)
└── example_test.go   # Testable examples for go doc
```

### Dependencies

| Package                  | License | Purpose                          |
|--------------------------|---------|----------------------------------|
| `chromedp/chromedp`      | MIT     | Headless Chrome driver           |
| `chromedp/cdproto`       | MIT     | Chrome DevTools Protocol types   |
| `go-rod/rod`             | MIT     | Chromium auto-download (launcher)|

No paid services, no SaaS APIs, no CGo.

## Running in Docker

Chrome needs a few flags when running inside containers:

```go
c, err := htmlpdf.NewConverter(htmlpdf.WithNoSandbox())
```

You can install Chromium in the image or let the library download it automatically:

```go
c, err := htmlpdf.NewConverter(
	htmlpdf.WithAutoDownload(),
	htmlpdf.WithNoSandbox(),
)
```

A minimal Dockerfile (with system Chromium):

```dockerfile
FROM golang:1.24-bookworm

RUN apt-get update && apt-get install -y chromium && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .
RUN go build -o server .
CMD ["./server"]
```

## License

Apache 2.0 — see [LICENSE](LICENSE).
