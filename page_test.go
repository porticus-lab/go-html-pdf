package htmlpdf

import (
	"math"
	"testing"
)

func almostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestCmToInches(t *testing.T) {
	tests := []struct {
		cm   float64
		want float64
	}{
		{2.54, 1.0},
		{0, 0},
		{21.0, 8.2677},
		{29.7, 11.6929},
	}
	for _, tt := range tests {
		got := cmToInches(tt.cm)
		if !almostEqual(got, tt.want, 0.001) {
			t.Errorf("cmToInches(%v) = %v, want ~%v", tt.cm, got, tt.want)
		}
	}
}

func TestDefaultPageConfig(t *testing.T) {
	d := DefaultPageConfig()
	if d.Size != A4 {
		t.Errorf("default size = %v, want A4", d.Size)
	}
	if d.Orientation != Portrait {
		t.Errorf("default orientation = %v, want Portrait", d.Orientation)
	}
	if d.Scale != 1.0 {
		t.Errorf("default scale = %v, want 1.0", d.Scale)
	}
	if !d.PrintBackground {
		t.Error("default PrintBackground = false, want true")
	}
	if d.Margin != UniformMargin(1.0) {
		t.Errorf("default margin = %v, want uniform 1.0", d.Margin)
	}
}

func TestUniformMargin(t *testing.T) {
	m := UniformMargin(2.5)
	if m.Top != 2.5 || m.Right != 2.5 || m.Bottom != 2.5 || m.Left != 2.5 {
		t.Errorf("UniformMargin(2.5) = %+v, want all 2.5", m)
	}
}

func TestPageConfigResolved_Nil(t *testing.T) {
	var pc *PageConfig
	r := pc.resolved()
	d := DefaultPageConfig()
	if r != d {
		t.Errorf("nil resolved = %+v, want %+v", r, d)
	}
}

func TestPageConfigResolved_ZeroValues(t *testing.T) {
	pc := &PageConfig{}
	r := pc.resolved()
	if r.Size != A4 {
		t.Errorf("zero size resolved to %v, want A4", r.Size)
	}
	if r.Scale != 1.0 {
		t.Errorf("zero scale resolved to %v, want 1.0", r.Scale)
	}
	if r.Margin != UniformMargin(1.0) {
		t.Errorf("zero margin resolved to %v, want uniform 1.0", r.Margin)
	}
}

func TestPageConfigResolved_PreservesExplicit(t *testing.T) {
	pc := &PageConfig{
		Size:        Letter,
		Orientation: Landscape,
		Scale:       0.5,
		Margin:      Margin{Top: 2, Right: 3, Bottom: 2, Left: 3},
	}
	r := pc.resolved()
	if r.Size != Letter {
		t.Errorf("size = %v, want Letter", r.Size)
	}
	if r.Orientation != Landscape {
		t.Errorf("orientation = %v, want Landscape", r.Orientation)
	}
	if r.Scale != 0.5 {
		t.Errorf("scale = %v, want 0.5", r.Scale)
	}
	if r.Margin.Top != 2 {
		t.Errorf("margin top = %v, want 2", r.Margin.Top)
	}
}

func TestPaperDimensions_Portrait(t *testing.T) {
	pc := &PageConfig{Size: A4, Orientation: Portrait}
	w, h := pc.paperDimensions()
	// A4 = 21.0 x 29.7 cm = 8.267 x 11.693 inches
	if !almostEqual(w, 8.267, 0.01) {
		t.Errorf("portrait width = %v, want ~8.267", w)
	}
	if !almostEqual(h, 11.693, 0.01) {
		t.Errorf("portrait height = %v, want ~11.693", h)
	}
}

func TestPaperDimensions_Landscape(t *testing.T) {
	pc := &PageConfig{Size: A4, Orientation: Landscape, Scale: 1.0, Margin: UniformMargin(1.0)}
	w, h := pc.paperDimensions()
	// Landscape swaps width and height.
	if !almostEqual(w, 11.693, 0.01) {
		t.Errorf("landscape width = %v, want ~11.693", w)
	}
	if !almostEqual(h, 8.267, 0.01) {
		t.Errorf("landscape height = %v, want ~8.267", h)
	}
}

func TestMarginInches(t *testing.T) {
	pc := &PageConfig{
		Size:   A4,
		Scale:  1.0,
		Margin: Margin{Top: 2.54, Right: 5.08, Bottom: 2.54, Left: 5.08},
	}
	top, right, bottom, left := pc.marginInches()
	if !almostEqual(top, 1.0, 0.001) {
		t.Errorf("top = %v, want 1.0", top)
	}
	if !almostEqual(right, 2.0, 0.001) {
		t.Errorf("right = %v, want 2.0", right)
	}
	if !almostEqual(bottom, 1.0, 0.001) {
		t.Errorf("bottom = %v, want 1.0", bottom)
	}
	if !almostEqual(left, 2.0, 0.001) {
		t.Errorf("left = %v, want 2.0", left)
	}
}
