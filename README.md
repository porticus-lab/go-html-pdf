# go-html-pdf

Go library (`package htmlpdf`) with two complementary PDF capabilities under a **single import**:

| Capability | What it does |
|-----------|--------------|
| **HTML + CSS → PDF** | Converts modern HTML5/CSS3 to PDF via headless Chrome (CDP) |
| **PDF → text** | Extracts plain text from PDF files — pure Go, no external deps |

```bash
go get github.com/porticus-lab/go-html-pdf
```

```go
import htmlpdf "github.com/porticus-lab/go-html-pdf"
```

**Requirements**: Go 1.24+. Chrome/Chromium in PATH for the HTML→PDF side (or use `WithAutoDownload()`). No requirements for the PDF→text side.

---

## HTML + CSS to PDF

The library renders HTML through a headless Chrome instance via the [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/). Every CSS feature your browser supports works out of the box — flexbox, grid, custom properties, `@media print`, web fonts, gradients.

### Quick Start

```go
package main

import (
    "context"
    "log"

    htmlpdf "github.com/porticus-lab/go-html-pdf"
)

func main() {
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
<body><h1>Hello, PDF!</h1></body>
</html>`

    res, err := c.ConvertHTML(context.Background(), html, nil)
    if err != nil {
        log.Fatal(err)
    }
    res.WriteToFile("hello.pdf", 0o644)
}
```

Pass `nil` as page config to get defaults: **A4, portrait, 1 cm margins, scale 1.0, backgrounds on**.

### Input Sources

```go
res, err := c.ConvertHTML(ctx, "<h1>Hello</h1>", page)
res, err  = c.ConvertFile(ctx, "report.html", page)
res, err  = c.ConvertURL(ctx, "https://example.com", page)
```

### Page Configuration

```go
page := &htmlpdf.PageConfig{
    Size:            htmlpdf.Letter,
    Orientation:     htmlpdf.Landscape,
    Margin:          htmlpdf.Margin{Top: 2, Right: 2.5, Bottom: 2, Left: 2.5},
    Scale:           0.9,
    PrintBackground: true,
}
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
| `Margin` | `Margin` | 1 cm all | Top/Right/Bottom/Left in centimetres |
| `Scale` | `float64` | `1.0` | Content scale (0.1–2.0) |
| `PrintBackground` | `bool` | `true` | Include background colors/images |
| `DisplayHeaderFooter` | `bool` | `false` | Enable header/footer templates |
| `HeaderTemplate` | `string` | `""` | HTML header template |
| `FooterTemplate` | `string` | `""` | HTML footer template |
| `PreferCSSPageSize` | `bool` | `false` | Honor CSS `@page` size |

### Converter Options

```go
c, err := htmlpdf.NewConverter(
    htmlpdf.WithTimeout(60 * time.Second),      // default: 30s
    htmlpdf.WithChromePath("/usr/bin/chromium"), // custom browser path
    htmlpdf.WithNoSandbox(),                    // required in Docker / root
    htmlpdf.WithAutoDownload(),                 // auto-download Chromium
)
```

`WithAutoDownload()` caches Chromium in `~/.cache/rod/browser` (Unix) or `%APPDATA%\rod\browser` (Windows). First run: 10–30 s; subsequent: ~1 ms overhead. Ignored when `WithChromePath` is set.

### One-off Conversions

```go
// No need to create a Converter explicitly
res, err := htmlpdf.ConvertHTML(ctx, html, page, htmlpdf.WithNoSandbox())
res, err  = htmlpdf.ConvertURL(ctx, "https://example.com", page)
res, err  = htmlpdf.ConvertFile(ctx, "report.html", page)
```

For repeated conversions prefer `NewConverter` — it reuses the browser process and is significantly faster.

### Headers and Footers

```go
page := &htmlpdf.PageConfig{
    DisplayHeaderFooter: true,
    HeaderTemplate: `<div style="font-size:10px;text-align:center;width:100%">
        <span class="title"></span></div>`,
    FooterTemplate: `<div style="font-size:10px;text-align:center;width:100%">
        Page <span class="pageNumber"></span> of <span class="totalPages"></span></div>`,
}
```

Available template classes: `date`, `title`, `url`, `pageNumber`, `totalPages`.

### Result Object

