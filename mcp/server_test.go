package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	pkgwebcap "github.com/goliatone/webcap"
)

type fakeCaptureService struct {
	captureResult pkgwebcap.CaptureResult
	batchResult   pkgwebcap.BatchResult
	captureErr    error
	batchErr      error
	lastCapture   pkgwebcap.CaptureRequest
	lastOutputDir string
}

func (s *fakeCaptureService) CaptureArtifact(context.Context, pkgwebcap.CaptureRequest) (pkgwebcap.CaptureArtifact, error) {
	return pkgwebcap.CaptureArtifact{}, nil
}

func (s *fakeCaptureService) Capture(_ context.Context, req pkgwebcap.CaptureRequest) (pkgwebcap.CaptureResult, error) {
	s.lastCapture = req
	if s.captureErr != nil {
		return pkgwebcap.CaptureResult{}, s.captureErr
	}
	return s.captureResult, nil
}

func (s *fakeCaptureService) CaptureBatch(_ context.Context, _ pkgwebcap.Manifest, outputDir string) (pkgwebcap.BatchResult, error) {
	s.lastOutputDir = outputDir
	if s.batchErr != nil {
		return pkgwebcap.BatchResult{}, s.batchErr
	}
	return s.batchResult, nil
}

type fakeDiffService struct {
	result  pkgwebcap.DiffResult
	err     error
	lastReq pkgwebcap.DiffRequest
}

func (s *fakeDiffService) Diff(_ context.Context, req pkgwebcap.DiffRequest) (pkgwebcap.DiffResult, error) {
	s.lastReq = req
	if s.err != nil {
		return pkgwebcap.DiffResult{}, s.err
	}
	return s.result, nil
}

type fakeSemanticDiffService struct {
	result  pkgwebcap.SemanticDiffResult
	err     error
	lastReq pkgwebcap.SemanticDiffRequest
}

func (s *fakeSemanticDiffService) SemanticDiff(_ context.Context, req pkgwebcap.SemanticDiffRequest) (pkgwebcap.SemanticDiffResult, error) {
	s.lastReq = req
	if s.err != nil {
		return pkgwebcap.SemanticDiffResult{}, s.err
	}
	return s.result, nil
}

func TestSessionToolsList(t *testing.T) {
	server := mustTestServer(t)
	session := server.NewSession()

	resp := session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("initialize failed: %#v", resp)
	}
	session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))

	resp = session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/list failed: %#v", resp)
	}
	result, ok := resp.Result.(listToolsResult)
	if !ok {
		t.Fatalf("unexpected tools/list result type: %T", resp.Result)
	}
	if len(result.Tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(result.Tools))
	}
}

func TestCaptureSectionValidationError(t *testing.T) {
	server := mustTestServer(t)
	session := server.NewSession()
	initializeSession(t, session)

	resp := session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"capture_section","arguments":{"url":"http://localhost:3000"}}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call returned protocol error: %#v", resp)
	}
	payload, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result callToolResult
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool result to be an error")
	}
	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "requires selector") {
		t.Fatalf("unexpected error text: %#v", result.Content)
	}
}

func TestCaptureManifestToolCall(t *testing.T) {
	capture := &fakeCaptureService{
		batchResult: pkgwebcap.BatchResult{
			Results: []pkgwebcap.CaptureResult{sampleCaptureResult("/tmp/out.png")},
		},
	}
	server, err := NewServer(Config{
		Name:    "webcap",
		Version: "0.1.0",
		Capture: capture,
		Diff:    &fakeDiffService{result: sampleDiffResult()},
		LoadManifest: func(path string) (pkgwebcap.Manifest, error) {
			if path != "webcap.yaml" {
				t.Fatalf("unexpected manifest path: %s", path)
			}
			return pkgwebcap.Manifest{Shots: []pkgwebcap.ManifestShot{{URL: "http://localhost:3000"}}}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	session := server.NewSession()
	initializeSession(t, session)

	resp := session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"capture_manifest","arguments":{"manifest_path":"webcap.yaml","output_dir":"shots"}}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call returned protocol error: %#v", resp)
	}
	payload, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result callToolResult
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful tool result: %#v", result)
	}
	if capture.lastOutputDir != "shots" {
		t.Fatalf("unexpected output dir: %s", capture.lastOutputDir)
	}
}

