package webcap

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeDiffRequestRejectsBadThreshold(t *testing.T) {
	_, err := NormalizeDiffRequest(DiffRequest{
		BasePath:    "base.png",
		ComparePath: "compare.png",
		Threshold:   1.5,
	})
	if err == nil {
		t.Fatal("expected threshold validation error")
	}
}

func TestServiceDiffSingleImage(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.png")
	comparePath := filepath.Join(dir, "compare.png")
	if err := writeTestPNG(basePath, []color.NRGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 255, B: 255, A: 255},
	}); err != nil {
		t.Fatalf("writeTestPNG base: %v", err)
	}
	if err := writeTestPNG(comparePath, []color.NRGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 0, B: 0, A: 255},
	}); err != nil {
		t.Fatalf("writeTestPNG compare: %v", err)
	}

	service := NewService(stubEngine{})
	result, err := service.Diff(context.Background(), DiffRequest{
		BasePath:    basePath,
		ComparePath: comparePath,
		OutputPath:  filepath.Join(dir, "diff.png"),
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if result.Mode != DiffModeImage {
		t.Fatalf("unexpected mode: %s", result.Mode)
	}
	if result.Entry == nil {
		t.Fatal("expected single diff entry")
	}
	if result.Entry.ChangedPixels != 1 {
		t.Fatalf("unexpected changed pixels: %d", result.Entry.ChangedPixels)
	}
	if !result.Entry.Changed {
		t.Fatal("expected changed=true")
	}
}

func TestServiceDiffThresholdSuppressesSmallChanges(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.png")
	comparePath := filepath.Join(dir, "compare.png")
	if err := writeTestPNG(basePath, []color.NRGBA{{R: 100, G: 100, B: 100, A: 255}}); err != nil {
		t.Fatalf("writeTestPNG base: %v", err)
	}
	if err := writeTestPNG(comparePath, []color.NRGBA{{R: 110, G: 100, B: 100, A: 255}}); err != nil {
		t.Fatalf("writeTestPNG compare: %v", err)
	}

	service := NewService(stubEngine{})
	result, err := service.Diff(context.Background(), DiffRequest{
		BasePath:    basePath,
		ComparePath: comparePath,
		OutputPath:  filepath.Join(dir, "diff.png"),
		Threshold:   0.1,
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if result.Entry == nil {
		t.Fatal("expected single diff entry")
	}
	if result.Entry.ChangedPixels != 0 {
		t.Fatalf("expected no changed pixels, got %d", result.Entry.ChangedPixels)
	}
}

func TestServiceDiffDirectories(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "base")
	compareDir := filepath.Join(dir, "compare")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir base: %v", err)
	}
	if err := os.MkdirAll(compareDir, 0o755); err != nil {
		t.Fatalf("mkdir compare: %v", err)
	}
	if err := writeTestPNG(filepath.Join(baseDir, "a.png"), []color.NRGBA{{R: 10, G: 10, B: 10, A: 255}}); err != nil {
		t.Fatalf("write base a: %v", err)
	}
	if err := writeTestPNG(filepath.Join(compareDir, "a.png"), []color.NRGBA{{R: 20, G: 20, B: 20, A: 255}}); err != nil {
		t.Fatalf("write compare a: %v", err)
	}
	if err := writeTestPNG(filepath.Join(baseDir, "missing.png"), []color.NRGBA{{R: 10, G: 10, B: 10, A: 255}}); err != nil {
		t.Fatalf("write base missing: %v", err)
	}

	service := NewService(stubEngine{})
	result, err := service.Diff(context.Background(), DiffRequest{
		BasePath:    baseDir,
		ComparePath: compareDir,
		OutputPath:  filepath.Join(dir, "diffs"),
	})
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if result.Mode != DiffModeDirectory {
		t.Fatalf("unexpected mode: %s", result.Mode)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("unexpected entry count: %d", len(result.Entries))
	}
	if result.Summary.ComparedFiles != 1 {
		t.Fatalf("unexpected compared files: %d", result.Summary.ComparedFiles)
	}
	if result.Summary.MissingCompareFiles != 1 {
		t.Fatalf("unexpected missing compare files: %d", result.Summary.MissingCompareFiles)
	}
	if result.Summary.ChangedFiles != 2 {
		t.Fatalf("unexpected changed files: %d", result.Summary.ChangedFiles)
	}
	if !result.Entries[1].Changed {
		t.Fatal("expected missing file entry to be marked changed")
	}
}

func writeTestPNG(path string, pixels []color.NRGBA) error {
	img := image.NewNRGBA(image.Rect(0, 0, len(pixels), 1))
	for x, pixel := range pixels {
		img.SetNRGBA(x, 0, pixel)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
