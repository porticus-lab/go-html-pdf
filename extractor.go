package htmlpdf

import (
	"math"
	"strings"
	"unicode"
)

// Extractor extracts plain text from PDF pages.
type Extractor struct {
	doc *Document
}

// NewExtractor creates a text extractor for the given document.
func NewExtractor(doc *Document) *Extractor {
	return &Extractor{doc: doc}
}

// ExtractPage returns the plain text for a single page (0-indexed).
func (e *Extractor) ExtractPage(pageIndex int) (string, error) {
	pages, err := e.doc.Pages()
	if err != nil {
		return "", err
	}
	if pageIndex < 0 || pageIndex >= len(pages) {
		return "", nil
	}
	return e.ExtractPageDict(pages[pageIndex])
}

// ExtractAll returns the plain text for all pages, one page per element.
func (e *Extractor) ExtractAll() ([]string, error) {
	pages, err := e.doc.Pages()
	if err != nil {
		return nil, err
	}
	results := make([]string, len(pages))
	for i, page := range pages {
		text, err := e.ExtractPageDict(page)
		if err != nil {
			continue
		}
		results[i] = text
	}
	return results, nil
}

// ExtractPageDict extracts text from a page dictionary.
func (e *Extractor) ExtractPageDict(page Dict) (string, error) {
	// Get fonts for this page
	fontObjs, err := e.doc.PageFonts(page)
	if err != nil {
		fontObjs = nil
	}

	// Build font encoding map: resource name -> FontEncoding
	fonts := make(map[string]*FontEncoding)
	for name, obj := range fontObjs {
		fonts[name] = NewFontEncoding(obj)
	}

	// Get and parse content streams
	content, err := e.doc.ContentStreams(page)
	if err != nil {
		return "", err
	}
	if len(content) == 0 {
		return "", nil
	}

	return parseContentStream(content, fonts), nil
}

// ---- Content stream parser ----

// textSpan represents a positioned piece of text extracted from a content stream.
type textSpan struct {
	x, y     float64
	text     string
	fontSize float64
}

// textState holds the current PDF text state during content stream parsing.
type textState struct {
	fontName    string
	fontSize    float64
	charSpacing float64
	wordSpacing float64
	// Text matrix components (simplified: we only track tx, ty)
	tx, ty float64
	// Line matrix
	lx, ly  float64
	leading float64
	// Current transformation matrix (simplified)
	ctmA, ctmB, ctmC, ctmD, ctmE, ctmF float64
}

func newTextState() textState {
	return textState{
		ctmA:     1,
		ctmD:     1,
		fontSize: 12,
	}
}

// parseContentStream parses a PDF content stream and extracts text.
func parseContentStream(data []byte, fonts map[string]*FontEncoding) string {
	p := NewParser(data, 0)
	ts := newTextState()
	inText := false

	var spans []textSpan
	var operandStack []*Object

	for p.pos < len(data) {
		p.skipWhitespace()
		if p.pos >= len(data) {
			break
		}

		c := data[p.pos]

		// Operand: string, name, number, array, dict
		if c == '(' || c == '<' || c == '/' || c == '[' ||
			c == '+' || c == '-' || c == '.' ||
			(c >= '0' && c <= '9') {
			obj, err := p.ParseObject()
			if err == nil {
				operandStack = append(operandStack, obj)
			}
			continue
		}

		// Operator: alphabetic or special
		if isOperatorStart(c) {
			op := p.readOperator()
			processOperator(op, &operandStack, &ts, &inText, &spans, fonts)
			continue
		}

		p.pos++
	}

	return spansToText(spans)
}

func isOperatorStart(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		c == '\'' || c == '"' || c == '*'
}

// readOperator reads a PDF content stream operator.
func (p *Parser) readOperator() string {
	start := p.pos
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' ||
			c == '(' || c == '<' || c == '[' || c == '/' ||
			(c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.' {
			break
		}
		p.pos++
	}
	return string(p.data[start:p.pos])
}

