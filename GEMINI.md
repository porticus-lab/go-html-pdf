# GEMINI.md — go-html-pdf

## Project Overview

Go module with **two complementary PDF libraries**:

| Package | Import | Purpose |
|---------|--------|---------|
| `htmlpdf` | `github.com/porticus-lab/go-html-pdf` | HTML + CSS → PDF via headless Chrome (CDP) |
| `pdf` | `github.com/porticus-lab/go-html-pdf/pdf` | PDF → text extraction, pure Go, no external deps |

Module: `github.com/porticus-lab/go-html-pdf`. Licensed under Apache 2.0.

---

## Package 1: `htmlpdf` — HTML to PDF

### Tech Stack

- **Language**: Go 1.24+
- **Core dependency**: `chromedp` + `cdproto` (MIT) — headless Chrome via CDP
- **Optional dependency**: `go-rod/rod` (MIT) — Chromium auto-download via `lib/launcher`
- **No paid dependencies allowed**

### Architecture

| File | Purpose |
|------|---------|
| `doc.go` | Package-level documentation |
| `page.go` | `PageSize`, `Orientation`, `Margin`, `PageConfig`, `DefaultPageConfig` |
| `options.go` | Functional options: `WithTimeout`, `WithChromePath`, `WithNoSandbox`, `WithAutoDownload` |
| `errors.go` | Sentinel errors (`ErrClosed`) |
| `result.go` | `Result`: `Bytes`, `Base64`, `Reader`, `WriteTo`, `WriteToFile`, `Len` |
| `browser.go` | Auto-download logic via `go-rod/rod/lib/launcher` |
| `converter.go` | `Converter` struct + package-level convenience functions |
| `page_test.go` | Unit tests — no Chrome required |
| `result_test.go` | Unit tests — no Chrome required |
| `converter_test.go` | Integration tests — skipped if Chrome not in PATH |
| `example_test.go` | Testable examples for `go doc` |

### Key Design Decisions

- **Units**: Public API uses centimetres; internally converted to inches for Chrome's `printToPDF`
- **Browser reuse**: `Converter` keeps a single browser process alive; each conversion opens a new tab
- **Thread safety**: `Converter` is safe for concurrent use (`sync.Mutex` on close state)
- **Functional options**: `Option func(*converterConfig)` with `With*` constructors
- **Nil-safe PageConfig**: `nil` or zero-value `PageConfig` resolves to defaults (A4, portrait, 1cm, scale 1.0)
- **Result type**: `*Result` with `Bytes()`, `Base64()`, `Reader()`, `WriteTo()`, `WriteToFile()`, `Len()` — designed for direct cloud storage uploads
- **Auto-download**: `WithAutoDownload()` uses `go-rod/rod/lib/launcher` to cache a Chromium binary; ignored when `WithChromePath` is set. Lookup order: explicit path > auto-download > system PATH

---

## Package 2: `pdf` — PDF to Text

### Tech Stack

- **Language**: Go 1.21+
- **Dependencies**: none (stdlib only)

### Architecture

| File | Purpose |
|------|---------|
| `pdf/parser.go` | Recursive-descent PDF object parser (all object types) |
| `pdf/document.go` | Document loading, XRef table/stream, object resolution, page tree |
| `pdf/decompress.go` | Stream filters: FlateDecode, ASCII85, ASCIIHex, LZW, RunLength |
| `pdf/encoding.go` | Font encoding: WinAnsi, MacRoman, ToUnicode CMap, Adobe Glyph List |
| `pdf/extractor.go` | Content-stream text extraction, positional line assembly |
| `pdf/extractor_test.go` | Table-driven tests |

### Key Design Decisions

- **Pure Go**: no CGo, no external dependencies
- **Full PDF object model**: null, bool, int, float, literal/hex strings, names, arrays, dicts, streams, indirect references
- **XRef**: traditional tables + PDF 1.5+ cross-reference streams + compressed object streams
- **Decompression guard**: 256 MB limit on decompressed output
- **Font decoding priority**: ToUnicode CMap > Encoding dict > Named encoding > Default

---

## Commands

```bash
# Build everything
go build ./...

# All tests
go test ./...

# Only unit tests (no Chrome required)
go test -run 'TestCm|TestDefault|TestUniform|TestPageConfig|TestPaper|TestMargin|TestResult_|TestExtract|TestParser|TestAscii|TestRunLength|TestWinAnsi|TestDecode' ./...

# Verbose
go test -v ./...

# Vet
go vet ./...

# Tidy
go mod tidy
```

## Conventions

- Exported names documented; lowercase package names; no underscores in file names
- `htmlpdf` errors: always `fmt.Errorf("htmlpdf: ...: %w", err)`
- `pdf` errors: always `fmt.Errorf("...: %w", err)`
- Integration tests that need Chrome call `skipIfNoChrome(t)`
- `converter_test.go` and `example_test.go` use `package htmlpdf_test`; `page_test.go` and `result_test.go` use `package htmlpdf`
- `pdf/extractor_test.go` uses `package pdf`
- Keep `pdf/` free of external dependencies; keep deps in the root package minimal and free/open-source
- Both packages are **library-only** — no CLI layer
