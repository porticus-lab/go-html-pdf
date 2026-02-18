# go-html-pdf

Go module with two complementary PDF libraries:

| Package | Import path | What it does |
|---------|-------------|--------------|
| `htmlpdf` | `github.com/porticus-lab/go-html-pdf` | **HTML + CSS → PDF** via headless Chrome (full CSS3, flexbox, grid, web fonts) |
| `pdf` | `github.com/porticus-lab/go-html-pdf/pdf` | **PDF → text** extraction, pure Go, no external dependencies |

---

## Part 1 — HTML + CSS to PDF (`htmlpdf`)

Fast, open-source Go library that converts modern HTML + CSS into PDF documents. It renders pages through a headless Chrome browser via the [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/), so every CSS feature your browser supports — flexbox, grid, custom properties, gradients, `@media print`, web fonts — works out of the box.

### Requirements

- **Go 1.24+**
- **Chrome or Chromium** installed and available in `PATH`, **or** use `WithAutoDownload()` to let the library fetch one automatically

### Install

```bash
go get github.com/porticus-lab/go-html-pdf
```

### Quick Start

```go
package main

import (
    "context"
    "log"

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

### Input Sources

```go
// From an HTML string
res, err := c.ConvertHTML(ctx, "<h1>Hello</h1>", page)

// From a local file
res, err := c.ConvertFile(ctx, "report.html", page)

// From a URL
res, err := c.ConvertURL(ctx, "https://example.com", page)
```

### Page Configuration

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

#### Available page sizes

| Name | Dimensions (cm) |
|------|-----------------|
| `A3` | 29.7 × 42.0 |
| `A4` | 21.0 × 29.7 |
| `A5` | 14.8 × 21.0 |
| `Letter` | 21.59 × 27.94 |
| `Legal` | 21.59 × 35.56 |
| `Tabloid` | 27.94 × 43.18 |

#### PageConfig fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Size` | `PageSize` | `A4` | Paper dimensions |
| `Orientation` | `Orientation` | `Portrait` | `Portrait` or `Landscape` |
| `Margin` | `Margin` | 1 cm all | Top, Right, Bottom, Left in centimeters |
| `Scale` | `float64` | `1.0` | Content scale factor (0.1–2.0) |
| `PrintBackground` | `bool` | `true` | Include background colors and images |
| `DisplayHeaderFooter` | `bool` | `false` | Enable header/footer templates |
| `HeaderTemplate` | `string` | `""` | HTML template for the page header |
| `FooterTemplate` | `string` | `""` | HTML template for the page footer |
| `PreferCSSPageSize` | `bool` | `false` | Prefer CSS `@page` size over `Size` |

### Converter Options

```go
c, err := htmlpdf.NewConverter(
    htmlpdf.WithTimeout(60 * time.Second),      // conversion timeout (default: 30s)
    htmlpdf.WithChromePath("/usr/bin/chromium"), // custom Chrome path
    htmlpdf.WithNoSandbox(),                    // required when running as root / Docker
    htmlpdf.WithAutoDownload(),                 // auto-download Chromium if not installed
)
```

#### Auto-download

If Chrome is not installed, `WithAutoDownload()` downloads and caches a compatible Chromium binary on first run:

```go
c, err := htmlpdf.NewConverter(htmlpdf.WithAutoDownload())
```

Binary is stored in `~/.cache/rod/browser` (Unix) or `%APPDATA%\rod\browser` (Windows) and reused on subsequent calls. First run may take 10–30 s; after that the check adds ~1 ms.

### One-off Conversions

For single conversions without reusing the browser:

```go
res, err := htmlpdf.ConvertHTML(ctx, html, page, htmlpdf.WithNoSandbox())
res, err := htmlpdf.ConvertURL(ctx, "https://example.com", page)
res, err := htmlpdf.ConvertFile(ctx, "report.html", page)
```

For repeated conversions, prefer `NewConverter` — it reuses the browser process and is significantly faster.

### Headers and Footers

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

### Result Object

Every conversion returns a `*Result`:

```go
res.Bytes()                       // []byte — raw PDF content
res.Base64()                      // string — base64-encoded (RFC 4648)
res.Reader()                      // *bytes.Reader — io.Reader + io.Seeker
res.WriteTo(w)                    // writes to any io.Writer
res.WriteToFile("out.pdf", 0o644) // writes directly to disk
res.Len()                         // int — size in bytes
```