```go
res.Bytes()                       // []byte
res.Base64()                      // string (RFC 4648)
res.Reader()                      // *bytes.Reader — io.Reader + io.Seeker
res.WriteTo(w)                    // io.WriterTo
res.WriteToFile("out.pdf", 0o644)
res.Len()                         // int
```

### Cloud Storage Upload

```go
// GCP Cloud Storage
w := client.Bucket(bucket).Object(object).NewWriter(ctx)
w.ContentType = "application/pdf"
res.WriteTo(w); w.Close()

// AWS S3
client.PutObject(ctx, &s3.PutObjectInput{
    Bucket: &bucket, Key: &key,
    Body: res.Reader(), ContentType: aws.String("application/pdf"),
})

// JSON API
json.NewEncoder(w).Encode(map[string]string{"pdf": res.Base64()})
```

### Running in Docker

Chrome requires `--no-sandbox` inside containers. Always pair `WithNoSandbox()` with any Docker deployment:

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

#### Option A — Auto-download (smallest image, fastest to set up)

No need to install Chromium in the image. The library downloads it on first run and caches it in `~/.cache/rod/browser`. Only the shared libraries Chrome needs at runtime are required:

```dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . ./
RUN go build -o server

FROM alpine:latest

# Runtime shared libraries for headless Chromium (no browser package needed)
RUN apk add --no-cache \
    nss atk at-spi2-core cups-libs libdrm \
    libxcomposite libxdamage libxrandr mesa-gbm pango \
    cairo alsa-lib libxshmfence font-noto

COPY --from=builder /app/server /app/server
CMD ["/app/server"]
```

```go
c, err := htmlpdf.NewConverter(
    htmlpdf.WithAutoDownload(),
    htmlpdf.WithNoSandbox(),
)
```

#### Option B — System Chromium (larger image, no first-run download)

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

```go
c, err := htmlpdf.NewConverter(
    htmlpdf.WithChromePath("/usr/bin/chromium-browser"),
    htmlpdf.WithNoSandbox(),
)
```

---

## PDF to Text

