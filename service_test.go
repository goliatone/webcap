package webcap

import (
	"context"
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
