package presentation

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	pkgwebcap "github.com/goliatone/webcap"
	"github.com/goliatone/webcap/pkg/agents/skills"
)

func TestJSONPresenterPreservesResultShape(t *testing.T) {
	result := sampleCaptureResult()
	var buf bytes.Buffer
	if err := New(Options{Format: FormatJSON}).Present(&buf, result); err != nil {
		t.Fatalf("Present returned error: %v", err)
	}
	var decoded pkgwebcap.CaptureResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if decoded.OutputPath != result.OutputPath || decoded.ByteSize != result.ByteSize {
		t.Fatalf("unexpected decoded result: %#v", decoded)
	}
	if !strings.Contains(buf.String(), "\n  \"output_path\"") {
		t.Fatalf("expected indented JSON, got:\n%s", buf.String())
	}
}

func TestJSONPresenterRendersStructuredErrors(t *testing.T) {
	err := skills.ConflictError{Path: "/tmp/webcap-agent/SKILL.md"}
	var buf bytes.Buffer
	if writeErr := New(Options{Format: FormatJSON}).PresentError(&buf, err); writeErr != nil {
		t.Fatalf("PresentError returned error: %v", writeErr)
	}
	var envelope ErrorEnvelope
	if decodeErr := json.Unmarshal(buf.Bytes(), &envelope); decodeErr != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", decodeErr, buf.String())
	}
	if envelope.Code != "skill_conflict" || envelope.Operation != "skill_install" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if envelope.Metadata["path"] != "/tmp/webcap-agent/SKILL.md" {
		t.Fatalf("unexpected metadata: %#v", envelope.Metadata)
	}
}

func TestHumanCaptureOutputNoColorIsStable(t *testing.T) {
	var buf bytes.Buffer
	if err := New(Options{Format: FormatHuman, Color: false}).Present(&buf, sampleCaptureResult()); err != nil {
		t.Fatalf("Present returned error: %v", err)
	}
	output := buf.String()
	for _, expected := range []string{
		"Capture complete",
		"Output: shots/home.png",
		"Bytes: 42",
		"Engine: chromium",
		"Warnings:",
		"viewport: viewport adjusted",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestHumanBatchOutputIncludesEachCapture(t *testing.T) {
	var buf bytes.Buffer
	result := pkgwebcap.BatchResult{Results: []pkgwebcap.CaptureResult{
		sampleCaptureResult(),
		func() pkgwebcap.CaptureResult {
			item := sampleCaptureResult()
			item.OutputPath = "shots/about.png"
			item.Warnings = nil
			return item
		}(),
	}}
	if err := New(Options{Format: FormatHuman}).Present(&buf, result); err != nil {
		t.Fatalf("Present returned error: %v", err)
	}
	output := buf.String()
	for _, expected := range []string{"Captures: 2", "shots/home.png", "shots/about.png"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestHumanErrorOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := New(Options{Format: FormatHuman}).PresentError(&buf, errors.New("boom")); err != nil {
		t.Fatalf("PresentError returned error: %v", err)
	}
	if got := buf.String(); got != "Error: boom\n" {
		t.Fatalf("unexpected human error: %q", got)
	}
}

func sampleCaptureResult() pkgwebcap.CaptureResult {
	capturedAt := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	return pkgwebcap.CaptureResult{
		OutputPath:   "shots/home.png",
		MetadataPath: "shots/home.png.json",
		ByteSize:     42,
		CapturedAt:   capturedAt,
		Engine:       "chromium",
		Artifact: pkgwebcap.CaptureArtifact{
			ImageFormat: "png",
			URL:         "https://example.test",
			Viewport:    pkgwebcap.Viewport{Width: 1440, Height: 900},
		},
		Browser: pkgwebcap.BrowserInfo{Engine: "chromium", Headless: true},
		Timing:  pkgwebcap.CaptureTiming{TotalDuration: "120ms"},
		Warnings: []pkgwebcap.CaptureWarning{{
			Code:    "viewport",
			Message: "viewport adjusted",
		}},
		ResolvedConfig: pkgwebcap.CaptureRequest{Readiness: pkgwebcap.ReadinessComplete},
	}
}
