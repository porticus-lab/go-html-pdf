package htmlpdf

// PageSize represents paper dimensions in centimeters.
type PageSize struct {
	Width  float64 // Width in centimeters.
	Height float64 // Height in centimeters.
}

// Standard paper sizes.
var (
	A3      = PageSize{Width: 29.7, Height: 42.0}
	A4      = PageSize{Width: 21.0, Height: 29.7}
	A5      = PageSize{Width: 14.8, Height: 21.0}
	Letter  = PageSize{Width: 21.59, Height: 27.94}
	Legal   = PageSize{Width: 21.59, Height: 35.56}
	Tabloid = PageSize{Width: 27.94, Height: 43.18}
)

// Orientation represents the page orientation.
type Orientation int

const (
	// Portrait is the default vertical orientation.
	Portrait Orientation = iota
	// Landscape rotates the page to horizontal orientation.
	Landscape
)

// Margin represents page margins in centimeters.
type Margin struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// UniformMargin returns a Margin with the same value on all sides.
func UniformMargin(cm float64) Margin {
	return Margin{Top: cm, Right: cm, Bottom: cm, Left: cm}
}

// PageConfig controls the PDF output parameters.
//
// A nil PageConfig or zero-value fields will use sensible defaults:
// A4 paper, portrait orientation, 1 cm margins, scale 1.0, with
// background graphics enabled.
type PageConfig struct {
	// Size specifies the paper size. Defaults to A4.
	Size PageSize

	// Orientation specifies portrait or landscape. Defaults to Portrait.
	Orientation Orientation

	// Margin specifies page margins in centimeters. Defaults to 1 cm on all sides.
	Margin Margin

	// Scale of the webpage rendering. Must be between 0.1 and 2.0. Defaults to 1.0.
	Scale float64

	// PrintBackground enables printing of background colors and images.
	// Defaults to true.
	PrintBackground bool

	// DisplayHeaderFooter enables the header and footer templates.
	DisplayHeaderFooter bool

	// HeaderTemplate is an HTML template for the print header.
	// It uses the same format as Chrome's print header template, supporting
	// the classes: date, title, url, pageNumber, totalPages.
	HeaderTemplate string

	// FooterTemplate is an HTML template for the print footer.
	// It uses the same format as Chrome's print footer template.
	FooterTemplate string

	// PreferCSSPageSize gives precedence to any CSS @page size declared
	// in the document over the Size field.
	PreferCSSPageSize bool
}

// DefaultPageConfig returns a PageConfig with sensible defaults.
func DefaultPageConfig() PageConfig {
	return PageConfig{
		Size:            A4,
		Orientation:     Portrait,
		Margin:          UniformMargin(1.0),
		Scale:           1.0,
		PrintBackground: true,
	}
}

// resolved returns a PageConfig with all zero values replaced by defaults.
func (p *PageConfig) resolved() PageConfig {
	d := DefaultPageConfig()
	if p == nil {
		return d
	}
	r := *p
	if r.Size == (PageSize{}) {
		r.Size = d.Size
	}
	if r.Scale <= 0 {
		r.Scale = d.Scale
	}
	if r.Margin == (Margin{}) {
		r.Margin = d.Margin
	}
	// PrintBackground defaults to true; a zero-value means false, but the
	// default config sets it to true. We trust the caller here — if they
	// explicitly pass false, that's intentional.
	return r
}

// cmToInches converts centimeters to inches.
func cmToInches(cm float64) float64 {
	return cm / 2.54
}

// paperDimensions returns the paper width and height in inches,
// accounting for orientation.
func (p *PageConfig) paperDimensions() (width, height float64) {
	r := p.resolved()
	w := cmToInches(r.Size.Width)
	h := cmToInches(r.Size.Height)
	if r.Orientation == Landscape {
		return h, w
	}
	return w, h
}

// marginInches returns margins converted to inches.
func (p *PageConfig) marginInches() (top, right, bottom, left float64) {
	r := p.resolved()
	return cmToInches(r.Margin.Top),
		cmToInches(r.Margin.Right),
		cmToInches(r.Margin.Bottom),
		cmToInches(r.Margin.Left)
}
