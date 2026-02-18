package htmlpdf

import "errors"

// Sentinel errors returned by the library.
var (
	// ErrClosed is returned when attempting to use a closed [Converter].
	ErrClosed = errors.New("htmlpdf: converter is closed")
)
