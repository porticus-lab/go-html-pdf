package pdf

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// XRefEntry describes one entry in the cross-reference table.
type XRefEntry struct {
	Offset     int64
	Generation int
	InUse      bool
	// For compressed objects (PDF 1.5+)
	Compressed  bool
	StreamObjID int
	IndexInStrm int
}

// Document represents a loaded PDF file.
type Document struct {
	data    []byte
	xref    map[int]XRefEntry
	trailer Dict
	cache   map[int]*Object // resolved indirect objects
}

// Open reads a PDF file from disk.
func Open(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	return Load(data)
}

// Load parses a PDF from raw bytes.
func Load(data []byte) (*Document, error) {
	doc := &Document{
		data:  data,
		xref:  make(map[int]XRefEntry),
		cache: make(map[int]*Object),
	}
	if err := doc.validateHeader(); err != nil {
		return nil, err
	}
	if err := doc.loadXRef(); err != nil {
		return nil, fmt.Errorf("loading xref: %w", err)
	}
	return doc, nil
}

// validateHeader checks the %PDF-n.n header.
func (doc *Document) validateHeader() error {
	if !bytes.HasPrefix(doc.data, []byte("%PDF-")) {
		return fmt.Errorf("not a PDF file")
	}
	return nil
}

// Version returns the PDF version string (e.g. "1.7").
func (doc *Document) Version() string {
	if len(doc.data) < 8 {
		return "?"
	}
	end := bytes.IndexByte(doc.data[5:20], '\n')
	if end < 0 {
		end = 5
	}
	v := string(doc.data[5 : 5+end])
	v = strings.TrimRight(v, "\r\n ")
	return v
}

// loadXRef finds startxref, then loads the xref table/stream and trailer.
func (doc *Document) loadXRef() error {
	offset, err := doc.findStartXRef()
	if err != nil {
		return err
	}
	return doc.loadXRefAt(offset)
}

