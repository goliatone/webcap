package webcap

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalizeCaptureRequestDefaults(t *testing.T) {
	req, err := NormalizeCaptureRequest(CaptureRequest{
		URL:        "http://localhost:3000",
		OutputPath: "shots/home",
	})
	if err != nil {
		t.Fatalf("NormalizeCaptureRequest returned error: %v", err)
	}
	if req.Viewport.Width != defaultViewportWidth || req.Viewport.Height != defaultViewportHeight {
		t.Fatalf("unexpected viewport: %+v", req.Viewport)
	}
	if req.Viewport.ScaleFactor != defaultScaleFactor {
		t.Fatalf("unexpected scale factor: %v", req.Viewport.ScaleFactor)
	}
	if req.OutputPath != "shots/home.png" {
		t.Fatalf("unexpected output path: %s", req.OutputPath)
	}
	if req.MetadataPath != "shots/home.png.json" {
		t.Fatalf("unexpected metadata path: %s", req.MetadataPath)
	}
	if req.Readiness != defaultReadinessMode {
		t.Fatalf("unexpected readiness: %s", req.Readiness)
	}
	if effectiveOversizePolicy(req) != OversizePolicyFail {
		t.Fatalf("unexpected oversize policy: %s", effectiveOversizePolicy(req))
	}
	if req.OversizePolicy != "" {
		t.Fatalf("default oversize policy should not be serialized into the request: %s", req.OversizePolicy)
	}
	if effectiveTileOptions(req.Tile).MaxWidth != DefaultTileMaxWidth {
		t.Fatalf("unexpected tile default: %+v", effectiveTileOptions(req.Tile))
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if string(encoded) == "" || jsonContains(encoded, "tile") || jsonContains(encoded, "oversize_policy") {
		t.Fatalf("normal request JSON should omit tile defaults: %s", encoded)
	}
}

func TestNormalizeCaptureRequestRejectsInvalidTileOptions(t *testing.T) {
	tests := []CaptureRequest{
		{URL: "http://localhost:3000", OversizePolicy: "crop"},
		{URL: "http://localhost:3000", Tile: CaptureTileOptions{MaxWidth: -1}},
		{URL: "http://localhost:3000", Tile: CaptureTileOptions{MaxWidth: 100, Overlap: 100}},
	}
	for _, req := range tests {
		if _, err := NormalizeCaptureRequest(req); err == nil {
			t.Fatalf("expected validation error for %+v", req)
		}
	}
}

func TestPartialCaptureErrorCarriesPersistedResult(t *testing.T) {
	partial := (&PartialCaptureError{
		Operation:       "capture_tiles",
		FailedTileIndex: 1,
		CompletedCount:  1,
		TotalCount:      2,
		Err:             errors.New("boom"),
	}).WithResult(CaptureResult{OutputPath: "out.png"})
	if partial.Result == nil || partial.Result.OutputPath != "out.png" {
		t.Fatalf("missing result: %#v", partial.Result)
	}
	if !errors.Is(partial, partial.Err) {
		t.Fatalf("partial error should unwrap underlying error")
	}
}

func jsonContains(payload []byte, key string) bool {
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return false
	}
	_, ok := decoded[key]
	return ok
}

func TestNormalizeCaptureRequestRejectsConflictingTargetModes(t *testing.T) {
	_, err := NormalizeCaptureRequest(CaptureRequest{
		URL:      "http://localhost:3000",
		FullPage: true,
		Selector: ".hero",
	})
	if err == nil {
		t.Fatal("expected conflicting target error")
	}
}

func TestCaptureRequestTargetSelectors(t *testing.T) {
	req, err := NormalizeCaptureRequest(CaptureRequest{
		URL:       "http://localhost:3000",
		Selectors: []string{" .hero ", ".cta"},
	})
	if err != nil {
		t.Fatalf("NormalizeCaptureRequest returned error: %v", err)
	}
	selectors, useAll := req.TargetSelectors()
	if useAll {
		t.Fatal("expected useAll=false")
	}
	if len(selectors) != 2 || selectors[0] != ".hero" || selectors[1] != ".cta" {
		t.Fatalf("unexpected selectors: %#v", selectors)
	}
}

func TestNormalizeCaptureRequestAppliesViewportPreset(t *testing.T) {
	req, err := NormalizeCaptureRequest(CaptureRequest{
		URL:            "http://localhost:3000",
		ViewportPreset: "desktop-xl",
	})
	if err != nil {
		t.Fatalf("NormalizeCaptureRequest returned error: %v", err)
	}
	if req.Viewport.Width != 1440 || req.Viewport.Height != 1200 {
		t.Fatalf("unexpected preset viewport: %+v", req.Viewport)
	}
}

func TestNormalizeCaptureRequestRejectsPresetAndExplicitViewport(t *testing.T) {
	_, err := NormalizeCaptureRequest(CaptureRequest{
		URL:            "http://localhost:3000",
		ViewportPreset: "desktop-xl",
		Viewport: Viewport{
			Width: 1024,
		},
	})
	if err == nil {
		t.Fatal("expected viewport preset conflict error")
	}
}

func TestResolvePersistedPathsGeneratesDeterministicName(t *testing.T) {
	req, err := NormalizeCaptureRequest(CaptureRequest{
		URL:      "http://localhost:3000",
		FullPage: true,
	})
	if err != nil {
		t.Fatalf("NormalizeCaptureRequest returned error: %v", err)
	}
	resolved, baseName, generated, err := ResolvePersistedPaths(req, "shots")
	if err != nil {
		t.Fatalf("ResolvePersistedPaths returned error: %v", err)
	}
	if !generated {
		t.Fatal("expected generated output path")
	}
	if baseName != "localhost-home-full-page" {
		t.Fatalf("unexpected base name: %s", baseName)
	}
	if resolved.OutputPath != "shots/localhost-home-full-page.png" {
		t.Fatalf("unexpected output path: %s", resolved.OutputPath)
	}
	if resolved.MetadataPath != "shots/localhost-home-full-page.png.json" {
		t.Fatalf("unexpected metadata path: %s", resolved.MetadataPath)
	}
}
