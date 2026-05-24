package webcap

import (
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareSemanticImagePayloadSupportsPNGAndJPEGExtensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")
	if err := writeTestPNG(path, []color.NRGBA{{R: 1, G: 2, B: 3, A: 255}}); err != nil {
		t.Fatalf("write png: %v", err)
	}
	payload, err := prepareSemanticImagePayload(path, "current", semanticImagePrepareOptions{MaxRawBytes: 1024})
	if err != nil {
		t.Fatalf("prepareSemanticImagePayload returned error: %v", err)
	}
	if payload.Role != "current" || payload.MIMEType != "image/png" || payload.Base64Data == "" || payload.ByteSize == 0 || payload.Width == 0 || payload.Height == 0 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestPrepareSemanticImagePayloadRejectsUnsupportedUnreadableAndLargeFiles(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "shot.txt")
	if err := os.WriteFile(txt, []byte("not an image"), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}
	if _, err := prepareSemanticImagePayload(txt, "current", semanticImagePrepareOptions{MaxRawBytes: 1024}); err == nil {
		t.Fatal("expected unsupported extension error")
	}
	if _, err := prepareSemanticImagePayload(filepath.Join(dir, "missing.png"), "current", semanticImagePrepareOptions{MaxRawBytes: 1024}); err == nil {
		t.Fatal("expected unreadable image error")
	}
	pngPath := filepath.Join(dir, "large.png")
	if err := writeTestPNG(pngPath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write png: %v", err)
	}
	if _, err := prepareSemanticImagePayload(pngPath, "current", semanticImagePrepareOptions{MaxRawBytes: 1}); err == nil {
		t.Fatal("expected max image byte error")
	}
}

func TestPrepareSemanticImagePayloadResizesProviderCopy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.png")
	pixels := make([]color.NRGBA, 80*40)
	for i := range pixels {
		pixels[i] = color.NRGBA{R: uint8(i), G: 20, B: 30, A: 255}
	}
	if err := writeTestPNG(path, pixels); err != nil {
		t.Fatalf("write png: %v", err)
	}
	tempDir := t.TempDir()
	payload, err := prepareSemanticImagePayload(path, "current", semanticImagePrepareOptions{
		MaxLongEdge:  20,
		MaxPixels:    400,
		ResizeImages: true,
		TempDir:      tempDir,
	})
	if err != nil {
		t.Fatalf("prepareSemanticImagePayload returned error: %v", err)
	}
	if payload.Path == path || payload.Width > 20 || payload.Height > 20 || payload.Metadata.ResizeReason == "" {
		t.Fatalf("expected resized provider copy, got %#v metadata=%#v", payload, payload.Metadata)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("original artifact was not preserved: %v", err)
	}
	if _, err := os.Stat(payload.Path); err != nil {
		t.Fatalf("expected temporary provider copy: %v", err)
	}
}

func TestValidateSemanticEncodedBudgets(t *testing.T) {
	err := validateSemanticEncodedBudgets([]SemanticImagePayload{
		{Base64Data: "12345"},
		{Base64Data: "12345"},
	}, 0, 9)
	var captureErr *Error
	if !errors.As(err, &captureErr) || captureErr.Code != CodeProviderPayloadTooLarge || captureErr.Metadata["limit_name"] != "max_combined_encoded_image_bytes" {
		t.Fatalf("expected encoded budget error, got %v %#v", err, captureErr)
	}
}
