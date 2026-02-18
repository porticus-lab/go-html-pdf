// Package htmlpdf provides two complementary PDF capabilities under a single import:
//
//   - HTML + CSS → PDF conversion via headless Chrome (Chrome DevTools Protocol)
//   - PDF → plain-text extraction, pure Go, no external dependencies
//
// # HTML to PDF
//
// For one-off conversions use the package-level helpers:
//
//	res, err := htmlpdf.ConvertHTML(ctx, "<h1>Hello</h1>", nil)
//
// For repeated conversions create a [Converter], which reuses the browser process:
//
//	c, err := htmlpdf.NewConverter()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer c.Close()
//
//	res, err := c.ConvertHTML(ctx, "<h1>Hello</h1>", nil)
//	res, err  = c.ConvertURL(ctx, "https://example.com", nil)
//	res, err  = c.ConvertFile(ctx, "report.html", nil)
//
// Use [PageConfig] to control paper size, orientation, margins, and scale:
//
//	page := &htmlpdf.PageConfig{
//	    Size:        htmlpdf.A4,
//	    Orientation: htmlpdf.Landscape,
//	    Margin:      htmlpdf.UniformMargin(2.0),
//	}
//	res, err := c.ConvertHTML(ctx, html, page)
//
// A [Result] gives flexible access to the generated PDF bytes:
//
//	res.Bytes()                       // []byte
//	res.Base64()                      // base64 string (RFC 4648)
//	res.Reader()                      // *bytes.Reader
//	res.WriteTo(w)                    // io.WriterTo
//	res.WriteToFile("out.pdf", 0o644) // write to disk
//
// Chrome or Chromium must be available in PATH, or use [WithAutoDownload]:
//
//	c, err := htmlpdf.NewConverter(htmlpdf.WithAutoDownload())
//
// # PDF to Text
//
// Open a PDF from disk or raw bytes:
//
//	doc, err := htmlpdf.Open("document.pdf")
//	doc, err  = htmlpdf.Load(data) // from []byte
//
// Extract text page by page:
//
//	ext := htmlpdf.NewExtractor(doc)
//
//	pages, err := ext.ExtractAll()     // []string, one per page
//	text, err  := ext.ExtractPage(0)   // single page, 0-indexed
//
// Access low-level page metadata:
//
//	pages, err := doc.Pages()
//	info := doc.GetPageInfo(pages[0]) // PageInfo{Width, Height, Rotation}
package htmlpdf
