package webcap

import (
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
	payload, err := prepareSemanticImagePayload(path, "current", 1024)
	if err != nil {
		t.Fatalf("prepareSemanticImagePayload returned error: %v", err)
	}
	if payload.Role != "current" || payload.MIMEType != "image/png" || payload.Base64Data == "" || payload.ByteSize == 0 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestPrepareSemanticImagePayloadRejectsUnsupportedUnreadableAndLargeFiles(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "shot.txt")
	if err := os.WriteFile(txt, []byte("not an image"), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}
	if _, err := prepareSemanticImagePayload(txt, "current", 1024); err == nil {
		t.Fatal("expected unsupported extension error")
	}
	if _, err := prepareSemanticImagePayload(filepath.Join(dir, "missing.png"), "current", 1024); err == nil {
		t.Fatal("expected unreadable image error")
	}
	pngPath := filepath.Join(dir, "large.png")
	if err := writeTestPNG(pngPath, []color.NRGBA{{A: 255}}); err != nil {
		t.Fatalf("write png: %v", err)
	}
	if _, err := prepareSemanticImagePayload(pngPath, "current", 1); err == nil {
		t.Fatal("expected max image byte error")
	}
}