// findStartXRef scans backward to locate "startxref" and reads the offset.
func (doc *Document) findStartXRef() (int64, error) {
	searchFrom := len(doc.data) - 1024
	if searchFrom < 0 {
		searchFrom = 0
	}
	idx := bytes.LastIndex(doc.data[searchFrom:], []byte("startxref"))
	if idx < 0 {
		return 0, fmt.Errorf("startxref not found")
	}
	pos := searchFrom + idx + len("startxref")
	for pos < len(doc.data) && isWhitespace(doc.data[pos]) {
		pos++
	}
	end := pos
	for end < len(doc.data) && doc.data[end] >= '0' && doc.data[end] <= '9' {
		end++
	}
	if end == pos {
		return 0, fmt.Errorf("invalid startxref value")
	}
	offset, err := strconv.ParseInt(string(doc.data[pos:end]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing startxref: %w", err)
	}
	return offset, nil
}

// loadXRefAt loads the xref section (table or stream) at the given file offset.
func (doc *Document) loadXRefAt(offset int64) error {
	if offset < 0 || int(offset) >= len(doc.data) {
		return fmt.Errorf("xref offset out of bounds: %d", offset)
	}

	p := NewParser(doc.data, int(offset))
	p.skipWhitespace()

	// Traditional xref table starts with "xref"
	if p.match("xref") {
		return doc.parseXRefTable(p)
	}

	// Otherwise it's a cross-reference stream (PDF 1.5+)
	return doc.parseXRefStream(p)
}

// parseXRefTable parses the classic "xref" keyword + subsections + trailer.
func (doc *Document) parseXRefTable(p *Parser) error {
	for {
		p.skipWhitespace()
		if p.pos >= len(doc.data) {
			break
		}
		if bytes.HasPrefix(doc.data[p.pos:], []byte("trailer")) {
			p.SetPos(p.Pos() + len("trailer"))
			break
		}
		firstStr := p.readToken()
		p.skipWhitespace()
		countStr := p.readToken()
		first, err1 := strconv.Atoi(firstStr)
		count, err2 := strconv.Atoi(countStr)
		if err1 != nil || err2 != nil {
			break
		}
		p.skipWhitespace()
		// Each entry is exactly 20 bytes: "nnnnnnnnnn ggggg n/f\r\n"
		for i := 0; i < count; i++ {
			id := first + i
			if p.Pos()+20 > len(doc.data) {
				break
			}
			entry := string(doc.data[p.Pos() : p.Pos()+20])
			p.SetPos(p.Pos() + 20)
			if len(entry) < 18 {
				continue
			}
			off, _ := strconv.ParseInt(strings.TrimSpace(entry[:10]), 10, 64)
			gen, _ := strconv.Atoi(strings.TrimSpace(entry[11:16]))
			inUse := entry[17] == 'n'
			if _, exists := doc.xref[id]; !exists {
				doc.xref[id] = XRefEntry{
					Offset:     off,
					Generation: gen,
					InUse:      inUse,
				}
			}
		}
	}

	// Parse trailer dict
	p.skipWhitespace()
	trailerObj, err := p.ParseObject()
	if err != nil {
		return fmt.Errorf("parsing trailer: %w", err)
	}
	if doc.trailer == nil && trailerObj.Type == ObjDict {
		doc.trailer = trailerObj.Dict
	}

	if prev, ok := doc.trailer.GetInt("Prev"); ok && prev > 0 {
		return doc.loadXRefAt(prev)
	}
	return nil
}

// parseXRefStream handles a cross-reference stream object (PDF 1.5+).
func (doc *Document) parseXRefStream(p *Parser) error {
	p.readToken() // object number
	p.skipWhitespace()
	p.readToken() // generation
	p.skipWhitespace()
	p.match("obj")
	p.skipWhitespace()

	obj, err := p.ParseObject()
	if err != nil {
		return fmt.Errorf("parsing xref stream object: %w", err)
	}
	if obj.Type != ObjStream {
		return fmt.Errorf("xref at offset is not a stream")
	}

	if doc.trailer == nil {
		doc.trailer = obj.Dict
	}

	streamData, err := DecompressStream(obj.Dict, obj.Stream)
	if err != nil {
		return fmt.Errorf("decompressing xref stream: %w", err)
	}

	w, _ := obj.Dict.GetArray("W")
	if len(w) < 3 {
		return fmt.Errorf("xref stream missing /W")
	}
	w1 := int(w[0].Int)
	w2 := int(w[1].Int)
	w3 := int(w[2].Int)
	entrySize := w1 + w2 + w3
	if entrySize == 0 {
		return fmt.Errorf("xref stream zero entry size")
	}

	size, _ := obj.Dict.GetInt("Size")
	indexArr, hasIndex := obj.Dict.GetArray("Index")
	var subsections [][2]int
	if hasIndex {
		for i := 0; i+1 < len(indexArr); i += 2 {
			first := int(indexArr[i].Int)
			count := int(indexArr[i+1].Int)
			subsections = append(subsections, [2]int{first, count})
		}
	} else {
		subsections = [][2]int{{0, int(size)}}
	}

	offset := 0
	for _, sub := range subsections {
		first, count := sub[0], sub[1]
		for i := 0; i < count; i++ {
			if offset+entrySize > len(streamData) {
				break
			}
			id := first + i
			t := readBigEndian(streamData[offset:], w1)
			f2 := readBigEndian(streamData[offset+w1:], w2)
			f3 := readBigEndian(streamData[offset+w1+w2:], w3)
			offset += entrySize

			if _, exists := doc.xref[id]; exists {
				continue
			}
			switch t {
			case 0:
				doc.xref[id] = XRefEntry{Generation: f3}
			case 1:
				doc.xref[id] = XRefEntry{Offset: int64(f2), Generation: f3, InUse: true}
			case 2:
				doc.xref[id] = XRefEntry{
					Compressed:  true,
					StreamObjID: f2,
					IndexInStrm: f3,
					InUse:       true,
				}
			}
		}
	}

	if prev, ok := obj.Dict.GetInt("Prev"); ok && prev > 0 {
		return doc.loadXRefAt(prev)
	}
	return nil
}

// readBigEndian reads n bytes as a big-endian integer.
func readBigEndian(data []byte, n int) int {
	v := 0
	for i := 0; i < n && i < len(data); i++ {
		v = v<<8 | int(data[i])
	}
	return v
}

// ResolveRef follows an indirect reference and returns the pointed-to object.
func (doc *Document) ResolveRef(ref Reference) (*Object, error) {
	if obj, ok := doc.cache[ref.Number]; ok {
		return obj, nil
	}
	entry, ok := doc.xref[ref.Number]
	if !ok || !entry.InUse {
		return &Object{Type: ObjNull}, nil
	}

	var obj *Object
	var err error
	if entry.Compressed {
		obj, err = doc.resolveCompressed(entry)
	} else {
		obj, err = doc.resolveAtOffset(entry.Offset)
	}
	if err != nil {
		return &Object{Type: ObjNull}, nil
	}
	doc.cache[ref.Number] = obj
	return obj, nil
}

// resolveAtOffset parses "N G obj ... endobj" at the given byte offset.
func (doc *Document) resolveAtOffset(offset int64) (*Object, error) {
	if offset < 0 || int(offset) >= len(doc.data) {
		return nil, fmt.Errorf("object offset %d out of bounds", offset)
	}
	p := NewParser(doc.data, int(offset))
	p.readToken() // object number
	p.skipWhitespace()
	p.readToken() // generation
	p.skipWhitespace()
	if !p.match("obj") {
		return nil, fmt.Errorf("expected 'obj' at offset %d", offset)
	}

	obj, err := p.ParseObject()
	if err != nil {
		return nil, err
	}

	// If stream /Length is an indirect ref, resolve and re-parse
	if obj.Type == ObjStream {
		if lenRef, ok := obj.Dict["Length"]; ok && lenRef.Type == ObjRef {
			lenObj, _ := doc.ResolveRef(lenRef.Ref)
			if lenObj != nil && lenObj.Type == ObjInt {
				obj.Dict["Length"] = lenObj
				p2 := NewParser(doc.data, int(offset))
				p2.readToken()
				p2.skipWhitespace()
				p2.readToken()
				p2.skipWhitespace()
				p2.match("obj")
				obj, err = p2.ParseObject()
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return obj, nil
}

// resolveCompressed reads an object stored inside an object stream (PDF 1.5+).
func (doc *Document) resolveCompressed(entry XRefEntry) (*Object, error) {
	strmObj, err := doc.ResolveRef(Reference{Number: entry.StreamObjID})
	if err != nil {
		return nil, err
	}
	if strmObj.Type != ObjStream {
		return nil, fmt.Errorf("compressed object container is not a stream")
	}

	data, err := DecompressStream(strmObj.Dict, strmObj.Stream)
	if err != nil {
		return nil, err
	}

	n, _ := strmObj.Dict.GetInt("N")
	first, _ := strmObj.Dict.GetInt("First")

	p := NewParser(data, 0)
	offsets := make(map[int]int)
	for i := 0; i < int(n); i++ {
		p.skipWhitespace()
		idStr := p.readToken()
		p.skipWhitespace()
		offStr := p.readToken()
		id, _ := strconv.Atoi(idStr)
		off, _ := strconv.Atoi(offStr)
		offsets[id] = off
	}

	off, ok := offsets[entry.StreamObjID]
	if !ok {
		off = entry.IndexInStrm
	}
	objPos := int(first) + off
	if objPos > len(data) {
		objPos = int(first) + entry.IndexInStrm
	}
	p2 := NewParser(data, objPos)
	return p2.ParseObject()
}

// Resolve returns the object, following any indirect reference.
func (doc *Document) Resolve(obj *Object) (*Object, error) {
	if obj == nil || obj.Type != ObjRef {
		return obj, nil
	}
	return doc.ResolveRef(obj.Ref)
}

// Catalog returns the document catalog dictionary.
func (doc *Document) Catalog() (Dict, error) {
	rootRef, ok := doc.trailer["Root"]
	if !ok {
		return nil, fmt.Errorf("no /Root in trailer")
	}
	root, err := doc.Resolve(rootRef)
	if err != nil {
		return nil, err
	}
	if root.Type != ObjDict {
		return nil, fmt.Errorf("root is not a dict")
	}
	return root.Dict, nil
}

// Pages returns all page dictionaries in order.
func (doc *Document) Pages() ([]Dict, error) {
	cat, err := doc.Catalog()
	if err != nil {
		return nil, err
	}
	pagesRef, ok := cat["Pages"]
	if !ok {
		return nil, fmt.Errorf("no /Pages in catalog")
	}
	pagesObj, err := doc.Resolve(pagesRef)
	if err != nil {
		return nil, err
	}
	var pages []Dict
	doc.collectPages(pagesObj.Dict, &pages)
	return pages, nil
}

// collectPages recursively collects all leaf Page dicts from a Pages tree.
func (doc *Document) collectPages(node Dict, pages *[]Dict) {
	typeObj, _ := node.GetName("Type")
	if typeObj == "Page" {
		*pages = append(*pages, node)
		return
	}
	kidsObj, ok := node["Kids"]
	if !ok {
		return
	}
	kids, err := doc.Resolve(kidsObj)
	if err != nil || kids.Type != ObjArray {
		return
	}
	for _, kidRef := range kids.Array {
		kidObj, err := doc.Resolve(kidRef)
		if err != nil || kidObj == nil {
			continue
		}
		if kidObj.Type == ObjDict || kidObj.Type == ObjStream {
			doc.collectPages(kidObj.Dict, pages)
		}
	}
}

// ContentStreams returns the combined decompressed content stream data for a page.
func (doc *Document) ContentStreams(page Dict) ([]byte, error) {
	contentsObj, ok := page["Contents"]
	if !ok {
		return nil, nil
	}
	contents, err := doc.Resolve(contentsObj)
	if err != nil {
		return nil, err
	}

	var result []byte
	streams := []*Object{contents}
	if contents.Type == ObjArray {
		streams = contents.Array
	}
	for _, s := range streams {
		resolved, err := doc.Resolve(s)
		if err != nil {
			continue
		}
		if resolved.Type != ObjStream {
			continue
		}
		data, err := DecompressStream(resolved.Dict, resolved.Stream)
		if err != nil {
			continue
		}
		result = append(result, data...)
		result = append(result, ' ')
	}
	return result, nil
}

// PageFonts returns the font resource objects for a page.
func (doc *Document) PageFonts(page Dict) (map[string]*Object, error) {
	resourcesObj, ok := page["Resources"]
	if !ok {
		return nil, nil
	}
	resources, err := doc.Resolve(resourcesObj)
	if err != nil || resources == nil {
		return nil, err
	}
	var resDict Dict
	if resources.Type == ObjDict || resources.Type == ObjStream {
		resDict = resources.Dict
	}
	if resDict == nil {
		return nil, nil
	}

	fontDictObj, ok := resDict["Font"]
	if !ok {
		return nil, nil
	}
	fontDict, err := doc.Resolve(fontDictObj)
	if err != nil || fontDict == nil {
		return nil, err
	}
	if fontDict.Type != ObjDict {
		return nil, nil
	}

	fonts := make(map[string]*Object)
	for name, ref := range fontDict.Dict {
		obj, err := doc.Resolve(ref)
		if err == nil && obj != nil {
			fonts[name] = obj
		}
	}
	return fonts, nil
}

// PageInfo holds metadata about a single page.
type PageInfo struct {
	Width    float64
	Height   float64
	Rotation int
}

// GetPageInfo extracts dimensions and rotation for a page.
func (doc *Document) GetPageInfo(page Dict) PageInfo {
	info := PageInfo{}

	// /MediaBox gives the page dimensions
	if mbObj, ok := page["MediaBox"]; ok {
		mb, err := doc.Resolve(mbObj)
		if err == nil && mb.Type == ObjArray && len(mb.Array) >= 4 {
			x0 := floatFromObj(mb.Array[0])
			y0 := floatFromObj(mb.Array[1])
			x1 := floatFromObj(mb.Array[2])
			y1 := floatFromObj(mb.Array[3])
			info.Width = x1 - x0
			info.Height = y1 - y0
		}
	}

	if rotObj, ok := page["Rotate"]; ok {
		rot, err := doc.Resolve(rotObj)
		if err == nil && rot.Type == ObjInt {
			info.Rotation = int(rot.Int)
		}
	}
	return info
}

func floatFromObj(obj *Object) float64 {
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
