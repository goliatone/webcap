package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	pkgwebcap "github.com/goliatone/webcap"
	"github.com/goliatone/webcap/pkg/llms"
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
	for _, tool := range result.Tools {
		if tool.Name != "capture_page" && tool.Name != "capture_section" {
			continue
		}
		properties, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s properties schema missing", tool.Name)
		}
		if _, ok := properties["wait_for_function"]; !ok {
			t.Fatalf("%s schema missing wait_for_function", tool.Name)
		}
		if _, ok := properties["auth"]; !ok {
			t.Fatalf("%s schema missing auth", tool.Name)
		}
		if _, ok := properties["guards"]; !ok {
			t.Fatalf("%s schema missing guards", tool.Name)
		}
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

func TestCapturePageAcceptsTileOptions(t *testing.T) {
	capture := &fakeCaptureService{captureResult: sampleCaptureResult("/tmp/out.png")}
	server, err := NewServer(Config{
		Name:    "webcap",
		Version: "0.1.0",
		Capture: capture,
		Diff:    &fakeDiffService{result: sampleDiffResult()},
		LoadManifest: func(string) (pkgwebcap.Manifest, error) {
			return pkgwebcap.Manifest{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	session := server.NewSession()
	initializeSession(t, session)

	resp := session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"capture_page","arguments":{"url":"http://localhost:3000","wait_for_function":" window.__webcapReady ","oversize_policy":"tile","tile":{"max_width":4096,"stitch":true}}}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call returned protocol error: %#v", resp)
	}
	if capture.lastCapture.OversizePolicy != pkgwebcap.OversizePolicyTile || capture.lastCapture.Tile.MaxWidth != 4096 || !capture.lastCapture.Tile.Stitch {
		t.Fatalf("tile options were not forwarded: %+v", capture.lastCapture)
	}
	if capture.lastCapture.WaitForFunction != "window.__webcapReady" {
		t.Fatalf("wait_for_function was not forwarded: %+v", capture.lastCapture)
	}
}

func TestCapturePageForwardsAuthAndGuards(t *testing.T) {
	captureResult := sampleCaptureResult("/tmp/out.png")
	captureResult.Guards = []pkgwebcap.GuardOutcome{{
		Kind:     "expect_url",
		Value:    "/admin",
		FinalURL: "http://localhost:3000/admin",
		Matched:  true,
		Status:   "passed",
	}}
	capture := &fakeCaptureService{captureResult: captureResult}
	server, err := NewServer(Config{
		Name:    "webcap",
		Version: "0.1.0",
		Capture: capture,
		Diff:    &fakeDiffService{result: sampleDiffResult()},
		LoadManifest: func(string) (pkgwebcap.Manifest, error) {
			return pkgwebcap.Manifest{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	session := server.NewSession()
	initializeSession(t, session)

	resp := session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"capture_page","arguments":{"url":"http://localhost:3000/admin","auth":{"headers":[{"name":"Authorization","value":"Bearer secret"}],"cookies":[{"name":"sid","value":"cookie-secret","path":"/"}]},"guards":{"expect_url":"/admin","fail_on_url":["/login"],"fail_on_selector":["form.login"]}}}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call returned protocol error: %#v", resp)
	}
	if len(capture.lastCapture.Auth.Headers) != 1 || capture.lastCapture.Auth.Headers[0].Name != "Authorization" {
		t.Fatalf("auth headers were not forwarded: %#v", capture.lastCapture.Auth)
	}
	if len(capture.lastCapture.Auth.Cookies) != 1 || capture.lastCapture.Auth.Cookies[0].Name != "sid" {
		t.Fatalf("auth cookies were not forwarded: %#v", capture.lastCapture.Auth)
	}
	if capture.lastCapture.Guards.ExpectURL != "/admin" || len(capture.lastCapture.Guards.FailOnURL) != 1 || len(capture.lastCapture.Guards.FailOnSelector) != 1 {
		t.Fatalf("guards were not forwarded: %#v", capture.lastCapture.Guards)
	}
	result, ok := resp.Result.(callToolResult)
	if !ok {
		t.Fatalf("unexpected tools/call result type: %T", resp.Result)
	}
	structured, ok := result.StructuredContent.(captureToolResult)
	if !ok {
		t.Fatalf("unexpected structured capture result type: %T", result.StructuredContent)
	}
	if len(structured.Guards) != 1 || structured.Guards[0].Kind != "expect_url" || structured.Guards[0].Status != "passed" {
		t.Fatalf("structured result missing guard outcomes: %#v", structured.Guards)
	}
}

func TestCaptureSectionForwardsAuthAndGuards(t *testing.T) {
	capture := &fakeCaptureService{captureResult: sampleCaptureResult("/tmp/section.png")}
	server, err := NewServer(Config{
		Name:    "webcap",
		Version: "0.1.0",
		Capture: capture,
		Diff:    &fakeDiffService{result: sampleDiffResult()},
		LoadManifest: func(string) (pkgwebcap.Manifest, error) {
			return pkgwebcap.Manifest{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	session := server.NewSession()
	initializeSession(t, session)

	resp := session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"capture_section","arguments":{"url":"http://localhost:3000/admin","selector":"#queue","auth":{"headers":[{"name":"Authorization","value":"Bearer secret"}]},"guards":{"expect_url":"/admin","fail_on_url":["/login"],"fail_on_selector":["form.login"]}}}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call returned protocol error: %#v", resp)
	}
	if capture.lastCapture.Selector != "#queue" {
		t.Fatalf("selector was not forwarded: %#v", capture.lastCapture)
	}
	if len(capture.lastCapture.Auth.Headers) != 1 || capture.lastCapture.Auth.Headers[0].Name != "Authorization" {
		t.Fatalf("auth headers were not forwarded: %#v", capture.lastCapture.Auth)
	}
	if capture.lastCapture.Guards.ExpectURL != "/admin" || len(capture.lastCapture.Guards.FailOnURL) != 1 || len(capture.lastCapture.Guards.FailOnSelector) != 1 {
		t.Fatalf("guards were not forwarded: %#v", capture.lastCapture.Guards)
	}
}

func TestCaptureSectionForwardsWaitForFunction(t *testing.T) {
	capture := &fakeCaptureService{captureResult: sampleCaptureResult("/tmp/out.png")}
	server, err := NewServer(Config{
		Name:    "webcap",
		Version: "0.1.0",
		Capture: capture,
		Diff:    &fakeDiffService{result: sampleDiffResult()},
		LoadManifest: func(string) (pkgwebcap.Manifest, error) {
			return pkgwebcap.Manifest{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	session := server.NewSession()
	initializeSession(t, session)

	resp := session.handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"capture_section","arguments":{"url":"http://localhost:3000","selector":"#app","wait_for_function":"() => window.__webcapReady"}}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call returned protocol error: %#v", resp)
	}
	if capture.lastCapture.WaitForFunction != "() => window.__webcapReady" {
		t.Fatalf("wait_for_function was not forwarded: %+v", capture.lastCapture)
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

func TestSemanticDiffToolCallUsesDefaultServiceProvider(t *testing.T) {
	serverHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mcp-key" {
			t.Fatalf("expected OpenAI auth header, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"summary\":\"MCP semantic ok\",\"verdict\":\"no_meaningful_change\",\"severity\":\"info\"}"}]}]}`))
	}))
	defer serverHTTP.Close()

	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	writeMCPTestPNG(t, currentPath, color.NRGBA{R: 255, A: 255})
	writeMCPTestPNG(t, referencePath, color.NRGBA{B: 255, A: 255})

	service := pkgwebcap.NewServiceWithOptions(nil, pkgwebcap.Options{SemanticDiff: pkgwebcap.SemanticDiffOptions{
		CredentialResolver: func(context.Context, string) (string, error) { return "mcp-key", nil },
		OpenAIBaseURL:      serverHTTP.URL,
	}})
	server, err := NewServer(Config{
		Name:         "webcap",
		Version:      "0.1.0",
		Capture:      &fakeCaptureService{captureResult: sampleCaptureResult("/tmp/out.png")},
		Diff:         service,
		LoadManifest: func(string) (pkgwebcap.Manifest, error) { return pkgwebcap.Manifest{}, nil },
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	session := server.NewSession()
	initializeSession(t, session)

	request := fmt.Sprintf(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"semantic_diff","arguments":{"current_path":%q,"reference_path":%q,"provider":"openai","model":"gpt-test","metadata_path":%q}}}`, currentPath, referencePath, filepath.Join(dir, "semantic.json"))
	resp := session.handle(context.Background(), []byte(request))
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
	structured, ok := result.StructuredContent.(map[string]any)
	if result.IsError || !ok || structured["summary"] != "MCP semantic ok" {
		t.Fatalf("expected default semantic provider result: %#v", result)
	}
}

func TestSemanticDiffToolCallUsesCodexCLIServiceProvider(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake uses POSIX sh")
	}
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.png")
	referencePath := filepath.Join(dir, "reference.png")
	writeMCPTestPNG(t, currentPath, color.NRGBA{R: 255, A: 255})
	writeMCPTestPNG(t, referencePath, color.NRGBA{B: 255, A: 255})
	fake := writeMCPFakeCodex(t, dir, `#!/bin/sh
cat >/dev/null
printf '{"summary":"MCP codex ok","verdict":"no_meaningful_change","severity":"info"}\n'
`)

	service := pkgwebcap.NewServiceWithOptions(nil, pkgwebcap.Options{SemanticDiff: pkgwebcap.SemanticDiffOptions{
		LLMs: llms.Options{CodexCLI: llms.CodexCLIOptions{CommandPath: fake}},
	}})
	server, err := NewServer(Config{
		Name:         "webcap",
		Version:      "0.1.0",
		Capture:      &fakeCaptureService{captureResult: sampleCaptureResult("/tmp/out.png")},
		Diff:         service,
		LoadManifest: func(string) (pkgwebcap.Manifest, error) { return pkgwebcap.Manifest{}, nil },
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	session := server.NewSession()
	initializeSession(t, session)

	request := fmt.Sprintf(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"semantic_diff","arguments":{"current_path":%q,"reference_path":%q,"provider":"codex-cli","model":"gpt-test","metadata_path":%q}}}`, currentPath, referencePath, filepath.Join(dir, "semantic.json"))
	resp := session.handle(context.Background(), []byte(request))
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
	structured, ok := result.StructuredContent.(map[string]any)
	if result.IsError || !ok || structured["summary"] != "MCP codex ok" {
		t.Fatalf("expected codex semantic provider result: %#v", result)
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

func writeMCPTestPNG(t *testing.T, path string, c color.NRGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create image dir: %v", err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, c)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatalf("encode image: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close image: %v", err)
	}
}

func writeMCPFakeCodex(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "codex-fake")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	return path
}
