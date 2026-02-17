package htmlpdf

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
)

// Result holds a generated PDF and provides helpers for common output
// formats such as raw bytes, base64 encoding, and streaming readers.
//
// A Result is returned by every conversion method. It is safe to call
// its methods multiple times — the underlying data is never modified.
type Result struct {
	data []byte
}

// Bytes returns the raw PDF content.
func (r *Result) Bytes() []byte {
	return r.data
}

// Base64 returns the PDF encoded as a standard base64 string (RFC 4648).
// This is useful for embedding in JSON payloads or uploading to services
// that accept base64-encoded content.
func (r *Result) Base64() string {
	return base64.StdEncoding.EncodeToString(r.data)
}

// Reader returns an [*bytes.Reader] over the PDF content.
// This is suitable for streaming uploads to cloud storage (GCP, AWS S3, etc.)
// or any API that accepts an [io.Reader].
func (r *Result) Reader() *bytes.Reader {
	return bytes.NewReader(r.data)
}

// WriteTo writes the full PDF content to w. It implements [io.WriterTo].
func (r *Result) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(r.data)
	return int64(n), err
}

// WriteToFile writes the PDF to the file at path, creating it if needed.
func (r *Result) WriteToFile(path string, perm os.FileMode) error {
	return os.WriteFile(path, r.data, perm)
}

// Len returns the size of the PDF in bytes.
func (r *Result) Len() int {
	return len(r.data)
}