// processOperator handles one content stream operator and its operands.
func processOperator(
	op string,
	stack *[]*Object,
	ts *textState,
	inText *bool,
	spans *[]textSpan,
	fonts map[string]*FontEncoding,
) {
	args := *stack
	*stack = (*stack)[:0]

	switch op {
	// ---- Graphics state ----
	case "q": // push graphics state (ignored for text)
	case "Q": // pop graphics state (ignored for text)
	case "cm": // concat matrix
		if len(args) >= 6 {
			ts.ctmA = floatArg(args[0])
			ts.ctmB = floatArg(args[1])
			ts.ctmC = floatArg(args[2])
			ts.ctmD = floatArg(args[3])
			ts.ctmE = floatArg(args[4])
			ts.ctmF = floatArg(args[5])
		}

	// ---- Text object ----
	case "BT": // Begin text
		*inText = true
		ts.tx, ts.ty = 0, 0
		ts.lx, ts.ly = 0, 0
	case "ET": // End text
		*inText = false

	// ---- Text state ----
	case "Tf": // Set font and size
		if len(args) >= 2 {
			if args[0].Type == ObjName {
				ts.fontName = args[0].Name
			}
			ts.fontSize = floatArg(args[1])
		}
	case "Tc": // Character spacing
		if len(args) >= 1 {
			ts.charSpacing = floatArg(args[0])
		}
	case "Tw": // Word spacing
		if len(args) >= 1 {
			ts.wordSpacing = floatArg(args[0])
		}
	case "TL": // Text leading
		if len(args) >= 1 {
			ts.leading = floatArg(args[0])
		}

	// ---- Text positioning ----
	case "Td": // Move text position
		if len(args) >= 2 {
			tx := floatArg(args[0])
			ty := floatArg(args[1])
			ts.lx += tx
			ts.ly += ty
			ts.tx = ts.lx
			ts.ty = ts.ly
		}
	case "TD": // Move text position and set leading
		if len(args) >= 2 {
			tx := floatArg(args[0])
			ty := floatArg(args[1])
			ts.leading = -ty
			ts.lx += tx
			ts.ly += ty
			ts.tx = ts.lx
			ts.ty = ts.ly
		}
	case "Tm": // Set text matrix
		if len(args) >= 6 {
			ts.tx = floatArg(args[4])
			ts.ty = floatArg(args[5])
			ts.lx = ts.tx
			ts.ly = ts.ty
		}
	case "T*": // Move to next line
		ts.lx = 0
		ts.ly -= ts.leading
		ts.tx = ts.lx
		ts.ty = ts.ly

	// ---- Text showing ----
	case "Tj": // Show text string
		if *inText && len(args) >= 1 {
			text := decodeTextObj(args[0], ts.fontName, fonts)
			if text != "" {
				*spans = append(*spans, textSpan{
					x:        ts.tx,
					y:        ts.ty,
					text:     text,
					fontSize: ts.fontSize,
				})
			}
		}
	case "TJ": // Show text array with kerning
		if *inText && len(args) >= 1 && args[0].Type == ObjArray {
			var sb strings.Builder
			for _, elem := range args[0].Array {
				switch elem.Type {
				case ObjString:
					sb.WriteString(decodeTextObj(elem, ts.fontName, fonts))
				case ObjInt, ObjFloat:
					// Negative kerning values indicate word spaces
					kern := floatArg(elem)
					if kern < -100 {
						sb.WriteRune(' ')
					}
				}
			}
			text := sb.String()
			if text != "" {
				*spans = append(*spans, textSpan{
					x:        ts.tx,
					y:        ts.ty,
					text:     text,
					fontSize: ts.fontSize,
				})
			}
		}
	case "'": // Move to next line and show text
		ts.lx = 0
		ts.ly -= ts.leading
		ts.tx = ts.lx
		ts.ty = ts.ly
		if *inText && len(args) >= 1 {
			text := decodeTextObj(args[0], ts.fontName, fonts)
			if text != "" {
				*spans = append(*spans, textSpan{
					x:        ts.tx,
					y:        ts.ty,
					text:     text,
					fontSize: ts.fontSize,
				})
			}
		}
	case `"`: // Set spacing, move to next line, and show text
		if len(args) >= 3 {
			ts.wordSpacing = floatArg(args[0])
			ts.charSpacing = floatArg(args[1])
		}
		ts.lx = 0
		ts.ly -= ts.leading
		ts.tx = ts.lx
		ts.ty = ts.ly
		if *inText && len(args) >= 3 {
			text := decodeTextObj(args[2], ts.fontName, fonts)
			if text != "" {
				*spans = append(*spans, textSpan{
					x:        ts.tx,
					y:        ts.ty,
					text:     text,
					fontSize: ts.fontSize,
				})
			}
		}

	// ---- Marked content (ignored for basic extraction) ----
	case "BMC", "BDC", "EMC", "MP", "DP":
		// Ignore marked content operators

	// All other operators (path, image, color, etc.) are ignored
	}
}

