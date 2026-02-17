# go-html-pdf

Fast, open-source Go library that converts modern HTML + CSS into PDF documents. It renders pages through a headless Chrome browser via the [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/), so every CSS feature your browser supports — flexbox, grid, custom properties, gradients, `@media print`, web fonts — works out of the box.

All dependencies are free and open source.

## Requirements

- **Go 1.24+**
- **Chrome or Chromium** installed and available in `PATH`

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

	pdf, err := c.ConvertHTML(context.Background(), html, nil)
	if err != nil {
		log.Fatal(err)
	}

	os.WriteFile("hello.pdf", pdf, 0o644)
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

pdf, err := c.ConvertHTML(ctx, html, page)
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
pdf, err := c.ConvertHTML(ctx, "<h1>Hello</h1>", page)

// From a local file
pdf, err := c.ConvertFile(ctx, "report.html", page)

// From a URL
pdf, err := c.ConvertURL(ctx, "https://example.com", page)
```

## Converter Options

Configure the `Converter` with functional options:

```go
c, err := htmlpdf.NewConverter(
	htmlpdf.WithTimeout(60 * time.Second),    // conversion timeout (default: 30s)
	htmlpdf.WithChromePath("/usr/bin/chromium"), // custom Chrome path
	htmlpdf.WithNoSandbox(),                    // required when running as root / Docker
)
```

## One-off Conversions

For single conversions where you don't need to reuse the browser, use the package-level functions:

```go
pdf, err := htmlpdf.ConvertHTML(ctx, html, page, htmlpdf.WithNoSandbox())
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

## How It Works

The library has a small surface area — around 250 lines of core code — built on two ideas:

1. **Chrome does the rendering.** Instead of reimplementing a layout engine in Go (which would inevitably lag behind real browser support), the library launches a headless Chrome instance using [`chromedp`](https://github.com/chromedp/chromedp) and talks to it over the Chrome DevTools Protocol. This gives you the exact same rendering fidelity as your browser.

2. **The browser stays alive.** A `Converter` starts Chrome once and reuses it. Each call to `ConvertHTML`/`ConvertURL`/`ConvertFile` opens a new tab, navigates to the content, calls Chrome's `Page.printToPDF` API, and closes the tab. This keeps subsequent conversions fast.

### File Layout

```
├── doc.go           # Package documentation
├── page.go          # PageSize, Orientation, Margin, PageConfig types
├── options.go       # Functional options (WithTimeout, WithChromePath, ...)
├── errors.go        # Sentinel errors
├── converter.go     # Converter struct, conversion methods, convenience functions
├── page_test.go     # Unit tests for page math (no Chrome needed)
├── converter_test.go # Integration tests (skipped if Chrome is not installed)
└── example_test.go  # Testable examples for go doc
```

### Dependencies

| Package                  | License | Purpose                          |
|--------------------------|---------|----------------------------------|
| `chromedp/chromedp`      | MIT     | Headless Chrome driver           |
| `chromedp/cdproto`       | MIT     | Chrome DevTools Protocol types   |

No paid services, no SaaS APIs, no CGo.

## Running in Docker

Chrome needs a few flags when running inside containers:

```go
c, err := htmlpdf.NewConverter(htmlpdf.WithNoSandbox())
```

A minimal Dockerfile:

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
