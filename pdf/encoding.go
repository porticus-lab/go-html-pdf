package pdf

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// FontEncoding decodes PDF glyph codes to Unicode strings.
// Priority (highest to lowest): ToUnicode CMap > Encoding dict > Built-in tables.
type FontEncoding struct {
	// codeToUnicode maps single-byte glyph codes to Unicode runes (simple fonts)
	codeToUnicode [256]rune
	// cmapRanges holds ToUnicode CMap bf-range entries (multi-byte CID fonts)
	cmapRanges []cmapRange
	// cmapChars holds individual ToUnicode CMap bf-char entries
	cmapChars map[uint32]string
	isSimple  bool
}

type cmapRange struct {
	low, high uint32
	start     string // UTF-16BE of the starting unicode code point
}

// NewFontEncoding builds a FontEncoding from a PDF font object.
func NewFontEncoding(fontObj *Object) *FontEncoding {
	enc := &FontEncoding{
		isSimple:  true,
		cmapChars: make(map[uint32]string),
	}

	// Initialize to identity/standard mapping as baseline
	for i := 0; i < 256; i++ {
		enc.codeToUnicode[i] = rune(i)
	}

	if fontObj == nil || (fontObj.Type != ObjDict && fontObj.Type != ObjStream) {
		return enc
	}

	d := fontObj.Dict
	subtype, _ := d.GetName("Subtype")

	// Determine base encoding
	if encObj, ok := d["Encoding"]; ok {
		switch encObj.Type {
		case ObjName:
			enc.applyNamedEncoding(encObj.Name)
		case ObjDict, ObjStream:
			// Encoding dictionary with optional /BaseEncoding and /Differences
			if base, ok := encObj.Dict.GetName("BaseEncoding"); ok {
				enc.applyNamedEncoding(base)
			}
			if diffsObj, ok := encObj.Dict["Differences"]; ok && diffsObj.Type == ObjArray {
				enc.applyDifferences(diffsObj.Array)
			}
		}
	} else {
		// Default encoding depends on font subtype
		switch subtype {
		case "Type1", "MMType1":
			enc.applyNamedEncoding("StandardEncoding")
		default:
			enc.applyNamedEncoding("WinAnsiEncoding")
		}
	}

	// Check if this is a CID font (composite/Type0)
	if subtype == "Type0" {
		enc.isSimple = false
	}

	// Apply ToUnicode CMap if present (highest priority)
	if toUniObj, ok := d["ToUnicode"]; ok {
		if toUniObj.Type == ObjStream {
			enc.parseToUnicodeCMap(toUniObj.Stream)
		}
	}

	return enc
}

// applyNamedEncoding loads a standard PDF encoding table.
func (e *FontEncoding) applyNamedEncoding(name string) {
	var table [128]rune
	switch name {
	case "WinAnsiEncoding":
		table = winAnsiUpper128
	case "MacRomanEncoding":
		table = macRomanUpper128
	case "StandardEncoding":
		table = standardEncodingUpper128
	case "PDFDocEncoding":
		table = pdfDocEncodingUpper128
	default:
		return
	}
	for i, r := range table {
		if r != 0 {
			e.codeToUnicode[128+i] = r
		}
	}
}

// applyDifferences applies a /Differences array to override specific codes.
func (e *FontEncoding) applyDifferences(diffs []*Object) {
	code := 0
	for _, obj := range diffs {
		switch obj.Type {
		case ObjInt:
			code = int(obj.Int)
		case ObjName:
			if r, ok := glyphNameToRune(obj.Name); ok {
				if code >= 0 && code < 256 {
					e.codeToUnicode[code] = r
				}
			}
			code++
		}
	}
}

