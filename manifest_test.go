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
		Wait:            "250ms",
		WaitForFunction: "window.defaultReady",
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
	if req.WaitForFunction != "window.defaultReady" {
		t.Fatalf("unexpected wait_for_function: %s", req.WaitForFunction)
	}
	if req.Viewport.Width != 1600 || req.Viewport.Height != 900 {
		t.Fatalf("unexpected viewport: %+v", req.Viewport)
	}
	if req.Readiness != defaultReadinessMode {
		t.Fatalf("unexpected readiness: %s", req.Readiness)
	}
}

func TestManifestRequestsWaitForFunctionShotOverridesDefault(t *testing.T) {
	manifest := Manifest{
		WaitForFunction: "window.defaultReady",
		Shots: []ManifestShot{
			{
				URL:             "http://localhost:3000",
				WaitForFunction: "window.shotReady",
			},
		},
	}

	requests, err := manifest.Requests("")
	if err != nil {
		t.Fatalf("Requests returned error: %v", err)
	}
	if requests[0].WaitForFunction != "window.shotReady" {
		t.Fatalf("unexpected wait_for_function: %s", requests[0].WaitForFunction)
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

func TestManifestRequestsMergeAuthAndGuards(t *testing.T) {
	dir := t.TempDir()
	jarPath := filepath.Join(dir, "cookies.txt")
	statePath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(jarPath, []byte("# cookie jar\n"), 0o644); err != nil {
		t.Fatalf("write jar: %v", err)
	}
	if err := os.WriteFile(statePath, []byte(`{"cookies":[],"origins":[]}`), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	manifestPath := filepath.Join(dir, "webcap.yaml")
	if err := os.WriteFile(manifestPath, []byte(`
auth:
  cookie_jar: cookies.txt
  headers:
    - name: X-Test
      value: default-secret
  cookies:
    - name: sid
      value: default-cookie
      path: /
guards:
  expect_url: /admin
  fail_on_url:
    - /login
shots:
  - id: queue
    url: http://localhost:3000/admin/translations/queue
    auth:
      storage_state: state.json
      headers:
        - name: x-test
          value: shot-secret
    guards:
      fail_on_selector:
        - form[action="/login"]
`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	requests, err := manifest.Requests("")
	if err != nil {
		t.Fatalf("Requests returned error: %v", err)
	}
	req := requests[0]
	if req.Auth.CookieJar != jarPath || req.Auth.StorageState != statePath {
		t.Fatalf("auth paths were not resolved relative to manifest: %#v", req.Auth)
	}
	if len(req.Auth.Headers) != 1 || req.Auth.Headers[0].Name != "x-test" || req.Auth.Headers[0].Value != "shot-secret" {
		t.Fatalf("headers were not merged with replacement: %#v", req.Auth.Headers)
	}
	if len(req.Auth.Cookies) != 1 || req.Auth.Cookies[0].Name != "sid" {
		t.Fatalf("cookies were not inherited: %#v", req.Auth.Cookies)
	}
	if req.Guards.ExpectURL != "/admin" || len(req.Guards.FailOnURL) != 1 || len(req.Guards.FailOnSelector) != 1 {
		t.Fatalf("guards were not merged: %#v", req.Guards)
	}
}
