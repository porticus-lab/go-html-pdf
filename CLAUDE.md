# CLAUDE.md — go-html-pdf

## Project Overview

Go library (`htmlpdf` package) that converts modern HTML5+CSS3 to PDF using the Chrome DevTools Protocol. Module: `github.com/porticus-lab/go-html-pdf`. Licensed under Apache 2.0.

## Tech Stack

- **Language**: Go 1.24+
- **Core dependency**: `chromedp` + `cdproto` (MIT, free) — headless Chrome via CDP
- **Optional dependency**: `go-rod/rod` (MIT, free) — Chromium auto-download via `lib/launcher`
- **No paid dependencies allowed**: all deps must be free and open source

## Architecture

| File              | Purpose                                                    |
|-------------------|------------------------------------------------------------|
| `doc.go`          | Package-level documentation and usage examples             |
| `page.go`         | Types: `PageSize`, `Orientation`, `Margin`, `PageConfig`   |
| `options.go`      | Functional options: `WithTimeout`, `WithChromePath`, etc.   |
| `errors.go`       | Sentinel errors (`ErrClosed`)                              |
| `result.go`       | `Result` type: `Bytes`, `Base64`, `Reader`, `WriteTo`, `WriteToFile` |
| `browser.go`      | Auto-download logic via `go-rod/rod/lib/launcher`          |
| `converter.go`    | `Converter` struct + package-level convenience functions    |
| `page_test.go`    | Unit tests for page config (no Chrome needed)              |
| `result_test.go`  | Unit tests for Result methods (no Chrome needed)           |
| `converter_test.go` | Integration tests (skipped if Chrome not in PATH)        |
| `example_test.go` | Testable examples for `go doc`                             |

## Key Design Decisions

- **Units**: Public API uses centimeters for margins/dimensions; internal conversion to inches for Chrome's printToPDF
- **Browser reuse**: `Converter` keeps a single browser process alive; each conversion opens a new tab
- **Thread safety**: `Converter` is safe for concurrent use (`sync.Mutex` on close state)
- **Functional options pattern**: `Option` type with `With*` constructors
- **Nil-safe PageConfig**: `nil` or zero-value `PageConfig` resolves to defaults (A4, portrait, 1cm margins, scale 1.0)
- **Result type**: Conversion methods return `*Result` with helpers: `Bytes()`, `Base64()`, `Reader()`, `WriteTo()`, `WriteToFile()`, `Len()` — designed for direct cloud storage uploads (GCP, S3)
- **Auto-download**: `WithAutoDownload()` uses `go-rod/rod/lib/launcher` to download and cache a Chromium binary; ignored when `WithChromePath` is set. Browser path resolution order: explicit path > auto-download > system PATH

## Commands

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run only unit tests (no Chrome required)
go test -run 'TestCm|TestDefault|TestUniform|TestPageConfig|TestPaper|TestMargin|TestResult_' ./...

# Run with verbose output
go test -v ./...

# Vet
go vet ./...

# Tidy dependencies
go mod tidy
```

## Conventions

- Follow standard Go library conventions: exported names documented, lowercase package name, no underscores in file names
- Use `fmt.Errorf("htmlpdf: ...: %w", err)` for error wrapping — always prefix with `htmlpdf:`
- Integration tests that need Chrome must call `skipIfNoChrome(t)` at the top
- Predefined page sizes are package-level vars (`A4`, `Letter`, etc.), not constants
- Keep the dependency tree minimal — do not add dependencies unless strictly necessary
- Tests in `converter_test.go` and `example_test.go` use `package htmlpdf_test` (black-box); `page_test.go` and `result_test.go` use `package htmlpdf` (white-box for internal helpers)