// decodeTextObj decodes a PDF string object to a Unicode string using the current font.
func decodeTextObj(obj *Object, fontName string, fonts map[string]*FontEncoding) string {
	if obj.Type != ObjString {
		return ""
	}
	if enc, ok := fonts[fontName]; ok {
		return enc.Decode(obj.Str)
	}
	// Fallback: treat as Latin-1
	var sb strings.Builder
	for _, b := range obj.Str {
		if b >= 32 && b < 128 {
			sb.WriteByte(b)
		} else if b >= 128 {
			sb.WriteRune(rune(b))
		}
	}
	return sb.String()
}

func floatArg(obj *Object) float64 {
	if obj == nil {
		return 0
	}
	switch obj.Type {
	case ObjFloat:
		return obj.Float
	case ObjInt:
		return float64(obj.Int)
	}
	return 0
}

// ---- Span-to-text assembly ----

// spansToText converts positioned text spans into a readable string,
// inserting spaces and newlines based on position differences.
func spansToText(spans []textSpan) string {
	if len(spans) == 0 {
		return ""
	}

	// Group spans into lines by Y coordinate (within tolerance)
	type line struct {
		y     float64
		spans []textSpan
	}

	var lines []line
	lineTol := averageFontSize(spans) * 0.5
	if lineTol < 2 {
		lineTol = 2
	}

	for _, sp := range spans {
		found := false
		for i := range lines {
			if math.Abs(lines[i].y-sp.y) < lineTol {
				lines[i].spans = append(lines[i].spans, sp)
				found = true
				break
			}
		}
		if !found {
			lines = append(lines, line{y: sp.y, spans: []textSpan{sp}})
		}
	}

	// Sort lines by descending Y (PDF y=0 is bottom)
	for i := 0; i < len(lines)-1; i++ {
		for j := i + 1; j < len(lines); j++ {
			if lines[j].y > lines[i].y {
				lines[i], lines[j] = lines[j], lines[i]
			}
		}
	}

	// Sort spans within each line by X
	for i := range lines {
		sortSpansByX(lines[i].spans)
	}

	var sb strings.Builder
	for li, l := range lines {
		if li > 0 {
			sb.WriteByte('\n')
		}
		// Emit spans with space between them if there's a gap
		for si, sp := range l.spans {
			if si > 0 {
				prev := l.spans[si-1]
				gap := sp.x - (prev.x + estimateWidth(prev))
				avgFS := (sp.fontSize + prev.fontSize) / 2
				if avgFS < 1 {
					avgFS = 12
				}
				if gap > avgFS*0.3 {
					sb.WriteByte(' ')
				}
			}
			sb.WriteString(cleanText(sp.text))
		}
	}

	return strings.TrimSpace(sb.String())
}

func averageFontSize(spans []textSpan) float64 {
	if len(spans) == 0 {
		return 12
	}
	sum := 0.0
	for _, s := range spans {
		sum += s.fontSize
	}
	return sum / float64(len(spans))
}

// estimateWidth gives a rough character-width estimate for a span.
func estimateWidth(sp textSpan) float64 {
	return float64(len([]rune(sp.text))) * sp.fontSize * 0.5
}

func sortSpansByX(spans []textSpan) {
	for i := 0; i < len(spans)-1; i++ {
		for j := i + 1; j < len(spans); j++ {
			if spans[j].x < spans[i].x {
				spans[i], spans[j] = spans[j], spans[i]
			}
		}
	}
}

// cleanText normalises whitespace and removes control characters.
func cleanText(s string) string {
	var sb strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == '\r' || r == '\n' || r == '\f' {
			if !prevSpace {
				sb.WriteByte(' ')
			}
			prevSpace = true
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		if r == ' ' || r == '\t' {
			if !prevSpace {
				sb.WriteRune(r)
			}
			prevSpace = true
			continue
		}
		prevSpace = false
		sb.WriteRune(r)
	}
	return sb.String()
}
