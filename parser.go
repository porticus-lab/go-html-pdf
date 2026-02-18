package htmlpdf

import (
	"bytes"
	"fmt"
	"strconv"
)

// ObjectType identifies the kind of a PDF object.
type ObjectType int

const (
	ObjNull ObjectType = iota
	ObjBool
	ObjInt
	ObjFloat
	ObjString
	ObjName
	ObjArray
	ObjDict
	ObjStream
	ObjRef
)

// Object holds any PDF object value.
type Object struct {
	Type   ObjectType
	Bool   bool
	Int    int64
	Float  float64
	Str    []byte
	Name   string
	Array  []*Object
	Dict   Dict
	Stream []byte // raw stream data
	Ref    Reference
}

// Reference is an indirect object reference (N G R).
type Reference struct {
	Number int
	Gen    int
}

// Dict is a PDF dictionary (name -> object).
type Dict map[string]*Object

// GetInt returns the integer value of a Dict entry.
func (d Dict) GetInt(key string) (int64, bool) {
	obj, ok := d[key]
	if !ok {
		return 0, false
	}
	if obj.Type == ObjInt {
		return obj.Int, true
	}
	if obj.Type == ObjFloat {
		return int64(obj.Float), true
	}
	return 0, false
}

// GetName returns the name value of a Dict entry.
func (d Dict) GetName(key string) (string, bool) {
	obj, ok := d[key]
	if !ok {
		return "", false
	}
	if obj.Type == ObjName {
		return obj.Name, true
	}
	if obj.Type == ObjString {
		return string(obj.Str), true
	}
	return "", false
}

// GetArray returns the array value of a Dict entry.
func (d Dict) GetArray(key string) ([]*Object, bool) {
	obj, ok := d[key]
	if !ok {
		return nil, false
	}
	if obj.Type == ObjArray {
		return obj.Array, true
	}
	// Single object treated as 1-element array
	return []*Object{obj}, true
}

// GetDict returns the dict value of a Dict entry.
func (d Dict) GetDict(key string) (Dict, bool) {
	obj, ok := d[key]
	if !ok {
		return nil, false
	}
	if obj.Type == ObjDict || obj.Type == ObjStream {
		return obj.Dict, true
	}
	return nil, false
}

const maxNesting = 100

// Parser is a recursive-descent PDF object parser.
type Parser struct {
	data  []byte
	pos   int
	depth int
}

// NewParser creates a parser for the given data at the given start position.
func NewParser(data []byte, pos int) *Parser {
	return &Parser{data: data, pos: pos}
}

// Pos returns the current parse position.
func (p *Parser) Pos() int { return p.pos }

// SetPos moves the parse position.
func (p *Parser) SetPos(pos int) { p.pos = pos }

// skipWhitespace skips spaces, tabs, CR, LF, and PDF comments.
func (p *Parser) skipWhitespace() {
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == '%' {
			// Skip comment to end of line
			for p.pos < len(p.data) && p.data[p.pos] != '\n' && p.data[p.pos] != '\r' {
				p.pos++
			}
		} else if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' {
			p.pos++
		} else {
			break
		}
	}
}

// peek returns the current byte without advancing.
func (p *Parser) peek() (byte, bool) {
	if p.pos >= len(p.data) {
		return 0, false
	}
	return p.data[p.pos], true
}

// match checks whether the upcoming bytes match s and advances past them if so.
func (p *Parser) match(s string) bool {
	end := p.pos + len(s)
	if end > len(p.data) {
		return false
	}
	if string(p.data[p.pos:end]) == s {
		p.pos = end
		return true
	}
	return false
}

// isDelim reports whether b is a PDF delimiter character.
func isDelim(b byte) bool {
	switch b {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

// isWhitespace reports whether b is a PDF whitespace character.
func isWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '\f', 0:
		return true
	}
	return false
}