// parseToUnicodeCMap parses a ToUnicode CMap stream.
// Handles beginbfchar/endbfchar and beginbfrange/endbfrange sections.
func (e *FontEncoding) parseToUnicodeCMap(data []byte) {
	lines := bytes.Split(data, []byte("\n"))
	inBFChar := false
	inBFRange := false

	for _, rawLine := range lines {
		line := strings.TrimSpace(string(rawLine))

		switch {
		case strings.HasSuffix(line, "beginbfchar"):
			inBFChar = true
		case line == "endbfchar":
			inBFChar = false
		case strings.HasSuffix(line, "beginbfrange"):
			inBFRange = true
		case line == "endbfrange":
			inBFRange = false
		case inBFChar:
			e.parseBFCharEntry(line)
		case inBFRange:
			e.parseBFRangeEntry(line)
		}
	}
}

// parseBFCharEntry handles: <srcCode> <dstCode>
func (e *FontEncoding) parseBFCharEntry(line string) {
	tokens := parseCMapTokens(line)
	if len(tokens) < 2 {
		return
	}
	src := parseHexToken(tokens[0])
	dst := parseHexUTF16(tokens[1])
	if e.isSimple && src < 256 {
		runes := []rune(dst)
		if len(runes) > 0 {
			e.codeToUnicode[src] = runes[0]
		}
	} else {
		e.cmapChars[src] = dst
	}
}

// parseBFRangeEntry handles: <srcLow> <srcHigh> <dstStart>  (or array)
func (e *FontEncoding) parseBFRangeEntry(line string) {
	tokens := parseCMapTokens(line)
	if len(tokens) < 3 {
		return
	}
	low := parseHexToken(tokens[0])
	high := parseHexToken(tokens[1])

	// Destination can be a hex code or an array [<code1> <code2> ...]
	dst := tokens[2]
	if strings.HasPrefix(dst, "[") {
		// Array form: each element maps to successive codes
		// Collect array elements
		var arrTokens []string
		joined := strings.Join(tokens[2:], " ")
		joined = strings.TrimPrefix(joined, "[")
		joined = strings.TrimSuffix(joined, "]")
		arrTokens = parseCMapTokens(joined)
		for i, code := 0, low; code <= high; code, i = code+1, i+1 {
			if i < len(arrTokens) {
				s := parseHexUTF16(arrTokens[i])
				if e.isSimple && code < 256 {
					runes := []rune(s)
					if len(runes) > 0 {
						e.codeToUnicode[code] = runes[0]
					}
				} else {
					e.cmapChars[code] = s
				}
			}
		}
	} else {
		// Sequential range
		startStr := parseHexUTF16(dst)
		startRunes := []rune(startStr)
		var startCode rune
		if len(startRunes) > 0 {
			startCode = startRunes[0]
		}
		for code := low; code <= high; code++ {
			offset := rune(code - low)
			r := startCode + offset
			s := string(r)
			if e.isSimple && code < 256 {
				e.codeToUnicode[code] = r
			} else {
				e.cmapChars[code] = s
			}
		}
	}
}

// Decode converts a byte sequence from a PDF text string to a UTF-8 string.
func (e *FontEncoding) Decode(data []byte) string {
	if e.isSimple {
		var buf strings.Builder
		for _, b := range data {
			r := e.codeToUnicode[b]
			if r == 0 {
				r = rune(b)
			}
			if r > 0 && utf8.ValidRune(r) {
				buf.WriteRune(r)
			}
		}
		return buf.String()
	}
	// CID font: try 2-byte codes first, fall back to 1-byte
	var buf strings.Builder
	i := 0
	for i < len(data) {
		// Try 2-byte lookup
		if i+1 < len(data) {
			code := uint32(data[i])<<8 | uint32(data[i+1])
			if s, ok := e.cmapChars[code]; ok {
				buf.WriteString(s)
				i += 2
				continue
			}
		}
		// Try 1-byte lookup
		code := uint32(data[i])
		if s, ok := e.cmapChars[code]; ok {
			buf.WriteString(s)
		} else {
			r := e.codeToUnicode[data[i]]
			if r > 0 && utf8.ValidRune(r) {
				buf.WriteRune(r)
			}
		}
		i++
	}
	return buf.String()
}

