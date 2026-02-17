// Package htmlpdf converts HTML and CSS to PDF documents.
//
// It supports modern HTML5 and CSS3 features including flexbox, grid,
// media queries, web fonts, and more by leveraging the Chrome DevTools
// Protocol for accurate rendering. All dependencies are free and open source.
//
// # Quick Start
//
// For one-off conversions, use the package-level functions:
//
//	pdf, err := htmlpdf.ConvertHTML(ctx, "<h1>Hello</h1>", nil)
//
// For repeated conversions, create a [Converter] to reuse the browser instance:
//
//	c, err := htmlpdf.NewConverter()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer c.Close()
//
//	pdf, err := c.ConvertHTML(ctx, "<h1>Hello</h1>", nil)
//
// # Page Configuration
//
// Use [PageConfig] to control paper size, orientation, margins, and more:
//
//	page := &htmlpdf.PageConfig{
//	    Size:        htmlpdf.A4,
//	    Orientation: htmlpdf.Landscape,
//	    Margin:      htmlpdf.UniformMargin(2.0),
//	    Scale:       1.0,
//	}
//	pdf, err := c.ConvertHTML(ctx, html, page)
//
// # Requirements
//
// A Chrome or Chromium browser must be available. By default the library
// searches standard system locations. Alternatively, pass
// [WithAutoDownload] to automatically download and cache a compatible
// Chromium binary:
//
//	c, err := htmlpdf.NewConverter(htmlpdf.WithAutoDownload())
//
// The library uses headless mode and does not open any visible browser
// windows.
package htmlpdf