// ParseObject parses one PDF object at the current position.
func (p *Parser) ParseObject() (*Object, error) {
	if p.depth > maxNesting {
		return nil, fmt.Errorf("exceeded maximum nesting depth")
	}
	p.depth++
	defer func() { p.depth-- }()

	p.skipWhitespace()
	if p.pos >= len(p.data) {
		return &Object{Type: ObjNull}, nil
	}

	c := p.data[p.pos]

	switch {
	case c == 'n' && p.match("null"):
		return &Object{Type: ObjNull}, nil
	case c == 't' && p.match("true"):
		return &Object{Type: ObjBool, Bool: true}, nil
	case c == 'f' && p.match("false"):
		return &Object{Type: ObjBool, Bool: false}, nil
	case c == '(':
		return p.parseString()
	case c == '<' && p.pos+1 < len(p.data) && p.data[p.pos+1] == '<':
		return p.parseDict()
	case c == '<':
		return p.parseHexString()
	case c == '/':
		return p.parseName()
	case c == '[':
		return p.parseArray()
	case c == '+' || c == '-' || c == '.' || (c >= '0' && c <= '9'):
		return p.parseNumberOrRef()
	default:
		// Unknown token - skip it
		return &Object{Type: ObjNull}, nil
	}
}

// parseString parses a literal string (...)
func (p *Parser) parseString() (*Object, error) {
	p.pos++ // consume '('
	var buf bytes.Buffer
	depth := 1
	for p.pos < len(p.data) && depth > 0 {
		c := p.data[p.pos]
		if c == '\\' {
			p.pos++
			if p.pos >= len(p.data) {
				break
			}
			esc := p.data[p.pos]
			p.pos++
			switch esc {
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 't':
				buf.WriteByte('\t')
			case 'b':
				buf.WriteByte('\b')
			case 'f':
				buf.WriteByte('\f')
			case '(':
				buf.WriteByte('(')
			case ')':
				buf.WriteByte(')')
			case '\\':
				buf.WriteByte('\\')
			case '\r':
				// Line continuation
				if p.pos < len(p.data) && p.data[p.pos] == '\n' {
					p.pos++
				}
			case '\n':
				// Line continuation
			default:
				if esc >= '0' && esc <= '7' {
					// Octal escape
					oct := int(esc - '0')
					for i := 0; i < 2 && p.pos < len(p.data); i++ {
						d := p.data[p.pos]
						if d < '0' || d > '7' {
							break
						}
						oct = oct*8 + int(d-'0')
						p.pos++
					}
					buf.WriteByte(byte(oct))
				} else {
					buf.WriteByte(esc)
				}
			}
		} else if c == '(' {
			depth++
			buf.WriteByte(c)
			p.pos++
		} else if c == ')' {
			depth--
			if depth > 0 {
				buf.WriteByte(c)
			}
			p.pos++
		} else {
			buf.WriteByte(c)
			p.pos++
		}
	}
	return &Object{Type: ObjString, Str: buf.Bytes()}, nil
}

// parseHexString parses a hex string <...>
func (p *Parser) parseHexString() (*Object, error) {
	p.pos++ // consume '<'
	var buf bytes.Buffer
	for p.pos < len(p.data) && p.data[p.pos] != '>' {
		p.skipWhitespace()
		if p.pos >= len(p.data) || p.data[p.pos] == '>' {
			break
		}
		hi := hexVal(p.data[p.pos])
		p.pos++
		var lo byte
		if p.pos < len(p.data) && p.data[p.pos] != '>' {
			lo = hexVal(p.data[p.pos])
			p.pos++
		}
		buf.WriteByte(hi<<4 | lo)
	}
	if p.pos < len(p.data) {
		p.pos++ // consume '>'
	}
	return &Object{Type: ObjString, Str: buf.Bytes()}, nil
}

func hexVal(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	}
	return 0
}

// parseName parses a PDF name /Foo
func (p *Parser) parseName() (*Object, error) {
	p.pos++ // consume '/'
	start := p.pos
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if isWhitespace(c) || isDelim(c) {
			break
		}
		p.pos++
	}
	name := string(p.data[start:p.pos])
	name = decodeNameEscapes(name)
	return &Object{Type: ObjName, Name: name}, nil
}

// decodeNameEscapes handles #XX hex escapes in PDF names.
func decodeNameEscapes(s string) string {
	if !bytes.ContainsRune([]byte(s), '#') {
		return s
	}
	var buf bytes.Buffer
	i := 0
	for i < len(s) {
		if s[i] == '#' && i+2 < len(s) {
			hi := hexVal(s[i+1])
			lo := hexVal(s[i+2])
			buf.WriteByte(hi<<4 | lo)
			i += 3
		} else {
			buf.WriteByte(s[i])
			i++
		}
	}
	return buf.String()
}

