package htmlpdf_test

import (
	"context"
	"fmt"
	"log"
	"time"

	htmlpdf "github.com/porticus-lab/go-html-pdf"
)

func Example() {
	// Create a converter (reuses the browser across conversions).
	c, err := htmlpdf.NewConverter(htmlpdf.WithNoSandbox())
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Convert HTML to PDF with default page settings (A4, portrait).
	res, err := c.ConvertHTML(context.Background(), "<h1>Hello World</h1>", nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Generated PDF: %d bytes\n", res.Len())
}

func Example_withPageConfig() {
	c, err := htmlpdf.NewConverter(
		htmlpdf.WithTimeout(60*time.Second),
		htmlpdf.WithNoSandbox(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	page := &htmlpdf.PageConfig{
		Size:            htmlpdf.Letter,
		Orientation:     htmlpdf.Landscape,
		Margin:          htmlpdf.Margin{Top: 2, Right: 2.5, Bottom: 2, Left: 2.5},
		Scale:           1.0,
		PrintBackground: true,
	}

	html := `<!DOCTYPE html>
<html><body>
  <h1 style="color: navy;">Landscape Report</h1>
  <p>This PDF uses Letter size in landscape orientation.</p>
</body></html>`

	res, err := c.ConvertHTML(context.Background(), html, page)
	if err != nil {
		log.Fatal(err)
	}

	if err := res.WriteToFile("/tmp/report.pdf", 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("PDF saved to /tmp/report.pdf")
}

func Example_modernCSS() {
	c, err := htmlpdf.NewConverter(htmlpdf.WithNoSandbox())
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	html := `<!DOCTYPE html>
<html>
<head><style>
  :root { --accent: #6366f1; }
  body { font-family: system-ui; padding: 2rem; }
  .grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 1rem;
  }
  .card {
    background: linear-gradient(135deg, var(--accent), #8b5cf6);
    color: white;
    padding: 1.5rem;
    border-radius: 12px;
  }
</style></head>
<body>
  <h1>CSS Grid + Gradients</h1>
  <div class="grid">
    <div class="card"><h3>One</h3></div>
    <div class="card"><h3>Two</h3></div>
    <div class="card"><h3>Three</h3></div>
  </div>
</body>
</html>`

	res, err := c.ConvertHTML(context.Background(), html, &htmlpdf.PageConfig{
		PrintBackground: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Modern CSS PDF: %d bytes\n", res.Len())
}

func Example_resultOutputFormats() {
	c, err := htmlpdf.NewConverter(htmlpdf.WithNoSandbox())
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	res, err := c.ConvertHTML(context.Background(), "<h1>Output formats</h1>", nil)
	if err != nil {
		log.Fatal(err)
	}

	// Raw bytes — for any io.Writer or low-level use.
	_ = res.Bytes()

	// Base64 string — for JSON APIs or services that accept base64.
	_ = res.Base64()

	// io.Reader — for streaming uploads (GCP Cloud Storage, AWS S3, etc.).
	_ = res.Reader()

	// Write directly to a file.
	_ = res.WriteToFile("/tmp/output.pdf", 0o644)

	// io.WriterTo — write to any io.Writer.
	// res.WriteTo(w)

	fmt.Printf("PDF ready: %d bytes\n", res.Len())
}
