# go-html-pdf

Pure-Go PDF text extraction library. Go port of [zpdf](https://github.com/Lulzx/zpdf), with no external dependencies.

## Installation

```bash
go get github.com/porticus-lab/go-html-pdf/pdf
```

## Quick start

```go
import "github.com/porticus-lab/go-html-pdf/pdf"

// Open from disk
doc, err := pdf.Open("document.pdf")
if err != nil {
    log.Fatal(err)
}

// Extract text from every page
ext := pdf.NewExtractor(doc)
pages, err := ext.ExtractAll()
if err != nil {
    log.Fatal(err)
}
for i, text := range pages {
    fmt.Printf("=== Page %d ===\n%s\n", i+1, text)
}
```

---

## Opening documents

### `pdf.Open`

```go
func Open(path string) (*Document, error)
```

Reads a PDF file from disk and returns a parsed `*Document`.

```go
doc, err := pdf.Open("report.pdf")
```

### `pdf.Load`

```go
func Load(data []byte) (*Document, error)
```

Parses a PDF from a raw byte slice (useful when reading from HTTP responses, embed.FS, etc.).

```go
data, _ := os.ReadFile("report.pdf")
doc, err := pdf.Load(data)

// Or from an http.Response body:
body, _ := io.ReadAll(resp.Body)
doc, err := pdf.Load(body)
```

---

## Document

### `doc.Version`

```go
func (doc *Document) Version() string
```

Returns the PDF version string from the file header (e.g. `"1.7"`).

```go
fmt.Println(doc.Version()) // "1.7"
```

### `doc.Pages`

```go
func (doc *Document) Pages() ([]Dict, error)
```

Returns all page dictionaries in document order by traversing the page tree.

```go
pages, err := doc.Pages()
fmt.Printf("%d pages\n", len(pages))
```

### `doc.GetPageInfo`

```go
func (doc *Document) GetPageInfo(page Dict) PageInfo
```

Returns width (pt), height (pt), and rotation (degrees) for a page.

```go
pages, _ := doc.Pages()
for i, page := range pages {
    info := doc.GetPageInfo(page)
    fmt.Printf("Page %d: %.0f x %.0f pt, %d°\n",
        i+1, info.Width, info.Height, info.Rotation)
}
```

`PageInfo` fields:

| Field | Type | Description |
|-------|------|-------------|
| `Width` | `float64` | Width in points (1 pt = 1/72 inch) |
| `Height` | `float64` | Height in points |
| `Rotation` | `int` | Clockwise rotation in degrees (0, 90, 180, 270) |

### `doc.ContentStreams`

```go
func (doc *Document) ContentStreams(page Dict) ([]byte, error)
```

Returns the combined, fully-decompressed content stream bytes for a page.
Useful for custom content-stream parsing beyond text extraction.

```go
pages, _ := doc.Pages()
raw, err := doc.ContentStreams(pages[0])
fmt.Printf("content stream: %d bytes\n", len(raw))
```

### `doc.PageFonts`

```go
func (doc *Document) PageFonts(page Dict) (map[string]*Object, error)
```

Returns the font resource objects for a page keyed by their resource name (e.g. `"F1"`, `"F2"`).

```go
pages, _ := doc.Pages()
fonts, err := doc.PageFonts(pages[0])
for name, obj := range fonts {
    subtype, _ := obj.Dict.GetName("Subtype")
    fmt.Printf("  %s: /%s\n", name, subtype)
}
```

### `doc.Catalog`

```go
func (doc *Document) Catalog() (Dict, error)
```

Returns the document catalog dictionary (the root of the PDF object tree).

```go
cat, err := doc.Catalog()
if pdfType, ok := cat.GetName("Type"); ok {
    fmt.Println(pdfType) // "Catalog"
}
```

### `doc.ResolveRef` / `doc.Resolve`

```go
func (doc *Document) ResolveRef(ref Reference) (*Object, error)
func (doc *Document) Resolve(obj *Object) (*Object, error)
```

Follow indirect references to their target objects.
`Resolve` is a convenience wrapper: if the object is not a reference it is returned as-is.

```go
// Resolve an indirect reference directly
obj, err := doc.ResolveRef(pdf.Reference{Number: 5, Gen: 0})

// Or resolve whatever type an Object holds
obj, err := doc.Resolve(someObj) // no-op if someObj.Type != ObjRef
```

---

## Text extraction

### `pdf.NewExtractor`

```go
func NewExtractor(doc *Document) *Extractor
```

Creates a text extractor bound to a document.

### `ext.ExtractAll`

```go
func (e *Extractor) ExtractAll() ([]string, error)
```

Extracts plain text from every page and returns one string per page.

```go
ext := pdf.NewExtractor(doc)
texts, err := ext.ExtractAll()
for i, t := range texts {
    fmt.Printf("--- page %d ---\n%s\n", i+1, t)
}
```

### `ext.ExtractPage`

```go
func (e *Extractor) ExtractPage(pageIndex int) (string, error)
```

Extracts plain text from a single page (0-indexed). Returns `""` without error if the index is out of range.

```go
text, err := ext.ExtractPage(0) // first page
```

### `ext.ExtractPageDict`

```go
func (e *Extractor) ExtractPageDict(page Dict) (string, error)
```

Extracts plain text from a page dictionary directly. Useful when you already have the `Dict` from `doc.Pages()`.

```go
pages, _ := doc.Pages()
text, err := ext.ExtractPageDict(pages[2])
```

#### How text extraction works

1. Font resources for the page are loaded and encoding tables are built.
2. Content streams are decompressed and fed to the content-stream parser.
3. Text operators (`Tj`, `TJ`, `'`, `"`) emit positioned `textSpan` values.
4. Spans are grouped into lines by Y coordinate (tolerance = 50 % of average font size).
5. Lines are sorted top-to-bottom (PDF Y=0 is at the bottom).
6. Spans within a line are sorted left-to-right; a space is inserted when the horizontal gap exceeds 30 % of the average font size.

---

## Decompression

### `pdf.DecompressStream`

```go
func DecompressStream(dict Dict, data []byte) ([]byte, error)
```

Decompresses a PDF stream given its dictionary and raw bytes. Handles filter chains (multiple filters applied in sequence).

```go
compressed := someStreamObject.Stream
raw, err := pdf.DecompressStream(someStreamObject.Dict, compressed)
```

Supported filters:

| Filter name | Aliases | Notes |
|-------------|---------|-------|
| `FlateDecode` | `Fl` | zlib/deflate with optional PNG predictors (Sub, Up, Average, Paeth) and TIFF predictor |
| `ASCII85Decode` | `A85` | Base-85 encoding |
| `ASCIIHexDecode` | `AHx` | Pairs of hexadecimal digits |
| `LZWDecode` | `LZW` | MSB-first LZW (TIFF order, litWidth=8) |
| `RunLengthDecode` | `RL` | PackBits run-length encoding |
| `DCTDecode` | `DCT` | JPEG — passed through as-is |
| `CCITTFaxDecode` | `CCF` | CCITT fax — passed through as-is |
| `JBIG2Decode` | — | JBIG2 — passed through as-is |
| `JPXDecode` | — | JPEG 2000 — passed through as-is |
| `Crypt` | — | Identity — passed through as-is |

A 256 MB limit is enforced on decompressed output to prevent DoS via unbounded memory allocation.

---

## Font encoding

### `pdf.NewFontEncoding`

```go
func NewFontEncoding(fontObj *Object) *FontEncoding
```

Builds a `FontEncoding` from a PDF font object. The decoding priority is:

1. **ToUnicode CMap** (`beginbfchar` / `beginbfrange`) — highest priority
2. **Encoding dictionary** (`/Encoding` dict with `/BaseEncoding` + `/Differences`)
3. **Named encoding** (`/Encoding` name)
4. **Default** — WinAnsiEncoding for most fonts, StandardEncoding for Type1/MMType1

Supported named encodings:

| Name | Description |
|------|-------------|
| `WinAnsiEncoding` | Windows-1252 |
| `MacRomanEncoding` | Mac OS Roman |
| `StandardEncoding` | PostScript Standard Encoding |
| `PDFDocEncoding` | PDF document encoding |

`/Differences` arrays are resolved via the **Adobe Glyph List** (covers ~300 common glyph names: letters, digits, punctuation, accented characters, typographic symbols).

CID fonts (Type0/composite) are handled via multi-byte CMap lookups.

### `enc.Decode`

```go
func (e *FontEncoding) Decode(data []byte) string
```

Converts a raw PDF text string (byte slice from a `Tj`/`TJ` operand) to a UTF-8 string using the built encoding table.

```go
enc := pdf.NewFontEncoding(fontObj)
text := enc.Decode([]byte{0x48, 0x65, 0x6C, 0x6C, 0x6F})
fmt.Println(text) // "Hello"
```

---

## Low-level object model

### Types

| Type | Description |
|------|-------------|
| `Object` | Any PDF object (tagged union) |
| `ObjectType` | Enum: `ObjNull`, `ObjBool`, `ObjInt`, `ObjFloat`, `ObjString`, `ObjName`, `ObjArray`, `ObjDict`, `ObjStream`, `ObjRef` |
| `Reference` | Indirect object reference `{Number int, Gen int}` |
| `Dict` | `map[string]*Object` with helper methods |
| `XRefEntry` | Cross-reference table entry |
| `PageInfo` | `{Width, Height float64; Rotation int}` |

`Object` fields:

| Field | Type | Used for |
|-------|------|----------|
| `Type` | `ObjectType` | Discriminant |
| `Bool` | `bool` | `ObjBool` |
| `Int` | `int64` | `ObjInt` |
| `Float` | `float64` | `ObjFloat` |
| `Str` | `[]byte` | `ObjString` |
| `Name` | `string` | `ObjName` |
| `Array` | `[]*Object` | `ObjArray` |
| `Dict` | `Dict` | `ObjDict`, `ObjStream` |
| `Stream` | `[]byte` | `ObjStream` (raw, not decompressed) |
| `Ref` | `Reference` | `ObjRef` |

### Dict helper methods

```go
val, ok := dict.GetInt("Length")    // int64
val, ok := dict.GetName("Type")     // string
val, ok := dict.GetArray("Filter")  // []*Object (single obj treated as 1-elem array)
val, ok := dict.GetDict("Resources") // Dict
```

### Parser

```go
func NewParser(data []byte, pos int) *Parser
func (p *Parser) ParseObject() (*Object, error)
func (p *Parser) Pos() int
func (p *Parser) SetPos(pos int)
```

A recursive-descent parser for the PDF object syntax. Used internally to parse file structures, but exposed for advanced use cases (e.g. custom content-stream operators).

```go
p := pdf.NewParser(data, 0)
obj, err := p.ParseObject() // parses one PDF object at position 0
```

Nesting is capped at depth 100 to prevent stack overflow on malformed files.

---

## Features summary

- **PDF parsing** — full object model: null, bool, int, float, literal strings (with octal/escape sequences), hex strings, names (with `#XX` escapes), arrays, dicts, streams, indirect references
- **XRef** — traditional cross-reference tables and cross-reference streams (PDF 1.5+), compressed object streams, linearized/hybrid PDFs via chained `Prev` pointers
- **Decompression** — FlateDecode (zlib + PNG predictors Sub/Up/Average/Paeth + TIFF predictor), ASCII85, ASCIIHex, LZW, RunLength; 256 MB DoS guard; image filters passed through
- **Font encoding** — WinAnsiEncoding, MacRomanEncoding, StandardEncoding, PDFDocEncoding; ToUnicode CMap (`beginbfchar` / `beginbfrange`); `/Differences` with Adobe Glyph List; CID/composite font (Type0) multi-byte lookup
- **Text extraction** — `Tj`, `TJ`, `'`, `"` content-stream operators; text state tracking (Tf, Tc, Tw, TL, Td, TD, Tm, T\*); positional line grouping with smart space insertion; whitespace normalisation

## Requirements

Go 1.21+. No external dependencies.

## License

Apache 2.0 — see [LICENSE](LICENSE).