Pure-Go PDF text extraction. Go port of [zpdf](https://github.com/Lulzx/zpdf) — no CGo, no external dependencies.

### Quick Start

```go
doc, err := htmlpdf.Open("document.pdf")
if err != nil {
    log.Fatal(err)
}

ext := htmlpdf.NewExtractor(doc)
pages, err := ext.ExtractAll()
for i, text := range pages {
    fmt.Printf("=== Page %d ===\n%s\n", i+1, text)
}
```

### Opening Documents

```go
doc, err := htmlpdf.Open("report.pdf")        // from disk
doc, err  = htmlpdf.Load(data)               // from []byte (embed.FS, HTTP body, …)
```

### Text Extraction

```go
ext := htmlpdf.NewExtractor(doc)

pages, err := ext.ExtractAll()              // []string — one per page
text, err  := ext.ExtractPage(0)           // single page, 0-indexed
text, err  = ext.ExtractPageDict(pageDict) // from a Dict directly
```

**How extraction works:**
1. Font resources are resolved and encoding tables built per page.
2. Content streams are decompressed and parsed.
3. Text operators (`Tj`, `TJ`, `'`, `"`) emit positioned spans.
4. Spans are grouped into lines by Y coordinate (±50 % of average font size).
5. Lines sorted top-to-bottom; spans left-to-right; spaces inserted when gap > 30 % of font size.

### Document API

```go
doc.Version()                  // string — e.g. "1.7"
doc.Pages()                    // ([]Dict, error)
doc.GetPageInfo(page)          // PageInfo{Width, Height float64; Rotation int}
doc.ContentStreams(page)       // ([]byte, error) — decompressed content
doc.PageFonts(page)            // (map[string]*Object, error)
doc.Catalog()                  // (Dict, error)
doc.ResolveRef(ref Reference)  // (*Object, error)
doc.Resolve(obj *Object)       // (*Object, error)
```

`PageInfo`: `Width` and `Height` in points (1 pt = 1/72 inch), `Rotation` in degrees (0, 90, 180, 270).

### Decompression

```go
raw, err := htmlpdf.DecompressStream(streamObj.Dict, streamObj.Stream)
```

| Filter | Aliases | Notes |
|--------|---------|-------|
| `FlateDecode` | `Fl` | zlib + PNG predictors + TIFF predictor |
| `ASCII85Decode` | `A85` | |
| `ASCIIHexDecode` | `AHx` | |
| `LZWDecode` | `LZW` | MSB-first, litWidth=8 |
| `RunLengthDecode` | `RL` | PackBits |
| `DCTDecode`, `CCITTFaxDecode`, `JBIG2Decode`, `JPXDecode`, `Crypt` | | Passed through as-is |

256 MB limit on decompressed output (DoS guard).

### Font Encoding

```go
enc := htmlpdf.NewFontEncoding(fontObj)
text := enc.Decode(rawBytes)
```

Decoding priority: **ToUnicode CMap** > **Encoding dict** > **Named encoding** > Default.

Named encodings: `WinAnsiEncoding`, `MacRomanEncoding`, `StandardEncoding`, `PDFDocEncoding`. `/Differences` resolved via Adobe Glyph List (~300 glyph names). CID/Type0 fonts use multi-byte CMap lookup.

### Low-level Object Model

```go
p := htmlpdf.NewParser(data, 0)
obj, err := p.ParseObject()
p.Pos(); p.SetPos(n)
```

| Type | Description |
|------|-------------|
| `Object` | Tagged union for any PDF object |
| `ObjectType` | `ObjNull`, `ObjBool`, `ObjInt`, `ObjFloat`, `ObjString`, `ObjName`, `ObjArray`, `ObjDict`, `ObjStream`, `ObjRef` |
| `Reference` | `{Number int, Gen int}` |
| `Dict` | `map[string]*Object` — helpers: `GetInt`, `GetName`, `GetArray`, `GetDict` |
| `PageInfo` | `{Width, Height float64; Rotation int}` |

---

## Chrome Dependencies

The HTML→PDF side launches a headless Chromium process. Chromium needs several system libraries that are absent in minimal base images. The PDF→text side has **no system dependencies** — it uses only the Go standard library.

### Alpine Linux

```sh
# Shared libraries only (use with WithAutoDownload)
apk add --no-cache \
    nss atk at-spi2-core cups-libs libdrm \
    libxcomposite libxdamage libxrandr mesa-gbm pango \
    cairo alsa-lib libxshmfence font-noto

# Or install the full browser package (pulls all deps automatically)
apk add --no-cache chromium
```

### Debian / Ubuntu

```sh
# Shared libraries only (use with WithAutoDownload)
apt-get install -y --no-install-recommends \
    libnss3 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 \
    libxcomposite1 libxdamage1 libxrandr2 libgbm1 libpango-1.0-0 \
    libcairo2 libasound2 libxshmfence1 fonts-noto

# Or install the full browser package
apt-get install -y --no-install-recommends chromium
```

### macOS

No extra steps. Chrome/Chromium from the standard `.app` install is found automatically, or use `WithAutoDownload()`.

### Windows

No extra steps. Chrome is found via standard registry paths, or use `WithAutoDownload()`.

> **Tip:** `WithAutoDownload()` is the easiest cross-platform option — it downloads a pinned Chromium build the first time and reuses it on every subsequent run (~1 ms overhead).

---

## Architecture

```
├── doc.go            # Package documentation
├── page.go           # PageSize, Orientation, Margin, PageConfig
├── options.go        # Functional options (WithTimeout, WithChromePath, …)
├── errors.go         # Sentinel errors (ErrClosed)
├── result.go         # Result type (Bytes, Base64, Reader, WriteTo, WriteToFile)
├── browser.go        # Chromium auto-download via go-rod/rod/lib/launcher
├── converter.go      # Converter + package-level convenience functions
│
├── parser.go         # Recursive-descent PDF object parser
├── document.go       # Document loading, XRef, page tree, object resolution
├── decompress.go     # Stream filters: FlateDecode, ASCII85, LZW, RunLength
├── encoding.go       # Font encoding tables + ToUnicode CMap parser
└── extractor.go      # Content-stream text extraction + line assembly
```

### Dependencies

| Package | License | Purpose |
|---------|---------|---------|
| `chromedp/chromedp` | MIT | Headless Chrome driver |
| `chromedp/cdproto` | MIT | Chrome DevTools Protocol types |
| `go-rod/rod` | MIT | Chromium auto-download |

The PDF→text side uses only the Go standard library.

## License

Apache 2.0 — see [LICENSE](LICENSE).
