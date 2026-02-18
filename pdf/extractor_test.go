package pdf

import (
	"strings"
	"testing"
)

// buildTestPDF creates a minimal valid PDF with the given page content stream.
func buildTestPDF(contentStreams [][]byte) []byte {
	var parts [][]byte
	cat := func(s string) { parts = append(parts, []byte(s)) }
	catb := func(b []byte) { parts = append(parts, b) }
	totalLen := func() int {
		n := 0
		for _, p := range parts {
			n += len(p)
		}
		return n
	}

	cat("%PDF-1.4\n")

	objOffsets := map[int]int{}

	// Catalog
	objOffsets[1] = totalLen()
	cat("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	numPages := len(contentStreams)
	kidsRefs := ""
	for i := range contentStreams {
		kidsRefs += " " + itoa(3+i*2) + " 0 R"
	}
	if len(kidsRefs) > 0 {
		kidsRefs = kidsRefs[1:]
	}

	// Pages node
	objOffsets[2] = totalLen()
	cat("2 0 obj\n<< /Type /Pages /Kids [" + kidsRefs + "] /Count " + itoa(numPages) + " >>\nendobj\n")

	nextObjID := 3
	fontObjID := 3 + numPages*2

	for _, cs := range contentStreams {
		pageObjID := nextObjID
		csObjID := nextObjID + 1
		nextObjID += 2

		objOffsets[pageObjID] = totalLen()
		cat(itoa(pageObjID) + " 0 obj\n")
		cat("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]")
		cat(" /Contents " + itoa(csObjID) + " 0 R")
		cat(" /Resources << /Font << /F1 " + itoa(fontObjID) + " 0 R >> >> >>\n")
		cat("endobj\n")

		objOffsets[csObjID] = totalLen()
		cat(itoa(csObjID) + " 0 obj\n<< /Length " + itoa(len(cs)) + " >>\nstream\n")
		catb(cs)
		cat("\nendstream\nendobj\n")
	}

	// Font object
	objOffsets[fontObjID] = totalLen()
	cat(itoa(fontObjID) + " 0 obj\n")
	cat("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>\n")
	cat("endobj\n")
	nextObjID = fontObjID + 1

	xrefOff := totalLen()
	cat("xref\n0 " + itoa(nextObjID) + "\n")
	cat("0000000000 65535 f \n")
	for id := 1; id < nextObjID; id++ {
		cat(padLeft(itoa(objOffsets[id]), 10) + " 00000 n \n")
	}
	cat("trailer\n<< /Size " + itoa(nextObjID) + " /Root 1 0 R >>\n")
	cat("startxref\n" + itoa(xrefOff) + "\n%%EOF\n")

	out := []byte{}
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

func padLeft(s string, width int) string {
	for len(s) < width {
		s = "0" + s
	}
	return s
}

func TestExtractSimpleText(t *testing.T) {
	cs := []byte("BT /F1 12 Tf 100 700 Td (Hello, World!) Tj ET")
	data := buildTestPDF([][]byte{cs})

	doc, err := Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	ext := NewExtractor(doc)
	text, err := ext.ExtractPage(0)
	if err != nil {
		t.Fatalf("ExtractPage: %v", err)
	}

	if !strings.Contains(text, "Hello, World!") {
		t.Errorf("expected 'Hello, World!' in output, got: %q", text)
	}
}

func TestExtractTJOperator(t *testing.T) {
	cs := []byte("BT /F1 14 Tf 50 750 Td [(Go) -200 (PDF)] TJ ET")
	data := buildTestPDF([][]byte{cs})

	doc, err := Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	ext := NewExtractor(doc)
	text, err := ext.ExtractPage(0)
	if err != nil {
		t.Fatalf("ExtractPage: %v", err)
	}

	if !strings.Contains(text, "Go") || !strings.Contains(text, "PDF") {
		t.Errorf("expected 'Go' and 'PDF' in output, got: %q", text)
	}
}

func TestMultiplePages(t *testing.T) {
	pages := [][]byte{
		[]byte("BT /F1 12 Tf 100 700 Td (Page one) Tj ET"),
		[]byte("BT /F1 12 Tf 100 700 Td (Page two) Tj ET"),
	}
	data := buildTestPDF(pages)

	doc, err := Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	allPages, err := doc.Pages()
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(allPages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(allPages))
	}

	ext := NewExtractor(doc)
	text0, _ := ext.ExtractPage(0)
	text1, _ := ext.ExtractPage(1)

	if !strings.Contains(text0, "Page one") {
		t.Errorf("page 0: expected 'Page one', got %q", text0)
	}
	if !strings.Contains(text1, "Page two") {
		t.Errorf("page 1: expected 'Page two', got %q", text1)
	}
}

func TestPageInfo(t *testing.T) {
	cs := []byte("BT ET")
	data := buildTestPDF([][]byte{cs})

	doc, err := Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	pages, err := doc.Pages()
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	info := doc.GetPageInfo(pages[0])
	if info.Width != 612 || info.Height != 792 {
		t.Errorf("expected 612x792, got %.0fx%.0f", info.Width, info.Height)
	}
}

func TestAsciiHexDecode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"48656c6c6f>", "Hello"},
		{"48 65 6c 6c 6f>", "Hello"},
		{"4865 6c6c 6f>", "Hello"},
	}
	for _, tt := range tests {
		result, err := asciiHexDecode([]byte(tt.input))
		if err != nil {
			t.Errorf("asciiHexDecode(%q): %v", tt.input, err)
			continue
		}
		if string(result) != tt.expected {
			t.Errorf("asciiHexDecode(%q): expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}

func TestRunLengthDecode(t *testing.T) {
	// Literal run: length byte 2 means copy next 3 bytes
	input := []byte{2, 'A', 'B', 'C', 128}
	result, err := runLengthDecode(input)
	if err != nil {
		t.Fatalf("runLengthDecode: %v", err)
	}
	if string(result) != "ABC" {
		t.Errorf("expected 'ABC', got %q", result)
	}

	// Repeated run: 253 means repeat next byte (257-253)=4 times
	input2 := []byte{253, 'X', 128}
	result2, err := runLengthDecode(input2)
	if err != nil {
		t.Fatalf("runLengthDecode2: %v", err)
	}
	if string(result2) != "XXXX" {
		t.Errorf("expected 'XXXX', got %q", result2)
	}
}

func TestWinAnsiEncoding(t *testing.T) {
	enc := &FontEncoding{isSimple: true, cmapChars: make(map[uint32]string)}
	enc.applyNamedEncoding("WinAnsiEncoding")

	// Euro sign is at code 128 in WinAnsi
	r := enc.codeToUnicode[128]
	if r != 0x20AC {
		t.Errorf("expected Euro sign (U+20AC) at code 128, got U+%04X", r)
	}
}

func TestDecodeNameEscapes(t *testing.T) {
	if got := decodeNameEscapes("A#20B"); got != "A B" {
		t.Errorf("expected 'A B', got %q", got)
	}
	if got := decodeNameEscapes("NoEscapes"); got != "NoEscapes" {
		t.Errorf("expected 'NoEscapes', got %q", got)
	}
}

func TestParserBasicTypes(t *testing.T) {
	data := []byte("null true false 42 3.14 (hello) <48454C4C4F> /Name [1 2 3]")
	p := NewParser(data, 0)

	// null
	obj, err := p.ParseObject()
	if err != nil || obj.Type != ObjNull {
		t.Errorf("expected null, got %v (err=%v)", obj.Type, err)
	}
	// true
	obj, err = p.ParseObject()
	if err != nil || obj.Type != ObjBool || !obj.Bool {
		t.Errorf("expected true, got %v %v", obj.Type, obj.Bool)
	}
	// false
	obj, err = p.ParseObject()
	if err != nil || obj.Type != ObjBool || obj.Bool {
		t.Errorf("expected false, got %v %v", obj.Type, obj.Bool)
	}
	// 42
	obj, err = p.ParseObject()
	if err != nil || obj.Type != ObjInt || obj.Int != 42 {
		t.Errorf("expected int 42, got %v %v", obj.Type, obj.Int)
	}
	// 3.14
	obj, err = p.ParseObject()
	if err != nil || obj.Type != ObjFloat {
		t.Errorf("expected float 3.14, got %v", obj.Type)
	}
	// (hello)
	obj, err = p.ParseObject()
	if err != nil || obj.Type != ObjString || string(obj.Str) != "hello" {
		t.Errorf("expected string 'hello', got %v %q", obj.Type, obj.Str)
	}
	// <48454C4C4F> = "HELLO"
	obj, err = p.ParseObject()
	if err != nil || obj.Type != ObjString || string(obj.Str) != "HELLO" {
		t.Errorf("expected hex string 'HELLO', got %v %q", obj.Type, obj.Str)
	}
	// /Name
	obj, err = p.ParseObject()
	if err != nil || obj.Type != ObjName || obj.Name != "Name" {
		t.Errorf("expected name 'Name', got %v %q", obj.Type, obj.Name)
	}
	// [1 2 3]
	obj, err = p.ParseObject()
	if err != nil || obj.Type != ObjArray || len(obj.Array) != 3 {
		t.Errorf("expected array of 3, got %v len=%d", obj.Type, len(obj.Array))
	}
}
