# CLAUDE.md — go-html-pdf

## Project Overview

Single Go library (`package htmlpdf`) with two complementary PDF capabilities under one import path:

| Capability | What it does |
|-----------|--------------|
| HTML + CSS → PDF | Converts HTML5/CSS3 to PDF via headless Chrome (CDP) |
| PDF → text | Extracts plain text from PDFs — pure Go, no external deps |

**Module**: `github.com/porticus-lab/go-html-pdf`
**Package**: `htmlpdf`
**Import**: `import htmlpdf "github.com/porticus-lab/go-html-pdf"`
**Go**: 1.24+
**License**: Apache 2.0

---

## Architecture

| File | Purpose |
|------|---------|
| `doc.go` | Package-level documentation |
| `page.go` | `PageSize`, `Orientation`, `Margin`, `PageConfig`, `DefaultPageConfig` |
| `options.go` | Functional options: `WithTimeout`, `WithChromePath`, `WithNoSandbox`, `WithAutoDownload` |
| `errors.go` | Sentinel errors (`ErrClosed`) |
| `result.go` | `Result`: `Bytes`, `Base64`, `Reader`, `WriteTo`, `WriteToFile`, `Len` |
| `browser.go` | Chromium auto-download via `go-rod/rod/lib/launcher` |
| `converter.go` | `Converter` struct + package-level convenience functions |
| `parser.go` | Recursive-descent PDF object parser (all object types) |
| `document.go` | Document loading, XRef table/stream, object resolution, page tree |
| `decompress.go` | Stream filters: FlateDecode, ASCII85, ASCIIHex, LZW, RunLength |
| `encoding.go` | Font encoding: WinAnsi, MacRoman, ToUnicode CMap, Adobe Glyph List |
| `extractor.go` | Content-stream text extraction, positional line assembly |

### Test files

| File | Package | Notes |
|------|---------|-------|
| `page_test.go` | `htmlpdf` | Unit tests — no Chrome required |
| `result_test.go` | `htmlpdf` | Unit tests — no Chrome required |
| `extractor_test.go` | `htmlpdf` | Unit tests — no Chrome required |
| `converter_test.go` | `htmlpdf_test` | Integration tests — skipped if Chrome not in PATH |
| `example_test.go` | `htmlpdf_test` | Testable examples for `go doc` |

---

## Key Design Decisions

### HTML → PDF side

- **Units**: Public API in centimetres; internally converted to inches for Chrome's `printToPDF`
- **Browser reuse**: `Converter` keeps a single Chrome process alive; each conversion opens a new tab
- **Thread safety**: `sync.Mutex` guards closed state; safe for concurrent use
- **Functional options**: `Option func(*converterConfig)` with `With*` constructors
- **Nil-safe PageConfig**: `nil` or zero-value resolves to defaults (A4, portrait, 1 cm, scale 1.0)
- **Result type**: `*Result` with `Bytes()`, `Base64()`, `Reader()`, `WriteTo()`, `WriteToFile()`, `Len()` — designed for cloud storage uploads (GCP, S3)
- **Auto-download**: `WithAutoDownload()` uses `go-rod/rod/lib/launcher`; ignored when `WithChromePath` is set. Lookup order: explicit path > auto-download > system PATH
- **Error prefix**: `fmt.Errorf("htmlpdf: ...: %w", err)`

### PDF → text side

- **Pure Go**: no CGo, no external dependencies — only stdlib
- **Full PDF object model**: null, bool, int, float, literal/hex strings, names, arrays, dicts, streams, indirect references
- **XRef**: traditional tables + PDF 1.5+ cross-reference streams + compressed object streams
- **Decompression guard**: 256 MB limit on decompressed output
- **Font decoding priority**: ToUnicode CMap > Encoding dict > Named encoding > Default

---

## Dependencies

| Package | License | Purpose |
|---------|---------|---------|
| `chromedp/chromedp` | MIT | Headless Chrome driver |
| `chromedp/cdproto` | MIT | Chrome DevTools Protocol types |
| `go-rod/rod` | MIT | Chromium auto-download |

PDF→text uses stdlib only. No paid dependencies allowed.

---

## Commands

```bash
# Build
go build ./...

# All tests
go test ./...

# Unit tests only (no Chrome required)
go test -run 'TestCm|TestDefault|TestUniform|TestPageConfig|TestPaper|TestMargin|TestResult_|TestExtract|TestParser|TestAscii|TestRunLength|TestWinAnsi|TestDecode|TestMultiple|TestPageInfo' ./...

# Verbose
go test -v ./...

# Vet
go vet ./...

# Tidy
go mod tidy
```

## Conventions

- Exported names documented; lowercase package name; no underscores in file names
- `htmlpdf` errors: always `fmt.Errorf("htmlpdf: ...: %w", err)`
- PDF extraction errors: `fmt.Errorf("...: %w", err)` (no prefix)
- Integration tests that need Chrome call `skipIfNoChrome(t)`
- `converter_test.go` and `example_test.go` use `package htmlpdf_test` (black-box)
- `page_test.go`, `result_test.go`, `extractor_test.go` use `package htmlpdf` (white-box)
- Library-only — no CLI layer
