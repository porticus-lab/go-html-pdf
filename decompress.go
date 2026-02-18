package htmlpdf

import (
	"bytes"
	"compress/lzw"
	"compress/zlib"
	"encoding/ascii85"
	"fmt"
	"io"
)

// maxDecompressedSize prevents DoS via unbounded memory allocation (256 MB).
const maxDecompressedSize = 256 * 1024 * 1024

// DecompressStream decompresses a PDF stream given its dictionary and raw bytes.
// It handles filter chains (multiple filters applied in sequence).
func DecompressStream(dict Dict, data []byte) ([]byte, error) {
	filterObj, ok := dict["Filter"]
	if !ok {
		// No filter, return as-is
		return data, nil
	}

	// Filters can be a single name or an array of names
	var filters []string
	var params []Dict

	switch filterObj.Type {
	case ObjName:
		filters = []string{filterObj.Name}
		if pObj, ok := dict["DecodeParms"]; ok && pObj.Type == ObjDict {
			params = []Dict{pObj.Dict}
		} else {
			params = []Dict{nil}
		}
	case ObjArray:
		for _, f := range filterObj.Array {
			if f.Type == ObjName {
				filters = append(filters, f.Name)
			}
		}
		if pArr, ok := dict["DecodeParms"]; ok && pArr.Type == ObjArray {
			for _, p := range pArr.Array {
				if p != nil && p.Type == ObjDict {
					params = append(params, p.Dict)
				} else {
					params = append(params, nil)
				}
			}
		}
		for len(params) < len(filters) {
			params = append(params, nil)
		}
	default:
		return data, nil
	}

	current := data
	for i, filter := range filters {
		var parms Dict
		if i < len(params) {
			parms = params[i]
		}
		var err error
		current, err = applyFilter(filter, parms, current)
		if err != nil {
			return nil, fmt.Errorf("applying filter %s: %w", filter, err)
		}
	}
	return current, nil
}

// applyFilter applies a single named PDF filter to data.
func applyFilter(filter string, parms Dict, data []byte) ([]byte, error) {
	switch filter {
	case "FlateDecode", "Fl":
		return flateDecode(parms, data)
	case "ASCII85Decode", "A85":
		return ascii85Decode(data)
	case "ASCIIHexDecode", "AHx":
		return asciiHexDecode(data)
	case "LZWDecode", "LZW":
		return lzwDecode(parms, data)
	case "RunLengthDecode", "RL":
		return runLengthDecode(data)
	case "DCTDecode", "DCT",
		"CCITTFaxDecode", "CCF",
		"JBIG2Decode",
		"JPXDecode":
		// Image formats: pass through as-is (binary data)
		return data, nil
	case "Crypt":
		// Identity crypt: pass through
		return data, nil
	default:
		return data, fmt.Errorf("unsupported filter: %s", filter)
	}
}

// flateDecode decompresses zlib/deflate data with optional PNG/TIFF predictor.
func flateDecode(parms Dict, data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("zlib: %w", err)
	}
	defer r.Close()

	result, err := io.ReadAll(io.LimitReader(r, maxDecompressedSize+1))
	if err != nil {
		return nil, fmt.Errorf("zlib read: %w", err)
	}
	if len(result) > maxDecompressedSize {
		return nil, fmt.Errorf("decompressed size exceeds 256 MB limit")
	}

	if parms == nil {
		return result, nil
	}

	// Apply predictor if present
	predictor, hasPredictor := parms.GetInt("Predictor")
	if !hasPredictor || predictor == 1 {
		return result, nil
	}

	if predictor == 2 {
		// TIFF predictor
		return applyTIFFPredictor(parms, result)
	}
	if predictor >= 10 && predictor <= 15 {
		// PNG predictors
		return applyPNGPredictor(parms, result)
	}
	return result, nil
}

// applyTIFFPredictor undoes TIFF predictor encoding.
func applyTIFFPredictor(parms Dict, data []byte) ([]byte, error) {
	colors, _ := parms.GetInt("Colors")
	bitsPerComponent, _ := parms.GetInt("BitsPerComponent")
	columns, _ := parms.GetInt("Columns")
	if colors == 0 {
		colors = 1
	}
	if bitsPerComponent == 0 {
		bitsPerComponent = 8
	}
	if columns == 0 {
		columns = 1
	}
	rowBytes := int((int64(columns)*colors*bitsPerComponent + 7) / 8)
	if rowBytes == 0 {
		return data, nil
	}

	result := make([]byte, len(data))
	for row := 0; row*rowBytes < len(data); row++ {
		start := row * rowBytes
		end := start + rowBytes
		if end > len(data) {
			end = len(data)
		}
		copy(result[start:end], data[start:end])
		for i := start + 1; i < end; i++ {
			result[i] += result[i-1]
		}
	}
	return result, nil
}

