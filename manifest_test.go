package webcap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRequestsApplyDefaults(t *testing.T) {
	manifest := Manifest{
		OutputDir: "shots",
		Viewport: &Viewport{
			Width:  1600,
			Height: 900,
		},
		Wait: "250ms",
		Shots: []ManifestShot{
			{
				ID:       "home",
				URL:      "http://localhost:3000",
				FullPage: true,
			},
		},
	}

	requests, err := manifest.Requests("")
	if err != nil {
		t.Fatalf("Requests returned error: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("unexpected request count: %d", len(requests))
	}
	req := requests[0]
	if req.OutputPath != "shots/localhost-home-full-page.png" {
		t.Fatalf("unexpected output path: %s", req.OutputPath)
	}
	if req.Wait != "250ms" {
		t.Fatalf("unexpected wait: %s", req.Wait)
	}
	if req.Viewport.Width != 1600 || req.Viewport.Height != 900 {
		t.Fatalf("unexpected viewport: %+v", req.Viewport)
	}
	if req.Readiness != defaultReadinessMode {
		t.Fatalf("unexpected readiness: %s", req.Readiness)
	}
}

func TestManifestRequestsApplyTileDefaultsAndShotOverrides(t *testing.T) {
	manifest := Manifest{
		OutputDir:      "shots",
		OversizePolicy: OversizePolicyTile,
		Tile: CaptureTileOptions{
			MaxWidth:  4000,
			MaxHeight: 3000,
			Overlap:   10,
		},
		Shots: []ManifestShot{
			{
				ID:  "home",
				URL: "http://localhost:3000",
				Tile: CaptureTileOptions{
					MaxHeight: 2000,
				},
			},
		},
	}

	requests, err := manifest.Requests("")
	if err != nil {
		t.Fatalf("Requests returned error: %v", err)
	}
	req := requests[0]
	if req.OversizePolicy != OversizePolicyTile {
		t.Fatalf("unexpected oversize policy: %s", req.OversizePolicy)
	}
	if req.Tile.MaxWidth != 4000 || req.Tile.MaxHeight != 2000 || req.Tile.Overlap != 10 {
		t.Fatalf("unexpected tile merge: %+v", req.Tile)
	}
}

func TestManifestTileShotOverridesCanDisableBooleanDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(`
output_dir: shots
oversize_policy: tile
tile:
  stitch: true
  cleanup_tiles: true
  overlap: 12
shots:
  - id: home
    url: http://localhost:3000
    tile:
      stitch: false
      cleanup_tiles: false
      overlap: 0
`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	requests, err := manifest.Requests("")
	if err != nil {
		t.Fatalf("Requests returned error: %v", err)
	}
	if requests[0].Tile.Stitch || requests[0].Tile.CleanupTiles || requests[0].Tile.Overlap != 0 {
		t.Fatalf("shot tile options should override manifest defaults: %+v", requests[0].Tile)
	}
}