// parseArray parses [...]
func (p *Parser) parseArray() (*Object, error) {
	p.pos++ // consume '['
	var arr []*Object
	for {
		p.skipWhitespace()
		if p.pos >= len(p.data) {
			break
		}
		if p.data[p.pos] == ']' {
			p.pos++
			break
		}
		obj, err := p.ParseObject()
		if err != nil {
			return nil, err
		}
		arr = append(arr, obj)
	}
	return &Object{Type: ObjArray, Array: arr}, nil
}

// parseDict parses <<...>> and optionally a following stream.
func (p *Parser) parseDict() (*Object, error) {
	p.pos += 2 // consume '<<'
	d := make(Dict)
	for {
		p.skipWhitespace()
		if p.pos >= len(p.data) {
			break
		}
		if p.pos+1 < len(p.data) && p.data[p.pos] == '>' && p.data[p.pos+1] == '>' {
			p.pos += 2
			break
		}
		// Key must be a name
		if p.data[p.pos] != '/' {
			// Skip malformed token
			p.pos++
			continue
		}
		keyObj, err := p.parseName()
		if err != nil {
			return nil, err
		}
		valObj, err := p.ParseObject()
		if err != nil {
			return nil, err
		}
		d[keyObj.Name] = valObj
	}

	// Check for stream
	p.skipWhitespace()
	if !p.match("stream") {
		return &Object{Type: ObjDict, Dict: d}, nil
	}
	// Consume the newline after "stream"
	if p.pos < len(p.data) && p.data[p.pos] == '\r' {
		p.pos++
	}
	if p.pos < len(p.data) && p.data[p.pos] == '\n' {
		p.pos++
	}

	// Get stream length from dict
	streamStart := p.pos
	length := -1
	if lenObj, ok := d["Length"]; ok {
		if lenObj.Type == ObjInt {
			length = int(lenObj.Int)
		}
	}

	var streamData []byte
	if length >= 0 && streamStart+length <= len(p.data) {
		streamData = p.data[streamStart : streamStart+length]
		p.pos = streamStart + length
	} else {
		// Fallback: find endstream
		end := bytes.Index(p.data[streamStart:], []byte("endstream"))
		if end < 0 {
			end = len(p.data) - streamStart
		}
		streamData = p.data[streamStart : streamStart+end]
		p.pos = streamStart + end
	}

	p.skipWhitespace()
	p.match("endstream")

	return &Object{Type: ObjStream, Dict: d, Stream: streamData}, nil
}

// parseNumberOrRef parses a number or indirect reference (N G R).
func (p *Parser) parseNumberOrRef() (*Object, error) {
	saved := p.pos
	numStr := p.readToken()
	n, errN := strconv.ParseInt(numStr, 10, 64)

	// Check for possible reference: integer followed by integer followed by 'R'
	if errN == nil {
		savedAfterN := p.pos
		p.skipWhitespace()
		genStr := p.readToken()
		g, errG := strconv.ParseInt(genStr, 10, 64)
		if errG == nil {
			p.skipWhitespace()
			if p.pos < len(p.data) && p.data[p.pos] == 'R' {
				// Check it's followed by a delimiter or whitespace
				if p.pos+1 >= len(p.data) || isWhitespace(p.data[p.pos+1]) || isDelim(p.data[p.pos+1]) {
					p.pos++
					return &Object{Type: ObjRef, Ref: Reference{Number: int(n), Gen: int(g)}}, nil
				}
			}
		}
		// Not a reference, restore position after first number
		p.pos = savedAfterN
	}

	_ = saved
	// Try float
	if f, err := strconv.ParseFloat(numStr, 64); err == nil {
		if errN == nil {
			return &Object{Type: ObjInt, Int: n}, nil
		}
		return &Object{Type: ObjFloat, Float: f}, nil
	}
	if errN == nil {
		return &Object{Type: ObjInt, Int: n}, nil
	}
	return &Object{Type: ObjNull}, nil
}

// readToken reads a non-whitespace, non-delimiter token.
func (p *Parser) readToken() string {
	start := p.pos
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if isWhitespace(c) || isDelim(c) {
			break
		}
		p.pos++
	}
	return string(p.data[start:p.pos])
}
