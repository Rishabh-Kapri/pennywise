package service

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func jpegPage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 220, G: 220, B: 210, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encoding jpeg: %v", err)
	}
	return buf.Bytes()
}

func pngPage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding png: %v", err)
	}
	return buf.Bytes()
}

func TestBuildScannedPDFProducesOnePagePerImage(t *testing.T) {
	pages := []ScanPage{
		{Name: "a.jpg", Data: jpegPage(t, 1200, 1600)},
		{Name: "b.jpg", Data: jpegPage(t, 1600, 1200)}, // landscape
		{Name: "c.png", Data: pngPage(t, 800, 1000)},
	}

	out, err := buildScannedPDF(pages)
	if err != nil {
		t.Fatalf("buildScannedPDF: %v", err)
	}

	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF, got prefix %q", out[:min(8, len(out))])
	}
	if got := strings.Count(string(out), "/Type /Page\n"); got != len(pages) {
		t.Errorf("page count = %d, want %d", got, len(pages))
	}
	if len(out) < 1000 {
		t.Errorf("pdf suspiciously small: %d bytes", len(out))
	}
}

func TestBuildScannedPDFFitsWithinPageBounds(t *testing.T) {
	// a very wide image must be scaled to fit A4 width, not overflow it
	w, h := fitWithin(4000, 1000, pdfPageWidthMM, pdfPageHeightMM)
	if w > pdfPageWidthMM+0.01 || h > pdfPageHeightMM+0.01 {
		t.Errorf("fitWithin overflowed page: got %vx%v", w, h)
	}
	if ratio := w / h; ratio < 3.99 || ratio > 4.01 {
		t.Errorf("aspect ratio not preserved: %v", ratio)
	}

	// a tall image is bounded by height
	w2, h2 := fitWithin(1000, 4000, pdfPageWidthMM, pdfPageHeightMM)
	if h2 > pdfPageHeightMM+0.01 || w2 > pdfPageWidthMM+0.01 {
		t.Errorf("fitWithin overflowed page: got %vx%v", w2, h2)
	}
}

func TestBuildScannedPDFRejectsBadInput(t *testing.T) {
	if _, err := buildScannedPDF(nil); err == nil {
		t.Error("expected an error for zero pages")
	}
	if _, err := buildScannedPDF([]ScanPage{{Name: "x.txt", Data: []byte("not an image")}}); err == nil {
		t.Error("expected an error for a non-image page")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