### Cloud Storage Upload

```go
// GCP Cloud Storage
w := client.Bucket(bucket).Object(object).NewWriter(ctx)
w.ContentType = "application/pdf"
res.WriteTo(w)
w.Close()

// AWS S3
client.PutObject(ctx, &s3.PutObjectInput{
    Bucket:      &bucket,
    Key:         &key,
    Body:        res.Reader(),
    ContentType: aws.String("application/pdf"),
})

// JSON API (base64)
json.NewEncoder(w).Encode(map[string]string{"pdf": res.Base64()})
```

### Running in Docker

Chrome needs `--no-sandbox` inside containers:

```go
c, err := htmlpdf.NewConverter(
    htmlpdf.WithAutoDownload(),
    htmlpdf.WithNoSandbox(),
)
```

#### Dockerfile — auto-download (smallest image)

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . ./
RUN go build -o server

FROM alpine:latest
RUN apk add --no-cache \
    nss atk at-spi2-core cups-libs libdrm \
    libxcomposite libxdamage libxrandr mesa-gbm pango \
    cairo alsa-lib libxshmfence font-noto
COPY --from=builder /app/server /app/server
CMD ["/app/server"]
```

#### Dockerfile — system Chromium

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . ./
RUN go build -o server

FROM alpine:latest
RUN apk add --no-cache chromium
COPY --from=builder /app/server /app/server
CMD ["/app/server"]
```

### How It Works

