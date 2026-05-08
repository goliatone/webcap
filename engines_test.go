package webcap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNewEngineChromium(t *testing.T) {
	engine, err := NewEngine(EngineConfig{EngineName: EngineChromium, Headless: true})
	if err != nil {
		t.Fatalf("NewEngine returned error: %v", err)
	}
	if engine.Name() != "chromium" {
		t.Fatalf("unexpected engine name: %s", engine.Name())
	}
}

func TestNewEnginePlaywright(t *testing.T) {
	engine, err := NewEngine(EngineConfig{EngineName: EnginePlaywright, PlaywrightBrowser: "firefox", Headless: true})
	if err != nil {
		t.Fatalf("NewEngine returned error: %v", err)
	}
	if engine.Name() != "playwright" {
		t.Fatalf("unexpected engine name: %s", engine.Name())
	}
}

func TestPlaywrightEngineCaptureBridge(t *testing.T) {
	nodeBinary, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "capture.mjs")
	script := `process.stdout.write(JSON.stringify({
  artifact: {
    image_format: "png",
    mode: "viewport",
    url: "http://localhost:3000",
    viewport: { width: 800, height: 600, scale_factor: 1 }
  },
  browser: {
    engine: "playwright",
    product: "firefox/1.0",
    headless: true
  },
  timing: {
    navigation_started_at: "2026-03-30T00:00:00Z",
    captured_at: "2026-03-30T00:00:01Z",
    total_duration: "1s"
  },
  warnings: [],
  bytes_base64: Buffer.from("png").toString("base64")
}));`
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	engine, err := NewPlaywrightEngine(PlaywrightOptions{
		NodeBinary: nodeBinary,
		RuntimeDir: dir,
		ScriptPath: scriptPath,
	})
	if err != nil {
		t.Fatalf("NewPlaywrightEngine returned error: %v", err)
	}
	result, err := engine.Capture(context.Background(), CaptureRequest{
		URL: "http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if string(result.Artifact.Bytes) != "png" {
		t.Fatalf("unexpected payload: %q", string(result.Artifact.Bytes))
	}
	if result.Browser.Engine != "playwright" {
		t.Fatalf("unexpected browser engine: %s", result.Browser.Engine)
	}
}