// applyPNGPredictor undoes PNG filter encoding (filters 10-15).
func applyPNGPredictor(parms Dict, data []byte) ([]byte, error) {
	colors, _ := parms.GetInt("Colors")
	bitsPerComponent, _ := parms.GetInt("BitsPerComponent")
	columns, _ := parms.GetInt("Columns")
	if colors == 0 {
		colors = 1
	}
	if bitsPerComponent == 0 {
		bitsPerComponent = 8
	}
	if columns == 0 {
		columns = 1
	}
	rowBytes := int((int64(columns)*colors*bitsPerComponent + 7) / 8)
	stride := rowBytes + 1 // +1 for the filter byte

	if len(data) == 0 || stride <= 1 {
		return data, nil
	}

	numRows := len(data) / stride
	result := make([]byte, numRows*rowBytes)
	prev := make([]byte, rowBytes)

	for row := 0; row < numRows; row++ {
		srcRow := data[row*stride : row*stride+stride]
		filterType := srcRow[0]
		srcData := srcRow[1:]
		dstRow := result[row*rowBytes : row*rowBytes+rowBytes]

		switch filterType {
		case 0: // None
			copy(dstRow, srcData)
		case 1: // Sub
			for i := range dstRow {
				a := byte(0)
				if i > 0 {
					a = dstRow[i-1]
				}
				dstRow[i] = srcData[i] + a
			}
		case 2: // Up
			for i := range dstRow {
				dstRow[i] = srcData[i] + prev[i]
			}
		case 3: // Average
			for i := range dstRow {
				a := byte(0)
				if i > 0 {
					a = dstRow[i-1]
				}
				b := prev[i]
				dstRow[i] = srcData[i] + byte((int(a)+int(b))/2)
			}
		case 4: // Paeth
			for i := range dstRow {
				a := byte(0)
				c := byte(0)
				if i > 0 {
					a = dstRow[i-1]
					c = prev[i-1]
				}
				b := prev[i]
				dstRow[i] = srcData[i] + paethPredictor(a, b, c)
			}
		default:
			copy(dstRow, srcData)
		}
		copy(prev, dstRow)
	}
	return result, nil
}

func paethPredictor(a, b, c byte) byte {
	ia, ib, ic := int(a), int(b), int(c)
	p := ia + ib - ic
	pa := abs(p - ia)
	pb := abs(p - ib)
	pc := abs(p - ic)
	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ascii85Decode decodes ASCII85 encoded data.
func ascii85Decode(data []byte) ([]byte, error) {
	// Strip whitespace and find the end-of-data marker ~>
	end := bytes.Index(data, []byte("~>"))
	if end >= 0 {
		data = data[:end+2]
	}
	decoder := ascii85.NewDecoder(bytes.NewReader(data))
	result, err := io.ReadAll(io.LimitReader(decoder, maxDecompressedSize+1))
	if err != nil {
		return nil, fmt.Errorf("ascii85: %w", err)
	}
	return result, nil
}

// asciiHexDecode decodes ASCIIHex encoded data (pairs of hex digits).
func asciiHexDecode(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	i := 0
	for i < len(data) {
		// Skip whitespace
		for i < len(data) && isWhitespace(data[i]) {
			i++
		}
		if i >= len(data) || data[i] == '>' {
			break
		}
		hi := hexVal(data[i])
		i++
		lo := byte(0)
		// Skip whitespace between nibbles
		for i < len(data) && isWhitespace(data[i]) {
			i++
		}
		if i < len(data) && data[i] != '>' {
			lo = hexVal(data[i])
			i++
		}
		buf.WriteByte(hi<<4 | lo)
	}
	return buf.Bytes(), nil
}

// lzwDecode decompresses LZW-encoded data.
// PDF uses MSB-first LZW with early change (EarlyChange = 1 by default).
func lzwDecode(parms Dict, data []byte) ([]byte, error) {
	earlyChange := int64(1)
	if parms != nil {
		if ec, ok := parms.GetInt("EarlyChange"); ok {
			earlyChange = ec
		}
	}
	// Go's compress/lzw supports MSB-first (TIFF order)
	// PDF LZW uses MSB-first with litWidth=8
	order := lzw.MSB
	_ = earlyChange // Go's LZW handles early change internally for TIFF order
	r := lzw.NewReader(bytes.NewReader(data), order, 8)
	defer r.Close()
	result, err := io.ReadAll(io.LimitReader(r, maxDecompressedSize+1))
	if err != nil {
		return nil, fmt.Errorf("lzw: %w", err)
	}
	return result, nil
}

// runLengthDecode decompresses PackBits/RunLength encoded data.
// Each run is: length byte followed by data bytes.
// - length 0-127: copy the next (length+1) bytes literally
// - length 129-255: repeat the next byte (257-length) times
// - length 128: end of data marker
func runLengthDecode(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	i := 0
	for i < len(data) {
		length := int(data[i])
		i++
		if length == 128 {
			break // EOD
		} else if length < 128 {
			// Literal run: copy next (length+1) bytes
			count := length + 1
			if i+count > len(data) {
				count = len(data) - i
			}
			buf.Write(data[i : i+count])
			i += count
		} else {
			// Repeated run: repeat next byte (257-length) times
			count := 257 - length
			if i >= len(data) {
				break
			}
			b := data[i]
			i++
			for j := 0; j < count; j++ {
				buf.WriteByte(b)
			}
		}
		if buf.Len() > maxDecompressedSize {
			return nil, fmt.Errorf("decompressed size exceeds 256 MB limit")
		}
	}
	return buf.Bytes(), nil
}
