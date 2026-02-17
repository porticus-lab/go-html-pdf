package htmlpdf

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Converter converts HTML content to PDF documents.
//
// A Converter manages a headless browser instance that is reused across
// multiple conversions for performance. It is safe for concurrent use.
//
// Call [Converter.Close] when the Converter is no longer needed to release
// browser resources.
type Converter struct {
	cfg           converterConfig
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc

	mu     sync.Mutex
	closed bool
}

// NewConverter creates a Converter with the given options.
//
// It starts a headless browser in the background. The caller must call
// [Converter.Close] when finished.
func NewConverter(opts ...Option) (*Converter, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	allocOpts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("headless", cfg.headless),
	)
	if cfg.chromePath != "" {
		allocOpts = append(allocOpts, chromedp.ExecPath(cfg.chromePath))
	}
	if cfg.noSandbox {
		allocOpts = append(allocOpts, chromedp.Flag("no-sandbox", true))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	// Start the browser eagerly so errors surface at creation time.
	if err := chromedp.Run(browserCtx); err != nil {
		browserCancel()
		allocCancel()
		return nil, fmt.Errorf("htmlpdf: starting browser: %w", err)
	}

	return &Converter{
		cfg:           cfg,
		allocCtx:      allocCtx,
		allocCancel:   allocCancel,
		browserCtx:    browserCtx,
		browserCancel: browserCancel,
	}, nil
}

// Close releases all resources held by the Converter, including the
// browser process. Close is idempotent.
func (c *Converter) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	c.browserCancel()
	c.allocCancel()
	return nil
}

// ConvertHTML converts an HTML string to a PDF document.
// If page is nil, [DefaultPageConfig] values are used.
func (c *Converter) ConvertHTML(ctx context.Context, html string, pg *PageConfig) (*Result, error) {
	if err := c.checkClosed(); err != nil {
		return nil, err
	}

	f, err := os.CreateTemp("", "htmlpdf-*.html")
	if err != nil {
		return nil, fmt.Errorf("htmlpdf: creating temp file: %w", err)
	}
	name := f.Name()
	defer os.Remove(name)

	if _, err := f.WriteString(html); err != nil {
		f.Close()
		return nil, fmt.Errorf("htmlpdf: writing temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("htmlpdf: closing temp file: %w", err)
	}

	abs, err := filepath.Abs(name)
	if err != nil {
		return nil, fmt.Errorf("htmlpdf: resolving path: %w", err)
	}
	return c.convert(ctx, "file://"+abs, pg)
}

// ConvertURL converts the web page at rawURL to a PDF document.
// If page is nil, [DefaultPageConfig] values are used.
func (c *Converter) ConvertURL(ctx context.Context, rawURL string, pg *PageConfig) (*Result, error) {
	if err := c.checkClosed(); err != nil {
		return nil, err
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return nil, fmt.Errorf("htmlpdf: invalid URL %q: %w", rawURL, err)
	}
	return c.convert(ctx, rawURL, pg)
}

// ConvertFile converts a local HTML file to a PDF document.
// If page is nil, [DefaultPageConfig] values are used.
func (c *Converter) ConvertFile(ctx context.Context, path string, pg *PageConfig) (*Result, error) {
	if err := c.checkClosed(); err != nil {
		return nil, err
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("htmlpdf: resolving path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("htmlpdf: %w", err)
	}
	return c.convert(ctx, "file://"+abs, pg)
}

// convert performs the actual navigation and PDF generation.
func (c *Converter) convert(ctx context.Context, targetURL string, pg *PageConfig) (*Result, error) {
	resolved := pg.resolved()

	if c.cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.timeout)
		defer cancel()
	}

	tabCtx, tabCancel := chromedp.NewContext(c.browserCtx)
	defer tabCancel()

	width, height := resolved.paperDimensions()
	marginTop, marginRight, marginBottom, marginLeft := resolved.marginInches()

	var buf []byte
	if err := chromedp.Run(tabCtx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			params := page.PrintToPDF().
				WithPaperWidth(width).
				WithPaperHeight(height).
				WithMarginTop(marginTop).
				WithMarginRight(marginRight).
				WithMarginBottom(marginBottom).
				WithMarginLeft(marginLeft).
				WithScale(resolved.Scale).
				WithPrintBackground(resolved.PrintBackground).
				WithLandscape(resolved.Orientation == Landscape).
				WithPreferCSSPageSize(resolved.PreferCSSPageSize).
				WithDisplayHeaderFooter(resolved.DisplayHeaderFooter)

			if resolved.HeaderTemplate != "" {
				params = params.WithHeaderTemplate(resolved.HeaderTemplate)
			}
			if resolved.FooterTemplate != "" {
				params = params.WithFooterTemplate(resolved.FooterTemplate)
			}

			var err error
			buf, _, err = params.Do(ctx)
			return err
		}),
	); err != nil {
		return nil, fmt.Errorf("htmlpdf: conversion failed: %w", err)
	}

	return &Result{data: buf}, nil
}

func (c *Converter) checkClosed() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClosed
	}
	return nil
}

// --- Package-level convenience functions ---

// ConvertHTML converts an HTML string to PDF using a temporary [Converter].
// This is convenient for one-off conversions. For repeated use, create a
// [Converter] with [NewConverter] to reuse the browser instance.
func ConvertHTML(ctx context.Context, html string, pg *PageConfig, opts ...Option) (*Result, error) {
	conv, err := NewConverter(opts...)
	if err != nil {
		return nil, err
	}
	defer conv.Close()
	return conv.ConvertHTML(ctx, html, pg)
}

// ConvertURL converts a web page to PDF using a temporary [Converter].
func ConvertURL(ctx context.Context, rawURL string, pg *PageConfig, opts ...Option) (*Result, error) {
	conv, err := NewConverter(opts...)
	if err != nil {
		return nil, err
	}
	defer conv.Close()
	return conv.ConvertURL(ctx, rawURL, pg)
}

// ConvertFile converts a local HTML file to PDF using a temporary [Converter].
func ConvertFile(ctx context.Context, path string, pg *PageConfig, opts ...Option) (*Result, error) {
	conv, err := NewConverter(opts...)
	if err != nil {
		return nil, err
	}
	defer conv.Close()
	return conv.ConvertFile(ctx, path, pg)
}
