package htmlpdf_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	htmlpdf "github.com/porticus-lab/go-html-pdf"
)

// chromeAvailable reports whether a Chrome/Chromium executable is in PATH.
func chromeAvailable() bool {
	for _, name := range []string{
		"chromium-browser", "chromium", "google-chrome",
		"google-chrome-stable", "chrome",
	} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

func skipIfNoChrome(t *testing.T) {
	t.Helper()
	if !chromeAvailable() {
		t.Skip("skipping: Chrome/Chromium not found in PATH")
	}
}

func newTestConverter(t *testing.T) *htmlpdf.Converter {
	t.Helper()
	skipIfNoChrome(t)
	c, err := htmlpdf.NewConverter(htmlpdf.WithNoSandbox())
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// isPDF checks whether data starts with the PDF magic number.
func isPDF(data []byte) bool {
	return len(data) > 4 && string(data[:5]) == "%PDF-"
}

func TestConvertHTML_Basic(t *testing.T) {
	c := newTestConverter(t)

	res, err := c.ConvertHTML(context.Background(), "<h1>Hello World</h1>", nil)
	if err != nil {
		t.Fatalf("ConvertHTML: %v", err)
	}
	if !isPDF(res.Bytes()) {
		t.Fatal("output is not a valid PDF")
	}
	if res.Len() < 100 {
		t.Errorf("PDF unexpectedly small: %d bytes", res.Len())
	}
}

func TestConvertHTML_WithPageConfig(t *testing.T) {
	c := newTestConverter(t)

	page := &htmlpdf.PageConfig{
		Size:            htmlpdf.Letter,
		Orientation:     htmlpdf.Landscape,
		Margin:          htmlpdf.UniformMargin(2.0),
		Scale:           1.0,
		PrintBackground: true,
	}

	html := `<!DOCTYPE html>
<html>
<head><style>
  body { background: #f0f0f0; font-family: sans-serif; }
  .container { display: flex; gap: 1rem; padding: 2rem; }
  .card { background: white; border-radius: 8px; padding: 1rem; flex: 1; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
</style></head>
<body>
  <div class="container">
    <div class="card"><h2>Card 1</h2><p>Modern CSS with flexbox</p></div>
    <div class="card"><h2>Card 2</h2><p>Shadows and border-radius</p></div>
    <div class="card"><h2>Card 3</h2><p>Background colors</p></div>
  </div>
</body>
</html>`

	res, err := c.ConvertHTML(context.Background(), html, page)
	if err != nil {
		t.Fatalf("ConvertHTML: %v", err)
	}
	if !isPDF(res.Bytes()) {
		t.Fatal("output is not a valid PDF")
	}
}

func TestConvertHTML_ModernCSS(t *testing.T) {
	c := newTestConverter(t)

	html := `<!DOCTYPE html>
<html>
<head><style>
  :root { --primary: #3b82f6; --radius: 12px; }
  * { margin: 0; box-sizing: border-box; }
  body { font-family: system-ui, sans-serif; background: #f8fafc; padding: 2rem; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1.5rem; }
  .card {
    background: white;
    border-radius: var(--radius);
    padding: 1.5rem;
    box-shadow: 0 1px 3px rgba(0,0,0,0.12);
    transition: transform 0.2s;
  }
  .card h3 { color: var(--primary); margin-bottom: 0.5rem; }
  .badge {
    display: inline-block;
    background: var(--primary);
    color: white;
    padding: 0.25rem 0.75rem;
    border-radius: 9999px;
    font-size: 0.875rem;
  }
  @media print { body { background: white; } }
</style></head>
<body>
  <h1 style="margin-bottom:1.5rem">Modern CSS Grid Layout</h1>
  <div class="grid">
    <div class="card">
      <h3>CSS Grid</h3>
      <p>Auto-fit responsive columns</p>
      <span class="badge">Grid</span>
    </div>
    <div class="card">
      <h3>Custom Properties</h3>
      <p>CSS variables for theming</p>
      <span class="badge">Variables</span>
    </div>
    <div class="card">
      <h3>Modern Selectors</h3>
      <p>Using :root and var()</p>
      <span class="badge">CSS3</span>
    </div>
  </div>
</body>
</html>`

	res, err := c.ConvertHTML(context.Background(), html, nil)
	if err != nil {
		t.Fatalf("ConvertHTML: %v", err)
	}
	if !isPDF(res.Bytes()) {
		t.Fatal("output is not a valid PDF")
	}
}

func TestConvertFile(t *testing.T) {
	c := newTestConverter(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.html")
	if err := os.WriteFile(path, []byte("<h1>From File</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := c.ConvertFile(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if !isPDF(res.Bytes()) {
		t.Fatal("output is not a valid PDF")
	}
}

func TestConvertFile_NotFound(t *testing.T) {
	c := newTestConverter(t)

	_, err := c.ConvertFile(context.Background(), "/nonexistent/file.html", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestConvertURL_InvalidURL(t *testing.T) {
	c := newTestConverter(t)

	_, err := c.ConvertURL(context.Background(), "not a url", nil)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestConverter_CloseIdempotent(t *testing.T) {
	skipIfNoChrome(t)

	c, err := htmlpdf.NewConverter(htmlpdf.WithNoSandbox())
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestConverter_UsedAfterClose(t *testing.T) {
	skipIfNoChrome(t)

	c, err := htmlpdf.NewConverter(htmlpdf.WithNoSandbox())
	if err != nil {
		t.Fatal(err)
	}
	c.Close()

	_, err = c.ConvertHTML(context.Background(), "<p>test</p>", nil)
	if err != htmlpdf.ErrClosed {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestConvertHTML_PackageLevel(t *testing.T) {
	skipIfNoChrome(t)

	res, err := htmlpdf.ConvertHTML(
		context.Background(),
		"<p>Package-level function</p>",
		nil,
		htmlpdf.WithNoSandbox(),
	)
	if err != nil {
		t.Fatalf("ConvertHTML: %v", err)
	}
	if !isPDF(res.Bytes()) {
		t.Fatal("output is not a valid PDF")
	}
}

func TestAllPageSizes(t *testing.T) {
	c := newTestConverter(t)

	sizes := []struct {
		name string
		size htmlpdf.PageSize
	}{
		{"A3", htmlpdf.A3},
		{"A4", htmlpdf.A4},
		{"A5", htmlpdf.A5},
		{"Letter", htmlpdf.Letter},
		{"Legal", htmlpdf.Legal},
		{"Tabloid", htmlpdf.Tabloid},
	}

	for _, s := range sizes {
		t.Run(s.name, func(t *testing.T) {
			res, err := c.ConvertHTML(context.Background(), "<p>"+s.name+"</p>", &htmlpdf.PageConfig{
				Size:            s.size,
				Scale:           1.0,
				PrintBackground: true,
			})
			if err != nil {
				t.Fatalf("ConvertHTML(%s): %v", s.name, err)
			}
			if !isPDF(res.Bytes()) {
				t.Fatalf("%s: output is not a valid PDF", s.name)
			}
		})
	}
}

func TestResult_Base64(t *testing.T) {
	c := newTestConverter(t)

	res, err := c.ConvertHTML(context.Background(), "<p>base64 test</p>", nil)
	if err != nil {
		t.Fatal(err)
	}
	b64 := res.Base64()
	if len(b64) == 0 {
		t.Fatal("Base64 returned empty string")
	}
	// base64 of %PDF- starts with JVBER
	if b64[:5] != "JVBER" {
		t.Errorf("Base64 does not start with expected PDF prefix, got %s...", b64[:10])
	}
}

func TestResult_Reader(t *testing.T) {
	c := newTestConverter(t)

	res, err := c.ConvertHTML(context.Background(), "<p>reader test</p>", nil)
	if err != nil {
		t.Fatal(err)
	}
	r := res.Reader()
	if r.Len() != res.Len() {
		t.Errorf("Reader().Len() = %d, want %d", r.Len(), res.Len())
	}
}

func TestResult_WriteToFile(t *testing.T) {
	c := newTestConverter(t)

	res, err := c.ConvertHTML(context.Background(), "<p>file test</p>", nil)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "out.pdf")
	if err := res.WriteToFile(path, 0o644); err != nil {
		t.Fatalf("WriteToFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !isPDF(data) {
		t.Fatal("written file is not a valid PDF")
	}
	if len(data) != res.Len() {
		t.Errorf("written %d bytes, expected %d", len(data), res.Len())
	}
}
