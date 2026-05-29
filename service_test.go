package webcap

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"
)

type stubEngine struct {
	result EngineResult
	err    error
}

func (stubEngine) Name() string { return "stub" }

func (s stubEngine) Capture(ctx context.Context, req CaptureRequest) (EngineResult, error) {
	return s.result, s.err
}

func TestServiceCaptureWritesNormalizationAndMetadata(t *testing.T) {
	engine := stubEngine{
		result: EngineResult{
			Artifact: CaptureArtifact{
				Bytes:       []byte("png-bytes"),
				ImageFormat: "png",
				Mode:        CaptureModeFullPage,
				URL:         "http://localhost:3000",
				Viewport:    Viewport{Width: 1440, Height: 1200, ScaleFactor: 1},
			},
			Browser: BrowserInfo{
				Engine:   "stub",
				Headless: true,
			},
			Timing: CaptureTiming{
				NavigationStartedAt: time.Unix(10, 0).UTC(),
				ReadyAt:             time.Unix(11, 0).UTC(),
				CapturedAt:          time.Unix(12, 0).UTC(),
				TotalDuration:       "2s",
			},
		},
	}
	service := NewService(engine)

	dir := t.TempDir()
	result, err := service.Capture(context.Background(), CaptureRequest{
		URL:        "http://localhost:3000",
		FullPage:   true,
		OutputPath: dir + "/capture",
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if result.OutputPath != dir+"/capture.png" {
		t.Fatalf("unexpected output path: %s", result.OutputPath)
	}
	if result.Normalization.OutputBaseName != "capture" {
		t.Fatalf("unexpected normalization base name: %s", result.Normalization.OutputBaseName)
	}
	if result.Engine != "stub" {
		t.Fatalf("unexpected engine: %s", result.Engine)
	}
	if result.CapturedAt != time.Unix(12, 0).UTC() {
		t.Fatalf("unexpected captured at: %s", result.CapturedAt)
	}
}

func TestWriteFileUsesRestrictivePermissions(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/nested/capture.json"
	if err := writeFile(path, []byte("metadata")); err != nil {
		t.Fatalf("writeFile returned error: %v", err)
	}

	dirInfo, err := os.Stat(dir + "/nested")
	if err != nil {
		t.Fatalf("stat output dir: %v", err)
	}
	if mode := dirInfo.Mode().Perm(); mode&0o002 != 0 || mode&0o020 != 0 {
		t.Fatalf("output directory should not be writable by group/others: %o", mode)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat output file: %v", err)
	}
	if mode := fileInfo.Mode().Perm(); mode&0o077 != 0 {
		t.Fatalf("output file should be private to the owner: %o", mode)
	}
}

func TestServiceCaptureRedactsResolvedConfigMetadata(t *testing.T) {
	engine := stubEngine{
		result: EngineResult{
			Artifact: CaptureArtifact{
				Bytes:       []byte("png-bytes"),
				ImageFormat: "png",
				Mode:        CaptureModeFullPage,
				URL:         "http://localhost:3000/admin",
				Viewport:    Viewport{Width: 1440, Height: 1200, ScaleFactor: 1},
			},
			Browser: BrowserInfo{Engine: "stub", Headless: true},
			Timing: CaptureTiming{
				CapturedAt: time.Unix(12, 0).UTC(),
			},
			Guards: []GuardOutcome{{
				Kind:     "expect_url",
				Value:    "/admin",
				FinalURL: "http://localhost:3000/admin",
				Matched:  true,
				Status:   "passed",
			}},
		},
	}
	service := NewService(engine)
	dir := t.TempDir()
	result, err := service.Capture(context.Background(), CaptureRequest{
		URL:        "http://localhost:3000/admin",
		FullPage:   true,
		OutputPath: dir + "/capture.png",
		Auth: CaptureAuth{
			Headers: []CaptureHeader{{Name: "Authorization", Value: "Bearer raw-token"}},
			Cookies: []CaptureCookie{{Name: "sid", Value: "raw-cookie", Path: "/"}},
		},
		Guards: CaptureGuards{ExpectURL: "/admin"},
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if result.ResolvedConfig.Auth.Headers[0].Value != redactedSecretValue || result.ResolvedConfig.Auth.Cookies[0].Value != redactedSecretValue {
		t.Fatalf("resolved config was not redacted: %#v", result.ResolvedConfig.Auth)
	}
	payload, err := os.ReadFile(result.MetadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if bytes.Contains(payload, []byte("raw-token")) || bytes.Contains(payload, []byte("raw-cookie")) {
		t.Fatalf("metadata leaked auth secrets: %s", payload)
	}
	if len(result.Guards) != 1 || result.Guards[0].Kind != "expect_url" || result.Guards[0].Status != "passed" {
		t.Fatalf("guard outcomes were not copied to result: %#v", result.Guards)
	}
	var metadata CaptureResult
	if err := json.Unmarshal(payload, &metadata); err != nil {
		t.Fatalf("metadata JSON invalid: %v", err)
	}
	if len(metadata.Guards) != 1 || metadata.Guards[0].FinalURL != "http://localhost:3000/admin" {
		t.Fatalf("metadata missing guard outcomes: %#v", metadata.Guards)
	}
}

func TestServiceCapturePersistsTileArtifactsAndMetadata(t *testing.T) {
	engine := stubEngine{result: tiledEngineResult([]byte("tile-a"), []byte("tile-b"))}
	service := NewService(engine)
	dir := t.TempDir()
	result, err := service.Capture(context.Background(), CaptureRequest{
		URL:            "http://localhost:3000",
		FullPage:       true,
		OutputPath:     dir + "/capture.png",
		OversizePolicy: OversizePolicyTile,
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if result.Tiling == nil || result.Tiling.CompletedCount != 2 {
		t.Fatalf("unexpected tiling result: %+v", result.Tiling)
	}
	if _, statErr := os.Stat(dir + "/capture.tile-0000.png"); statErr != nil {
		t.Fatalf("expected first tile file: %v", statErr)
	}
	if _, statErr := os.Stat(dir + "/capture.tile-0001.png"); statErr != nil {
		t.Fatalf("expected second tile file: %v", statErr)
	}
	payload, readErr := os.ReadFile(result.MetadataPath)
	if readErr != nil {
		t.Fatalf("expected metadata: %v", readErr)
	}
	var metadata CaptureResult
	if err := json.Unmarshal(payload, &metadata); err != nil {
		t.Fatalf("metadata JSON invalid: %v", err)
	}
	if metadata.Tiling == nil || metadata.Tiling.Tiles[0].OutputPath == "" {
		t.Fatalf("metadata missing tile paths: %+v", metadata.Tiling)
	}
	if len(metadata.Tiling.Tiles[0].Bytes) != 0 {
		t.Fatalf("metadata must not embed tile bytes")
	}
}

func TestServiceCaptureReturnsPartialResultWithPersistedTiles(t *testing.T) {
	engineResult := tiledEngineResult([]byte("tile-a"), nil)
	engineResult.Tiling.Status = CaptureTilingPartial
	engineResult.Tiling.Tiles[1].Status = CaptureTileFailed
	engineResult.Tiling.Tiles[1].Error = "boom"
	engineResult.Tiling.CompletedCount = 1
	engineResult.Tiling.FailedCount = 1
	engine := stubEngine{
		result: engineResult,
		err: &PartialCaptureError{
			Operation:       "capture_tiles",
			FailedTileIndex: 1,
			CompletedCount:  1,
			TotalCount:      2,
		},
	}
	service := NewService(engine)
	dir := t.TempDir()
	result, err := service.Capture(context.Background(), CaptureRequest{
		URL:            "http://localhost:3000",
		FullPage:       true,
		OutputPath:     dir + "/capture.png",
		OversizePolicy: OversizePolicyTile,
	})
	if err == nil {
		t.Fatal("expected partial capture error")
	}
	partial, ok := err.(*PartialCaptureError)
	if !ok {
		t.Fatalf("expected partial error, got %T", err)
	}
	if partial.Result == nil || partial.Result.Tiling == nil || partial.Result.Tiling.CompletedCount != 1 {
		t.Fatalf("partial error missing persisted result: %+v", partial.Result)
	}
	if result.Tiling == nil || result.Tiling.Status != CaptureTilingPartial {
		t.Fatalf("unexpected returned result: %+v", result.Tiling)
	}
	if _, statErr := os.Stat(dir + "/capture.tile-0000.png"); statErr != nil {
		t.Fatalf("expected completed tile file: %v", statErr)
	}
	if _, statErr := os.Stat(dir + "/capture.tile-0001.png"); !os.IsNotExist(statErr) {
		t.Fatalf("failed tile should not be written, stat err=%v", statErr)
	}
}

func TestServiceCaptureBatchReturnsAccumulatedPartialResult(t *testing.T) {
	engineResult := tiledEngineResult([]byte("tile-a"), nil)
	engineResult.Tiling.Status = CaptureTilingPartial
	engineResult.Tiling.Tiles[1].Status = CaptureTileFailed
	engineResult.Tiling.CompletedCount = 1
	engineResult.Tiling.FailedCount = 1
	engine := stubEngine{
		result: engineResult,
		err: &PartialCaptureError{
			Operation:       "capture_tiles",
			FailedTileIndex: 1,
			CompletedCount:  1,
			TotalCount:      2,
		},
	}
	service := NewService(engine)
	dir := t.TempDir()
	batch, err := service.CaptureBatch(context.Background(), Manifest{
		OutputDir: dir,
		Shots: []ManifestShot{{
			URL:            "http://localhost:3000",
			Output:         "capture.png",
			FullPage:       true,
			OversizePolicy: OversizePolicyTile,
		}},
	}, "")
	if err == nil {
		t.Fatal("expected partial capture error")
	}
	if len(batch.Results) != 1 || batch.Results[0].Tiling == nil || batch.Results[0].Tiling.CompletedCount != 1 {
		t.Fatalf("batch should include partial result: %+v", batch)
	}
}

func TestServiceCaptureStitchesTiles(t *testing.T) {
	left := pngBytes(t, color.RGBA{R: 255, A: 255})
	right := pngBytes(t, color.RGBA{B: 255, A: 255})
	engine := stubEngine{result: tiledEngineResult(left, right)}
	service := NewService(engine)
	dir := t.TempDir()
	result, err := service.Capture(context.Background(), CaptureRequest{
		URL:            "http://localhost:3000",
		FullPage:       true,
		OutputPath:     dir + "/capture.png",
		OversizePolicy: OversizePolicyTile,
		Tile: CaptureTileOptions{
			Stitch: true,
		},
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if result.Tiling == nil || result.Tiling.StitchedPath != result.OutputPath {
		t.Fatalf("unexpected stitched result: %+v", result.Tiling)
	}
	if _, statErr := os.Stat(result.OutputPath); statErr != nil {
		t.Fatalf("expected stitched output: %v", statErr)
	}
}

func TestServiceCaptureStitchesOverlappingTilesWithoutDuplicateBand(t *testing.T) {
	left := pngImageBytes(t, []color.Color{color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}})
	right := pngImageBytes(t, []color.Color{color.RGBA{B: 255, A: 255}, color.RGBA{R: 255, G: 255, A: 255}})
	engineResult := tiledEngineResult(left, right)
	engineResult.Tiling.TargetBounds = Bounds{Width: 3, Height: 1}
	engineResult.Tiling.Tiles[0].SourceBounds = Bounds{X: 0, Y: 0, Width: 2, Height: 1}
	engineResult.Tiling.Tiles[0].DestinationBounds = &Bounds{X: 0, Y: 0, Width: 2, Height: 1}
	engineResult.Tiling.Tiles[1].SourceBounds = Bounds{X: 1, Y: 0, Width: 2, Height: 1}
	engineResult.Tiling.Tiles[1].DestinationBounds = &Bounds{X: 2, Y: 0, Width: 1, Height: 1}
	engine := stubEngine{result: engineResult}
	service := NewService(engine)
	dir := t.TempDir()
	result, err := service.Capture(context.Background(), CaptureRequest{
		URL:            "http://localhost:3000",
		FullPage:       true,
		OutputPath:     dir + "/capture.png",
		OversizePolicy: OversizePolicyTile,
		Tile:           CaptureTileOptions{Stitch: true},
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	payload, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("read stitched output: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("decode stitched output: %v", err)
	}
	if img.Bounds().Dx() != 3 {
		t.Fatalf("unexpected stitched width: %d", img.Bounds().Dx())
	}
	r, g, b, _ := img.At(2, 0).RGBA()
	if r == 0 || g == 0 || b != 0 {
		t.Fatalf("expected cropped pixel from second tile at destination edge")
	}
}

func TestServiceCaptureRejectsOversizedStitchedOutputBeforeAllocation(t *testing.T) {
	engine := stubEngine{result: tiledEngineResult(pngBytes(t, color.Black), pngBytes(t, color.White))}
	service := NewService(engine)
	_, err := service.Capture(context.Background(), CaptureRequest{
		URL:            "http://localhost:3000",
		FullPage:       true,
		OutputPath:     t.TempDir() + "/capture.png",
		OversizePolicy: OversizePolicyTile,
		Tile: CaptureTileOptions{
			Stitch:            true,
			MaxStitchedPixels: 1,
		},
	})
	if err == nil {
		t.Fatal("expected max stitched pixels error")
	}
}

func tiledEngineResult(first, second []byte) EngineResult {
	tiles := []CaptureTile{
		{
			Index:             0,
			Row:               0,
			Column:            0,
			SourceBounds:      Bounds{X: 0, Y: 0, Width: 1, Height: 1},
			DestinationBounds: &Bounds{X: 0, Y: 0, Width: 1, Height: 1},
			Status:            CaptureTileCompleted,
			Bytes:             first,
			ByteSize:          len(first),
		},
		{
			Index:             1,
			Row:               0,
			Column:            1,
			SourceBounds:      Bounds{X: 1, Y: 0, Width: 1, Height: 1},
			DestinationBounds: &Bounds{X: 1, Y: 0, Width: 1, Height: 1},
			Status:            CaptureTileCompleted,
			Bytes:             second,
			ByteSize:          len(second),
		},
	}
	return EngineResult{
		Artifact: CaptureArtifact{
			ImageFormat: "png",
			Mode:        CaptureModeFullPage,
			URL:         "http://localhost:3000",
			Viewport:    Viewport{Width: 1440, Height: 1200, ScaleFactor: 1},
		},
		Browser: BrowserInfo{Engine: "stub", Headless: true},
		Timing:  CaptureTiming{CapturedAt: time.Unix(12, 0).UTC()},
		Tiling: &CaptureTiling{
			Status:         CaptureTilingComplete,
			TargetBounds:   Bounds{Width: 2, Height: 1},
			Limits:         CaptureTileLimits{ScaleFactor: 1, MaxStitchedPixels: DefaultTileMaxStitchPixels},
			TileCount:      2,
			CompletedCount: 2,
			Tiles:          tiles,
		},
	}
}

func pngBytes(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, c)
	return encodeTestPNG(t, img)
}

func pngImageBytes(t *testing.T, pixels []color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, len(pixels), 1))
	for x, c := range pixels {
		img.Set(x, 0, c)
	}
	return encodeTestPNG(t, img)
}

func encodeTestPNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode failed: %v", err)
	}
	return buf.Bytes()
}
