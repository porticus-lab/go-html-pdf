package htmlpdf

import (
	"fmt"

	"github.com/go-rod/rod/lib/launcher"
)

// resolveBrowser downloads a compatible Chromium binary if one is not
// already cached and returns the path to the executable. The binary is
// stored in ~/.cache/rod/browser (Unix) or %APPDATA%\rod\browser (Windows).
func resolveBrowser() (string, error) {
	path, err := launcher.NewBrowser().Get()
	if err != nil {
		return "", fmt.Errorf("htmlpdf: downloading browser: %w", err)
	}
	return path, nil
}
