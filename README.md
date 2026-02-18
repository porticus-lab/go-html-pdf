# go-html-pdf

Pure-Go PDF text extraction library. Go port of [zpdf](https://github.com/Lulzx/zpdf), with no external dependencies.

## Installation

```bash
go get github.com/porticus-lab/go-html-pdf/pdf
```

## Usage

```go
import "github.com/porticus-lab/go-html-pdf/pdf"
```

### Extract text from all pages

```go
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

### Extract a specific page

```go
doc, err := pdf.Open("document.pdf")
if err != nil {
    log.Fatal(err)
}

ext := pdf.NewExtractor(doc)
text, err := ext.ExtractPage(0) // 0-indexed
if err != nil {
    log.Fatal(err)
}
fmt.Println(text)
```

### Load from bytes

```go
data, _ := os.ReadFile("document.pdf")

doc, err := pdf.Load(data)
if err != nil {
    log.Fatal(err)
}
```

### Page info (dimensions, rotation)

```go
doc, err := pdf.Open("document.pdf")
if err != nil {
    log.Fatal(err)
}

pages, _ := doc.Pages()
for i, page := range pages {
    info := doc.GetPageInfo(page)
    fmt.Printf("Page %d: %.0f x %.0f pt, rotation %d°\n",
        i+1, info.Width, info.Height, info.Rotation)
}
```

## API

### Opening documents

| Function | Description |
|----------|-------------|
| `pdf.Open(path string) (*Document, error)` | Open a PDF from disk |
| `pdf.Load(data []byte) (*Document, error)` | Parse a PDF from raw bytes |

### Document methods

| Method | Description |
|--------|-------------|
| `doc.Pages() ([]Dict, error)` | Return all page dictionaries in order |
| `doc.GetPageInfo(page Dict) PageInfo` | Width, height, and rotation for a page |
| `doc.Version() string` | PDF version string (e.g. `"1.7"`) |
| `doc.ContentStreams(page Dict) ([]byte, error)` | Raw decompressed content stream for a page |
| `doc.PageFonts(page Dict) (map[string]*Object, error)` | Font resource objects for a page |

### Extractor

| Method | Description |
|--------|-------------|
| `pdf.NewExtractor(doc *Document) *Extractor` | Create a text extractor |
| `ext.ExtractPage(index int) (string, error)` | Extract text from one page (0-indexed) |
| `ext.ExtractAll() ([]string, error)` | Extract text from every page |
| `ext.ExtractPageDict(page Dict) (string, error)` | Extract text from a page dictionary |

## Features

- **PDF parsing** — full object model: null, bool, int, float, strings, names, arrays, dicts, streams, indirect references
- **XRef** — traditional cross-reference tables and cross-reference streams (PDF 1.5+), compressed object streams
- **Decompression** — FlateDecode (zlib + PNG/TIFF predictors), ASCII85, ASCIIHex, LZW, RunLength; 256 MB DoS guard
- **Font encoding** — WinAnsiEncoding, MacRomanEncoding, StandardEncoding, PDFDocEncoding; ToUnicode CMap (`beginbfchar` / `beginbfrange`); Adobe Glyph List for `/Differences`
- **Text extraction** — `Tj`, `TJ`, `'`, `"` content-stream operators; positional line grouping with smart space insertion

## Requirements

Go 1.21+. No external dependencies.

## License

Apache 2.0 — see [LICENSE](LICENSE).