// parseCMapTokens splits a CMap line into hex tokens and other tokens.
func parseCMapTokens(line string) []string {
	var tokens []string
	i := 0
	for i < len(line) {
		if line[i] == ' ' || line[i] == '\t' || line[i] == '\r' {
			i++
			continue
		}
		if line[i] == '<' {
			// Hex token
			j := strings.Index(line[i+1:], ">")
			if j < 0 {
				break
			}
			tokens = append(tokens, line[i:i+j+2])
			i = i + j + 2
		} else if line[i] == '[' {
			// Array: collect until ]
			j := strings.Index(line[i:], "]")
			if j < 0 {
				tokens = append(tokens, line[i:])
				break
			}
			tokens = append(tokens, line[i:i+j+1])
			i = i + j + 1
		} else {
			// Regular token
			j := i
			for j < len(line) && line[j] != ' ' && line[j] != '\t' {
				j++
			}
			tokens = append(tokens, line[i:j])
			i = j
		}
	}
	return tokens
}

// parseHexToken parses a <HHHH> hex token to a uint32 code.
func parseHexToken(s string) uint32 {
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	s = strings.TrimSpace(s)
	var v uint32
	for _, c := range s {
		v <<= 4
		v |= uint32(hexValRune(c))
	}
	return v
}

// parseHexUTF16 parses a <HHHH> hex token as UTF-16BE and returns UTF-8.
func parseHexUTF16(s string) string {
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return ""
	}

	// Ensure even number of hex digits (UTF-16BE = 2 bytes per unit)
	if len(s)%4 != 0 && len(s)%2 == 0 {
		// Might be single byte
		var b byte
		for _, c := range s {
			b = b<<4 | byte(hexValRune(c))
		}
		return string(rune(b))
	}

	// Parse as UTF-16BE
	var utf16Units []uint16
	for i := 0; i+3 < len(s); i += 4 {
		hi := hexValRune(rune(s[i]))
		lo1 := hexValRune(rune(s[i+1]))
		lo2 := hexValRune(rune(s[i+2]))
		lo3 := hexValRune(rune(s[i+3]))
		unit := uint16(hi)<<12 | uint16(lo1)<<8 | uint16(lo2)<<4 | uint16(lo3)
		utf16Units = append(utf16Units, unit)
	}

	return utf16ToString(utf16Units)
}

// utf16ToString converts UTF-16 code units (with surrogate pair support) to UTF-8.
func utf16ToString(units []uint16) string {
	var buf strings.Builder
	for i := 0; i < len(units); i++ {
		u := units[i]
		if u >= 0xD800 && u <= 0xDBFF && i+1 < len(units) {
			// High surrogate
			low := units[i+1]
			if low >= 0xDC00 && low <= 0xDFFF {
				r := rune(u-0xD800)<<10 | rune(low-0xDC00) + 0x10000
				buf.WriteRune(r)
				i++
				continue
			}
		}
		buf.WriteRune(rune(u))
	}
	return buf.String()
}

func hexValRune(r rune) byte {
	switch {
	case r >= '0' && r <= '9':
		return byte(r - '0')
	case r >= 'a' && r <= 'f':
		return byte(r-'a') + 10
	case r >= 'A' && r <= 'F':
		return byte(r-'A') + 10
	}
	return 0
}

// glyphNameToRune maps Adobe glyph names to Unicode runes.
// This covers the most common glyphs.
func glyphNameToRune(name string) (rune, bool) {
	r, ok := adobeGlyphList[name]
	return r, ok
}

// ---- Standard encoding tables ----
// Each array covers codes 128-255 (index 0 = code 128).
// Zero means "undefined / use code directly".

