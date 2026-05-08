package webcap

import "testing"

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
