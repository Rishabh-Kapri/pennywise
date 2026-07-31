package service

import (
	"bytes"
	"fmt"
	"image"
	// registered for image.DecodeConfig so page dimensions can be read
	_ "image/jpeg"
	_ "image/png"
	"math"

	"github.com/go-pdf/fpdf"
)

const (
	// A4 portrait, the page size document scanners default to.
	pdfPageWidthMM  = 210.0
	pdfPageHeightMM = 297.0

	// MaxScanPages bounds a single scan batch.
	MaxScanPages = 20
	// MaxScanTotalBytes bounds the whole multi-page upload.
	MaxScanTotalBytes int64 = 40 << 20 // 40 MiB
)

// ScanPage is one captured page of a scan batch.
type ScanPage struct {
	Name string
	Data []byte
}

// scanPageImageTypes maps sniffed formats to the fpdf image type. Scanners hand
// back JPEG; PNG is accepted for uploads that originate elsewhere.
var scanPageImageTypes = map[string]string{
	"jpeg": "JPG",
	"png":  "PNG",
}

// buildScannedPDF places each page image on its own A4 page, scaled to fit and
// centred, preserving aspect ratio — the layout a document scanner produces.
func buildScannedPDF(pages []ScanPage) ([]byte, error) {
	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages to assemble")
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)

	for i, page := range pages {
		cfg, format, err := image.DecodeConfig(bytes.NewReader(page.Data))
		if err != nil {
			return nil, fmt.Errorf("page %d is not a readable image: %w", i+1, err)
		}
		imageType, ok := scanPageImageTypes[format]
		if !ok {
			return nil, fmt.Errorf("page %d has unsupported image format %q", i+1, format)
		}

		w, h := fitWithin(float64(cfg.Width), float64(cfg.Height), pdfPageWidthMM, pdfPageHeightMM)

		opt := fpdf.ImageOptions{ImageType: imageType}
		name := fmt.Sprintf("page-%d", i)
		pdf.RegisterImageOptionsReader(name, opt, bytes.NewReader(page.Data))
		pdf.AddPage()
		pdf.ImageOptions(name, (pdfPageWidthMM-w)/2, (pdfPageHeightMM-h)/2, w, h, false, opt, 0, "")
	}

	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("error building pdf: %w", err)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("error writing pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// fitWithin scales src down to fit the bounds, keeping aspect ratio.
func fitWithin(srcW, srcH, maxW, maxH float64) (float64, float64) {
	if srcW <= 0 || srcH <= 0 {
		return maxW, maxH
	}
	scale := math.Min(maxW/srcW, maxH/srcH)
	return srcW * scale, srcH * scale
}