// winAnsiUpper128 is the Windows-1252 upper half.
var winAnsiUpper128 = [128]rune{
	0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, // 128-135
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0, // 136-143
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, // 144-151
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178, // 152-159
	0x00A0, 0x00A1, 0x00A2, 0x00A3, 0x00A4, 0x00A5, 0x00A6, 0x00A7, // 160-167
	0x00A8, 0x00A9, 0x00AA, 0x00AB, 0x00AC, 0x00AD, 0x00AE, 0x00AF, // 168-175
	0x00B0, 0x00B1, 0x00B2, 0x00B3, 0x00B4, 0x00B5, 0x00B6, 0x00B7, // 176-183
	0x00B8, 0x00B9, 0x00BA, 0x00BB, 0x00BC, 0x00BD, 0x00BE, 0x00BF, // 184-191
	0x00C0, 0x00C1, 0x00C2, 0x00C3, 0x00C4, 0x00C5, 0x00C6, 0x00C7, // 192-199
	0x00C8, 0x00C9, 0x00CA, 0x00CB, 0x00CC, 0x00CD, 0x00CE, 0x00CF, // 200-207
	0x00D0, 0x00D1, 0x00D2, 0x00D3, 0x00D4, 0x00D5, 0x00D6, 0x00D7, // 208-215
	0x00D8, 0x00D9, 0x00DA, 0x00DB, 0x00DC, 0x00DD, 0x00DE, 0x00DF, // 216-223
	0x00E0, 0x00E1, 0x00E2, 0x00E3, 0x00E4, 0x00E5, 0x00E6, 0x00E7, // 224-231
	0x00E8, 0x00E9, 0x00EA, 0x00EB, 0x00EC, 0x00ED, 0x00EE, 0x00EF, // 232-239
	0x00F0, 0x00F1, 0x00F2, 0x00F3, 0x00F4, 0x00F5, 0x00F6, 0x00F7, // 240-247
	0x00F8, 0x00F9, 0x00FA, 0x00FB, 0x00FC, 0x00FD, 0x00FE, 0x00FF, // 248-255
}

// macRomanUpper128 is the Mac Roman upper half.
var macRomanUpper128 = [128]rune{
	0x00C4, 0x00C5, 0x00C7, 0x00C9, 0x00D1, 0x00D6, 0x00DC, 0x00E1, // 128-135
	0x00E0, 0x00E2, 0x00E4, 0x00E5, 0x00E7, 0x00E9, 0x00E8, 0x00EA, // 136-143
	0x00EB, 0x00ED, 0x00EC, 0x00EE, 0x00EF, 0x00F1, 0x00F3, 0x00F2, // 144-151
	0x00F4, 0x00F6, 0x00FA, 0x00F9, 0x00FB, 0x00FC, 0x2020, 0x00B0, // 152-159
	0x00A2, 0x00A3, 0x00A7, 0x2022, 0x00B6, 0x00DF, 0x00AE, 0x00A9, // 160-167
	0x2122, 0x00B4, 0x00A8, 0x2260, 0x00C6, 0x00D8, 0x221E, 0x00B1, // 168-175
	0x2264, 0x2265, 0x00A5, 0x00B5, 0x2202, 0x2211, 0x220F, 0x03C0, // 176-183
	0x222B, 0x00AA, 0x00BA, 0x03A9, 0x00E6, 0x00F8, 0x00BF, 0x00A1, // 184-191
	0x00AC, 0x221A, 0x0192, 0x2248, 0x2206, 0x00AB, 0x00BB, 0x2026, // 192-199
	0x00A0, 0x00C0, 0x00C3, 0x00D5, 0x0152, 0x0153, 0x2013, 0x2014, // 200-207
	0x201C, 0x201D, 0x2018, 0x2019, 0x00F7, 0x25CA, 0x00FF, 0x0178, // 208-215
	0x2044, 0x20AC, 0x2039, 0x203A, 0xFB01, 0xFB02, 0x2021, 0x00B7, // 216-223
	0x201A, 0x201E, 0x2030, 0x00C2, 0x00CA, 0x00C1, 0x00CB, 0x00C8, // 224-231
	0x00CD, 0x00CE, 0x00CF, 0x00CC, 0x00D3, 0x00D4, 0xF8FF, 0x00D2, // 232-239
	0x00DA, 0x00DB, 0x00D9, 0x0131, 0x02C6, 0x02DC, 0x00AF, 0x02D8, // 240-247
	0x02D9, 0x02DA, 0x00B8, 0x02DD, 0x02DB, 0x02C7, 0, 0, // 248-255
}

