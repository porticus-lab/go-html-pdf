package htmlpdf

import "time"

// converterConfig holds internal configuration for a Converter.
type converterConfig struct {
	chromePath   string
	timeout      time.Duration
	noSandbox    bool
	headless     string
	autoDownload bool
}

func defaultConfig() converterConfig {
	return converterConfig{
		timeout:  30 * time.Second,
		headless: "new",
	}
}

// Option configures a [Converter].
type Option func(*converterConfig)

// WithChromePath sets the path to the Chrome or Chromium executable.
// By default the library searches standard locations automatically.
func WithChromePath(path string) Option {
	return func(c *converterConfig) {
		c.chromePath = path
	}
}

// WithTimeout sets the maximum duration for a single conversion.
// Defaults to 30 seconds. A zero or negative value disables the timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *converterConfig) {
		c.timeout = d
	}
}

// WithNoSandbox disables the Chrome sandbox. This is required when
// running as root, for example inside Docker containers.
func WithNoSandbox() Option {
	return func(c *converterConfig) {
		c.noSandbox = true
	}
}

// WithAutoDownload enables automatic download of a compatible Chromium
// binary when no browser is found in the system PATH. The binary is
// cached in ~/.cache/rod/browser (Unix) or %APPDATA%\rod\browser (Windows)
// and reused on subsequent calls. The first invocation may take 10–30 s
// depending on network speed; subsequent calls add only ~1 ms to check the
// cache.
//
// This option is ignored when [WithChromePath] is also set.
func WithAutoDownload() Option {
	return func(c *converterConfig) {
		c.autoDownload = true
	}
}