1. **Chrome does the rendering.** The library launches a headless Chrome instance via [`chromedp`](https://github.com/chromedp/chromedp) and talks to it over the Chrome DevTools Protocol, giving you exact browser rendering fidelity.
2. **The browser stays alive.** A `Converter` starts Chrome once and reuses it. Each conversion opens a new tab, navigates, calls Chrome's `Page.printToPDF`, and closes the tab.

#### Dependencies

| Package | License | Purpose |
|---------|---------|---------|
| `chromedp/chromedp` | MIT | Headless Chrome driver |
| `chromedp/cdproto` | MIT | Chrome DevTools Protocol types |
| `go-rod/rod` | MIT | Chromium auto-download (launcher) |

#### File layout

```
├── doc.go            # Package documentation
├── page.go           # PageSize, Orientation, Margin, PageConfig
├── options.go        # Functional options (WithTimeout, WithChromePath, …)
├── errors.go         # Sentinel errors (ErrClosed)
├── result.go         # Result type
├── browser.go        # Auto-download via go-rod/rod/lib/launcher
├── converter.go      # Converter + package-level convenience functions
├── page_test.go      # Unit tests (no Chrome needed)
├── result_test.go    # Unit tests (no Chrome needed)
├── converter_test.go # Integration tests (skipped if Chrome not in PATH)
└── example_test.go   # Testable examples for go doc
```

---

## Part 2 — PDF to Text (`pdf`)

Pure-Go PDF text extraction library. Go port of [zpdf](https://github.com/Lulzx/zpdf), with no external dependencies.

### Install

```bash
go get github.com/porticus-lab/go-html-pdf/pdf
```

### Quick Start

```go
import "github.com/porticus-lab/go-html-pdf/pdf"

doc, err := pdf.Open("document.pdf")
if err != nil {
    log.Fatal(err)
}

ext := pdf.NewExtractor(doc)
pages, err := ext.ExtractAll()
if err != nil {
    log.Fatal(err)
}
for i, text := range pages {
    fmt.Printf("=== Page %d ===\n%s\n", i+1, text)
}
```

### Opening documents

```go
// From disk
doc, err := pdf.Open("report.pdf")

// From raw bytes (HTTP response, embed.FS, …)
data, _ := io.ReadAll(resp.Body)
doc, err := pdf.Load(data)
```

### Document methods

```go
doc.Version()                 // string — e.g. "1.7"
doc.Pages()                   // ([]Dict, error) — all page dicts in order
doc.GetPageInfo(page)         // PageInfo{Width, Height float64; Rotation int}
doc.ContentStreams(page)      // ([]byte, error) — decompressed content stream
doc.PageFonts(page)           // (map[string]*Object, error) — font resources
doc.Catalog()                 // (Dict, error) — document catalog
doc.ResolveRef(ref Reference) // (*Object, error) — follow indirect reference
doc.Resolve(obj *Object)      // (*Object, error) — resolve if ref, else no-op
```

`PageInfo` fields: `Width` and `Height` in points (1 pt = 1/72 inch), `Rotation` in degrees (0, 90, 180, 270).

### Text extraction

```go
ext := pdf.NewExtractor(doc)

// All pages → []string (one per page)
texts, err := ext.ExtractAll()

// Single page, 0-indexed
text, err := ext.ExtractPage(0)

// From a page Dict directly
text, err := ext.ExtractPageDict(pages[2])
```

**How it works:**
1. Font resources are loaded and encoding tables built per page.
2. Content streams are decompressed and parsed.
3. Text operators (`Tj`, `TJ`, `'`, `"`) emit positioned spans.
4. Spans are grouped into lines by Y coordinate (±50 % of average font size).
5. Lines sorted top-to-bottom; spans left-to-right with space insertion when gap > 30 % of font size.

### Decompression — `pdf.DecompressStream`

```go
raw, err := pdf.DecompressStream(streamObj.Dict, streamObj.Stream)
```

| Filter | Aliases | Notes |
|--------|---------|-------|
| `FlateDecode` | `Fl` | zlib + PNG predictors (Sub, Up, Average, Paeth) + TIFF predictor |
| `ASCII85Decode` | `A85` | |
| `ASCIIHexDecode` | `AHx` | |
| `LZWDecode` | `LZW` | MSB-first, litWidth=8 |
| `RunLengthDecode` | `RL` | PackBits |
| `DCTDecode`, `CCITTFaxDecode`, `JBIG2Decode`, `JPXDecode`, `Crypt` | | Passed through as-is |

256 MB limit on decompressed output (DoS guard).

### Font encoding — `pdf.NewFontEncoding`

```go
enc := pdf.NewFontEncoding(fontObj)
text := enc.Decode(rawBytes)
```

Decoding priority: **ToUnicode CMap** > **Encoding dict** (`/BaseEncoding` + `/Differences`) > **Named encoding** > Default (WinAnsi or Standard).

Named encodings: `WinAnsiEncoding`, `MacRomanEncoding`, `StandardEncoding`, `PDFDocEncoding`. `/Differences` resolved via Adobe Glyph List (~300 glyph names). CID/Type0 fonts use multi-byte CMap lookup.

### Low-level object model

| Type | Description |
|------|-------------|
| `Object` | Tagged union for any PDF object |
| `ObjectType` | `ObjNull`, `ObjBool`, `ObjInt`, `ObjFloat`, `ObjString`, `ObjName`, `ObjArray`, `ObjDict`, `ObjStream`, `ObjRef` |
| `Reference` | `{Number int, Gen int}` |
| `Dict` | `map[string]*Object` with `GetInt`, `GetName`, `GetArray`, `GetDict` helpers |
| `XRefEntry` | Cross-reference table entry |
| `PageInfo` | `{Width, Height float64; Rotation int}` |

`Object` fields: `Type`, `Bool`, `Int`, `Float`, `Str` (`[]byte`), `Name`, `Array` (`[]*Object`), `Dict`, `Stream` (`[]byte`, raw), `Ref`.

### Parser

```go
p := pdf.NewParser(data, 0)
obj, err := p.ParseObject() // parses one PDF object
p.Pos()                     // current byte position
p.SetPos(n)                 // seek to position
```

Recursive-descent parser for the full PDF object syntax. Nesting capped at depth 100.

#### File layout

```
pdf/
├── parser.go        # Recursive-descent PDF object parser
├── document.go      # Document, XRef loading, page tree, object resolution
├── decompress.go    # Stream decompression (FlateDecode, ASCII85, LZW, …)
├── encoding.go      # Font encoding tables + ToUnicode CMap parser
└── extractor.go     # Content-stream text extraction + line assembly
```

---

## Requirements

| Package | Go | External deps |
|---------|----|---------------|
| `htmlpdf` (root) | 1.24+ | chromedp, cdproto, go-rod/rod |
| `pdf` (subpackage) | 1.21+ | none (stdlib only) |

## License

Apache 2.0 — see [LICENSE](LICENSE).