// standardEncodingUpper128 is PostScript Standard Encoding upper half.
var standardEncodingUpper128 = [128]rune{
	0, 0, 0, 0, 0, 0, 0, 0, // 128-135
	0, 0, 0, 0, 0, 0, 0, 0, // 136-143
	0, 0, 0, 0, 0, 0, 0, 0, // 144-151
	0, 0, 0, 0, 0, 0, 0, 0, // 152-159
	0, 0x00A1, 0x00A2, 0x00A3, 0x2044, 0x00A5, 0x0192, 0x00A7, // 160-167
	0x00A4, 0x0027, 0x201C, 0x00AB, 0x2039, 0x203A, 0xFB01, 0xFB02, // 168-175
	0, 0x2013, 0x2020, 0x2021, 0x00B7, 0, 0x00B6, 0x2022, // 176-183
	0x201A, 0x201E, 0x201D, 0x00BB, 0x2026, 0x2030, 0, 0x00BF, // 184-191
	0, 0x0060, 0x00B4, 0x02C6, 0x02DC, 0x00AF, 0x02D8, 0x02D9, // 192-199
	0x00A8, 0, 0x02DA, 0x00B8, 0, 0x02DD, 0x02DB, 0x02C7, // 200-207
	0x2014, 0, 0, 0, 0, 0, 0, 0, // 208-215
	0, 0, 0, 0, 0, 0, 0, 0, // 216-223
	0, 0x00C6, 0, 0x00AA, 0, 0, 0, 0, // 224-231
	0x0141, 0x00D8, 0x0152, 0x00BA, 0, 0, 0, 0, // 232-239
	0, 0x00E6, 0, 0, 0, 0x0131, 0, 0, // 240-247
	0x0142, 0x00F8, 0x0153, 0x00DF, 0, 0, 0, 0, // 248-255
}

// pdfDocEncodingUpper128 is PDFDocEncoding upper half.
var pdfDocEncodingUpper128 = [128]rune{
	0x02D8, 0x02C7, 0x02C6, 0x02D9, 0x02DD, 0x02DB, 0x02DA, 0x02DC, // 128-135
	0x2013, 0x2014, 0x2018, 0x2019, 0x201C, 0x201D, 0x2039, 0x203A, // 136-143
	0x2026, 0x2030, 0x2020, 0x2021, 0x2022, 0x2122, 0x0192, 0x2044, // 144-151
	0x2212, 0xFB01, 0xFB02, 0x0141, 0x0152, 0x0160, 0x0178, 0x017D, // 152-159
	0x00A0, 0x00A1, 0x00A2, 0x00A3, 0x00A4, 0x00A5, 0x00A6, 0x00A7, // 160-167
	0x00A8, 0x00A9, 0x00AA, 0x00AB, 0x00AC, 0x00AD, 0x00AE, 0x00AF, // 168-175
	0x00B0, 0x00B1, 0x00B2, 0x00B3, 0x00B4, 0x00B5, 0x00B6, 0x00B7, // 176-183
	0x00B8, 0x00B9, 0x00BA, 0x00BB, 0x00BC, 0x00BD, 0x00BE, 0x00BF, // 184-191
	0x00C0, 0x00C1, 0x00C2, 0x00C3, 0x00C4, 0x00C5, 0x00C6, 0x00C7, // 192-199
	0x00C8, 0x00C9, 0x00CA, 0x00CB, 0x00CC, 0x00CD, 0x00CE, 0x00CF, // 200-207
	0x00D0, 0x00D1, 0x00D2, 0x00D3, 0x00D4, 0x00D5, 0x00D6, 0x00D7, // 208-215
	0x00D8, 0x00D9, 0x00DA, 0x00DB, 0x00DC, 0x00DD, 0x00DE, 0x00DF, // 216-223
	0x00E0, 0x00E1, 0x00E2, 0x00E3, 0x00E4, 0x00E5, 0x00E6, 0x00E7, // 224-231
	0x00E8, 0x00E9, 0x00EA, 0x00EB, 0x00EC, 0x00ED, 0x00EE, 0x00EF, // 232-239
	0x00F0, 0x00F1, 0x00F2, 0x00F3, 0x00F4, 0x00F5, 0x00F6, 0x00F7, // 240-247
	0x00F8, 0x00F9, 0x00FA, 0x00FB, 0x00FC, 0x00FD, 0x00FE, 0x00FF, // 248-255
}