func TestCompareImagesToolCall(t *testing.T) {
	diff := &fakeDiffService{result: sampleDiffResult()}
	server, err := NewServer(Config{
		Name:    "webcap",
		Version: "0.1.0",
		Capture: &fakeCaptureService{captureResult: sampleCaptureResult("/tmp/out.png")},
		Diff:    diff,
		LoadManifest: func(string) (pkgwebcap.Manifest, error) {
			return pkgwebcap.Manifest{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	session := server.NewSession()
	initializeSession(t, session)

	resp := session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"compare_images","arguments":{"base_path":"base.png","compare_path":"compare.png","threshold":0.2}}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call returned protocol error: %#v", resp)
	}
	payload, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result callToolResult
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful tool result: %#v", result)
	}
	if diff.lastReq.BasePath != "base.png" || diff.lastReq.ComparePath != "compare.png" || diff.lastReq.Threshold != 0.2 {
		t.Fatalf("unexpected diff request: %#v", diff.lastReq)
	}
}

func TestSemanticDiffToolCall(t *testing.T) {
	semantic := &fakeSemanticDiffService{result: sampleSemanticDiffResult()}
	server, err := NewServer(Config{
		Name:         "webcap",
		Version:      "0.1.0",
		Capture:      &fakeCaptureService{captureResult: sampleCaptureResult("/tmp/out.png")},
		Diff:         &fakeDiffService{result: sampleDiffResult()},
		SemanticDiff: semantic,
		LoadManifest: func(string) (pkgwebcap.Manifest, error) {
			return pkgwebcap.Manifest{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	session := server.NewSession()
	initializeSession(t, session)

	resp := session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"semantic_diff","arguments":{"current_path":"current.png","reference_path":"reference.png","provider":"openai","model":"gpt-test","mode":"focused","focus":["CTA"],"metadata_path":"semantic.json","pixel_diff_image_path":"diff.png","changed_pixels":2}}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call returned protocol error: %#v", resp)
	}
	payload, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result callToolResult
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful tool result: %#v", result)
	}
	if semantic.lastReq.CurrentPath != "current.png" || semantic.lastReq.ReferencePath != "reference.png" || semantic.lastReq.Provider != "openai" {
		t.Fatalf("unexpected semantic request: %#v", semantic.lastReq)
	}
	if semantic.lastReq.PixelContext.PixelDiffImagePath != "diff.png" || semantic.lastReq.PixelContext.ChangedPixels != 2 {
		t.Fatalf("expected pixel context: %#v", semantic.lastReq.PixelContext)
	}
}

func TestServeRoundTripCapturePage(t *testing.T) {
	capture := &fakeCaptureService{
		captureResult: sampleCaptureResult("/tmp/out.png"),
	}
	diff := &fakeDiffService{result: sampleDiffResult()}
	server, err := NewServer(Config{
		Name:    "webcap",
		Version: "0.1.0",
		Capture: capture,
		Diff:    diff,
		LoadManifest: func(string) (pkgwebcap.Manifest, error) {
			return pkgwebcap.Manifest{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	input := bytes.NewBufferString(
		"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-11-25\"}}\n" +
			"{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n" +
			"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"capture_page\",\"arguments\":{\"url\":\"http://localhost:3000\",\"full_page\":true,\"output_path\":\"/tmp/out.png\"}}}\n",
	)
	var output bytes.Buffer
	if serveErr := server.Serve(context.Background(), input, &output); serveErr != nil {
		t.Fatalf("Serve returned error: %v", serveErr)
	}

	scanner := bufio.NewScanner(bytes.NewReader(output.Bytes()))
	lines := make([][]byte, 0, 2)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		lines = append(lines, append([]byte(nil), line...))
	}
	if scanErr := scanner.Err(); scanErr != nil {
		t.Fatalf("scanner error: %v", scanErr)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(lines))
	}

	var callResp response
	if unmarshalErr := json.Unmarshal(lines[1], &callResp); unmarshalErr != nil {
		t.Fatalf("unmarshal call response: %v", unmarshalErr)
	}
	if callResp.Error != nil {
		t.Fatalf("unexpected protocol error: %#v", callResp.Error)
	}

	payload, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("marshal call result: %v", err)
	}
	var result callToolResult
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful tool result: %#v", result)
	}
	if capture.lastCapture.URL != "http://localhost:3000" || !capture.lastCapture.FullPage {
		t.Fatalf("unexpected capture request: %#v", capture.lastCapture)
	}
}

func TestReadMessageSupportsContentLength(t *testing.T) {
	body := "{\"jsonrpc\":\"2.0\",\"id\":1}"
	reader := bufio.NewReader(strings.NewReader(
		fmt.Sprintf("Content-Length: %d\r\n\r\n%s\n", len(body), body),
	))
	payload, err := readMessage(reader)
	if err != nil {
		t.Fatalf("readMessage returned error: %v", err)
	}
	if string(payload) != body {
		t.Fatalf("unexpected payload: %s", string(payload))
	}
}

func mustTestServer(t *testing.T) *Server {
	t.Helper()
	server, err := NewServer(Config{
		Name:    "webcap",
		Version: "0.1.0",
		Capture: &fakeCaptureService{
			captureResult: sampleCaptureResult("/tmp/out.png"),
			batchResult: pkgwebcap.BatchResult{
				Results: []pkgwebcap.CaptureResult{sampleCaptureResult("/tmp/out.png")},
			},
		},
		Diff: &fakeDiffService{result: sampleDiffResult()},
		LoadManifest: func(string) (pkgwebcap.Manifest, error) {
			return pkgwebcap.Manifest{Shots: []pkgwebcap.ManifestShot{{URL: "http://localhost:3000"}}}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	return server
}

func initializeSession(t *testing.T, session *Session) {
	t.Helper()
	resp := session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("initialize failed: %#v", resp)
	}
	session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
}

func sampleCaptureResult(path string) pkgwebcap.CaptureResult {
	now := time.Date(2026, 3, 30, 22, 15, 0, 0, time.UTC)
	return pkgwebcap.CaptureResult{
		OutputPath:   path,
		MetadataPath: path + ".json",
		ByteSize:     1024,
		CapturedAt:   now,
		Engine:       "chromium",
		Artifact: pkgwebcap.CaptureArtifact{
			ImageFormat: "png",
			Mode:        pkgwebcap.CaptureModeFullPage,
			URL:         "http://localhost:3000",
			Viewport:    pkgwebcap.Viewport{Width: 1440, Height: 1200, ScaleFactor: 1},
		},
		Browser: pkgwebcap.BrowserInfo{
			Engine:   "chromium",
			Product:  "chrome",
			Headless: true,
		},
	}
}

func sampleDiffResult() pkgwebcap.DiffResult {
	now := time.Date(2026, 3, 30, 22, 15, 0, 0, time.UTC)
	return pkgwebcap.DiffResult{
		Mode:        pkgwebcap.DiffModeImage,
		BasePath:    "/tmp/base.png",
		ComparePath: "/tmp/compare.png",
		OutputPath:  "/tmp/diff.png",
		Threshold:   0.1,
		Summary: pkgwebcap.DiffSummary{
			ComparedFiles: 1,
			ChangedFiles:  1,
		},
		CreatedAt: now,
	}
}

func sampleSemanticDiffResult() pkgwebcap.SemanticDiffResult {
	return pkgwebcap.SemanticDiffResult{
		CurrentPath:   "/tmp/current.png",
		ReferencePath: "/tmp/reference.png",
		Provider:      "openai",
		Model:         "gpt-test",
		Summary:       "CTA moved lower.",
		Verdict:       pkgwebcap.SemanticDiffVerdictNeedsReview,
		Severity:      pkgwebcap.SemanticDiffSeverityMajor,
		Differences: []pkgwebcap.SemanticDifference{{
			Area:        "CTA",
			Description: "CTA moved lower.",
			Severity:    pkgwebcap.SemanticDiffSeverityMajor,
		}},
		MetadataPath: "/tmp/semantic.json",
	}
}
