package htmlpdf

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

var samplePDF = []byte("%PDF-1.4 fake content for testing")

func newResult() *Result {
	return &Result{data: samplePDF}
}

func TestResult_Bytes(t *testing.T) {
	r := newResult()
	if !bytes.Equal(r.Bytes(), samplePDF) {
		t.Error("Bytes() did not return original data")
	}
}

func TestResult_Base64(t *testing.T) {
	r := newResult()
	got := r.Base64()
	want := base64.StdEncoding.EncodeToString(samplePDF)
	if got != want {
		t.Errorf("Base64() = %q, want %q", got, want)
	}
}

func TestResult_Reader(t *testing.T) {
	r := newResult()
	reader := r.Reader()
	if reader.Len() != len(samplePDF) {
		t.Errorf("Reader().Len() = %d, want %d", reader.Len(), len(samplePDF))
	}
	buf := make([]byte, len(samplePDF))
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Reader().Read: %v", err)
	}
	if !bytes.Equal(buf[:n], samplePDF) {
		t.Error("Reader() produced different content")
	}
}

func TestResult_WriteTo(t *testing.T) {
	r := newResult()
	var buf bytes.Buffer
	n, err := r.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if n != int64(len(samplePDF)) {
		t.Errorf("WriteTo wrote %d bytes, want %d", n, len(samplePDF))
	}
	if !bytes.Equal(buf.Bytes(), samplePDF) {
		t.Error("WriteTo produced different content")
	}
}

func TestResult_WriteToFile(t *testing.T) {
	r := newResult()
	path := filepath.Join(t.TempDir(), "test.pdf")
	if err := r.WriteToFile(path, 0o644); err != nil {
		t.Fatalf("WriteToFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if !bytes.Equal(data, samplePDF) {
		t.Error("WriteToFile produced different content")
	}
}

func TestResult_Len(t *testing.T) {
	r := newResult()
	if r.Len() != len(samplePDF) {
		t.Errorf("Len() = %d, want %d", r.Len(), len(samplePDF))
	}
}

func TestResult_ReaderMultipleCalls(t *testing.T) {
	r := newResult()
	r1 := r.Reader()
	r2 := r.Reader()
	if r1.Len() != r2.Len() {
		t.Error("multiple Reader() calls return different lengths")
	}
}
