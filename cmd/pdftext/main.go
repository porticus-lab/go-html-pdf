// pdftext extracts plain text from PDF files.
// It mirrors the functionality of zpdf (https://github.com/Lulzx/zpdf) in Go.
//
// Usage:
//
//	pdftext extract [options] <file.pdf>
//	pdftext info <file.pdf>
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/porticus-lab/go-html-pdf/internal/pdf"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "extract":
		if err := runExtract(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "info":
		if err := runInfo(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`pdftext - PDF text extraction tool (Go port of zpdf)

Usage:
  pdftext extract [options] <file.pdf>
  pdftext info <file.pdf>

Commands:
  extract   Extract plain text from a PDF file
  info      Display document metadata and page dimensions

Extract options:
  -o <file>       Write output to file (default: stdout)
  -p <range>      Page range, e.g. "1", "1-5", "1,3,5" (default: all)
  -f <format>     Output format: text, json, markdown (default: text)

Examples:
  pdftext extract document.pdf
  pdftext extract -p 1-10 -f json document.pdf > out.json
  pdftext extract -o extracted.txt document.pdf
  pdftext info document.pdf
`)
}

// runExtract implements the "extract" command.
func runExtract(args []string) error {
	var (
		outputFile string
		pageRange  string
		format     string
		inputFile  string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			i++
			if i >= len(args) {
				return fmt.Errorf("-o requires an argument")
			}
			outputFile = args[i]
		case "-p":
			i++
			if i >= len(args) {
				return fmt.Errorf("-p requires an argument")
			}
			pageRange = args[i]
		case "-f":
			i++
			if i >= len(args) {
				return fmt.Errorf("-f requires an argument")
			}
			format = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown option: %s", args[i])
			}
			inputFile = args[i]
		}
	}

	if inputFile == "" {
		return fmt.Errorf("no input file specified")
	}
	if format == "" {
		format = "text"
	}

	doc, err := pdf.Open(inputFile)
	if err != nil {
		return fmt.Errorf("opening %s: %w", inputFile, err)
	}

	pages, err := doc.Pages()
	if err != nil {
		return fmt.Errorf("reading pages: %w", err)
	}

	// Determine which pages to extract
	pageIndices, err := parsePageRange(pageRange, len(pages))
	if err != nil {
		return fmt.Errorf("invalid page range %q: %w", pageRange, err)
	}

	ext := pdf.NewExtractor(doc)

	type pageResult struct {
		Page int    `json:"page"`
		Text string `json:"text"`
	}
	var results []pageResult

	for _, idx := range pageIndices {
		text, err := ext.ExtractPage(idx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: page %d: %v\n", idx+1, err)
			continue
		}
		results = append(results, pageResult{Page: idx + 1, Text: text})
	}

	// Open output writer
	out := os.Stdout
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	// Format and write output
	switch format {
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			return fmt.Errorf("encoding JSON: %w", err)
		}
	case "markdown":
		for _, r := range results {
			fmt.Fprintf(out, "## Page %d\n\n%s\n\n", r.Page, r.Text)
		}
	default: // "text"
		for i, r := range results {
			if i > 0 {
				fmt.Fprintln(out, "\f") // form feed between pages
			}
			fmt.Fprintln(out, r.Text)
		}
	}

	return nil
}

// runInfo implements the "info" command.
func runInfo(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no input file specified")
	}
	inputFile := args[0]

	doc, err := pdf.Open(inputFile)
	if err != nil {
		return fmt.Errorf("opening %s: %w", inputFile, err)
	}

	pages, err := doc.Pages()
	if err != nil {
		return fmt.Errorf("reading pages: %w", err)
	}

	fmt.Printf("File:    %s\n", inputFile)
	fmt.Printf("Version: PDF-%s\n", doc.Version())
	fmt.Printf("Pages:   %d\n", len(pages))

	if len(pages) > 0 {
		fmt.Println()
		fmt.Println("Page dimensions:")
		for i, page := range pages {
			info := doc.GetPageInfo(page)
			fmt.Printf("  Page %d: %.0f x %.0f pt", i+1, info.Width, info.Height)
			if info.Rotation != 0 {
				fmt.Printf(" (rotated %d°)", info.Rotation)
			}
			fmt.Println()
		}
	}

	return nil
}

// parsePageRange converts a page range string to a slice of 0-based page indices.
// Supported formats: "" (all), "3" (single page), "1-5" (range), "1,3,5" (list).
func parsePageRange(spec string, total int) ([]int, error) {
	if spec == "" {
		// All pages
		indices := make([]int, total)
		for i := range indices {
			indices[i] = i
		}
		return indices, nil
	}

	var indices []int
	seen := make(map[int]bool)

	parts := strings.Split(spec, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// Range like "1-5"
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid page number: %s", bounds[0])
			}
			end, err := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid page number: %s", bounds[1])
			}
			if start < 1 || end > total || start > end {
				return nil, fmt.Errorf("page range %d-%d out of bounds (1-%d)", start, end, total)
			}
			for p := start; p <= end; p++ {
				if !seen[p] {
					indices = append(indices, p-1)
					seen[p] = true
				}
			}
		} else {
			// Single page
			p, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid page number: %s", part)
			}
			if p < 1 || p > total {
				return nil, fmt.Errorf("page %d out of bounds (1-%d)", p, total)
			}
			if !seen[p] {
				indices = append(indices, p-1)
				seen[p] = true
			}
		}
	}

	return indices, nil
}