// adobeGlyphList maps Adobe glyph names to Unicode code points.
// This is a subset covering the most common glyphs.
var adobeGlyphList = map[string]rune{
	"A": 0x0041, "B": 0x0042, "C": 0x0043, "D": 0x0044, "E": 0x0045,
	"F": 0x0046, "G": 0x0047, "H": 0x0048, "I": 0x0049, "J": 0x004A,
	"K": 0x004B, "L": 0x004C, "M": 0x004D, "N": 0x004E, "O": 0x004F,
	"P": 0x0050, "Q": 0x0051, "R": 0x0052, "S": 0x0053, "T": 0x0054,
	"U": 0x0055, "V": 0x0056, "W": 0x0057, "X": 0x0058, "Y": 0x0059,
	"Z": 0x005A,
	"a": 0x0061, "b": 0x0062, "c": 0x0063, "d": 0x0064, "e": 0x0065,
	"f": 0x0066, "g": 0x0067, "h": 0x0068, "i": 0x0069, "j": 0x006A,
	"k": 0x006B, "l": 0x006C, "m": 0x006D, "n": 0x006E, "o": 0x006F,
	"p": 0x0070, "q": 0x0071, "r": 0x0072, "s": 0x0073, "t": 0x0074,
	"u": 0x0075, "v": 0x0076, "w": 0x0077, "x": 0x0078, "y": 0x0079,
	"z": 0x007A,
	"zero": 0x0030, "one": 0x0031, "two": 0x0032, "three": 0x0033,
	"four": 0x0034, "five": 0x0035, "six": 0x0036, "seven": 0x0037,
	"eight": 0x0038, "nine": 0x0039,
	"space": 0x0020, "exclam": 0x0021, "quotedbl": 0x0022, "numbersign": 0x0023,
	"dollar": 0x0024, "percent": 0x0025, "ampersand": 0x0026, "quotesingle": 0x0027,
	"parenleft": 0x0028, "parenright": 0x0029, "asterisk": 0x002A, "plus": 0x002B,
	"comma": 0x002C, "hyphen": 0x002D, "period": 0x002E, "slash": 0x002F,
	"colon": 0x003A, "semicolon": 0x003B, "less": 0x003C, "equal": 0x003D,
	"greater": 0x003E, "question": 0x003F, "at": 0x0040,
	"bracketleft": 0x005B, "backslash": 0x005C, "bracketright": 0x005D,
	"asciicircum": 0x005E, "underscore": 0x005F, "grave": 0x0060,
	"braceleft": 0x007B, "bar": 0x007C, "braceright": 0x007D, "asciitilde": 0x007E,
	// Accented chars
	"Aacute": 0x00C1, "Agrave": 0x00C0, "Acircumflex": 0x00C2, "Atilde": 0x00C3,
	"Adieresis": 0x00C4, "Aring": 0x00C5, "AE": 0x00C6, "Ccedilla": 0x00C7,
	"Eacute": 0x00C9, "Egrave": 0x00C8, "Ecircumflex": 0x00CA, "Edieresis": 0x00CB,
	"Iacute": 0x00CD, "Igrave": 0x00CC, "Icircumflex": 0x00CE, "Idieresis": 0x00CF,
	"Eth": 0x00D0, "Ntilde": 0x00D1, "Oacute": 0x00D3, "Ograve": 0x00D2,
	"Ocircumflex": 0x00D4, "Otilde": 0x00D5, "Odieresis": 0x00D6, "multiply": 0x00D7,
	"Oslash": 0x00D8, "Uacute": 0x00DA, "Ugrave": 0x00D9, "Ucircumflex": 0x00DB,
	"Udieresis": 0x00DC, "Yacute": 0x00DD, "Thorn": 0x00DE, "germandbls": 0x00DF,
	"aacute": 0x00E1, "agrave": 0x00E0, "acircumflex": 0x00E2, "atilde": 0x00E3,
	"adieresis": 0x00E4, "aring": 0x00E5, "ae": 0x00E6, "ccedilla": 0x00E7,
	"eacute": 0x00E9, "egrave": 0x00E8, "ecircumflex": 0x00EA, "edieresis": 0x00EB,
	"iacute": 0x00ED, "igrave": 0x00EC, "icircumflex": 0x00EE, "idieresis": 0x00EF,
	"eth": 0x00F0, "ntilde": 0x00F1, "oacute": 0x00F3, "ograve": 0x00F2,
	"ocircumflex": 0x00F4, "otilde": 0x00F5, "odieresis": 0x00F6, "divide": 0x00F7,
	"oslash": 0x00F8, "uacute": 0x00FA, "ugrave": 0x00F9, "ucircumflex": 0x00FB,
	"udieresis": 0x00FC, "yacute": 0x00FD, "thorn": 0x00FE, "ydieresis": 0x00FF,
	// Punctuation and symbols
	"endash": 0x2013, "emdash": 0x2014, "quotesinglbase": 0x201A,
	"quotedblbase": 0x201E, "quotedblleft": 0x201C, "quotedblright": 0x201D,
	"quoteleft": 0x2018, "quoteright": 0x2019, "ellipsis": 0x2026,
	"dagger": 0x2020, "daggerdbl": 0x2021, "bullet": 0x2022,
	"perthousand": 0x2030, "guilsinglleft": 0x2039, "guilsinglright": 0x203A,
	"guillemotleft": 0x00AB, "guillemotright": 0x00BB,
	"trademark": 0x2122, "fi": 0xFB01, "fl": 0xFB02,
	"florin": 0x0192, "fraction": 0x2044,
	"Euro": 0x20AC, "currency": 0x00A4,
	"copyright": 0x00A9, "registered": 0x00AE,
	"degree": 0x00B0, "plusminus": 0x00B1, "mu": 0x00B5,
	"paragraph": 0x00B6, "periodcentered": 0x00B7,
	"cedilla": 0x00B8, "ordmasculine": 0x00BA, "ordfeminine": 0x00AA,
	"nobreakspace": 0x00A0, "softhyphen": 0x00AD,
	"OE": 0x0152, "oe": 0x0153, "Scaron": 0x0160, "scaron": 0x0161,
	"Zcaron": 0x017D, "zcaron": 0x017E, "Ydieresis": 0x0178,
	"circumflex": 0x02C6, "tilde": 0x02DC, "macron": 0x00AF,
	"breve": 0x02D8, "dotaccent": 0x02D9, "dieresis": 0x00A8,
	"ring": 0x02DA, "hungarumlaut": 0x02DD, "ogonek": 0x02DB, "caron": 0x02C7,
	"Lslash": 0x0141, "lslash": 0x0142, "dotlessi": 0x0131,
}
