package webcap

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type PlaywrightOptions struct {
	NodeBinary  string
	BrowserName string
	BrowserPath string
	Headless    bool
	RuntimeDir  string
	ScriptPath  string
}

type PlaywrightEngine struct {
	opts PlaywrightOptions
}

type playwrightBridgeRequest struct {
	Request CaptureRequest `json:"request"`
	Options struct {
		BrowserName string `json:"browser_name"`
		BrowserPath string `json:"browser_path,omitempty"`
		Headless    bool   `json:"headless"`
	} `json:"options"`
}

type playwrightBridgeResponse struct {
	Artifact    CaptureArtifact  `json:"artifact"`
	Browser     BrowserInfo      `json:"browser"`
	Timing      CaptureTiming    `json:"timing"`
	Warnings    []CaptureWarning `json:"warnings,omitempty"`
	BytesBase64 string           `json:"bytes_base64"`
}

func NewPlaywrightEngine(opts PlaywrightOptions) (*PlaywrightEngine, error) {
	if opts.NodeBinary == "" {
		opts.NodeBinary = "node"
	}
	if strings.TrimSpace(opts.BrowserName) == "" {
		opts.BrowserName = "chromium"
	}
	opts.BrowserName = strings.TrimSpace(strings.ToLower(opts.BrowserName))
	switch opts.BrowserName {
	case "chromium", "firefox", "webkit":
	default:
		return nil, newCaptureError(CodeValidation, "new_playwright_engine", fmt.Sprintf("unsupported playwright browser %q", opts.BrowserName), nil)
	}
	if strings.TrimSpace(opts.RuntimeDir) == "" {
		opts.RuntimeDir = DefaultPlaywrightRuntimeDir()
	}
	if !filepath.IsAbs(opts.RuntimeDir) {
		absoluteRuntimeDir, err := filepath.Abs(opts.RuntimeDir)
		if err != nil {
			return nil, wrapCaptureError("new_playwright_engine", err)
		}
		opts.RuntimeDir = absoluteRuntimeDir
	}
	if strings.TrimSpace(opts.ScriptPath) == "" {
		opts.ScriptPath = filepath.Join(opts.RuntimeDir, "capture.mjs")
	} else if !filepath.IsAbs(opts.ScriptPath) {
		opts.ScriptPath = filepath.Join(opts.RuntimeDir, opts.ScriptPath)
	}
	return &PlaywrightEngine{opts: opts}, nil
}

func (e *PlaywrightEngine) Name() string {
	return "playwright"
}

func (e *PlaywrightEngine) Capture(ctx context.Context, req CaptureRequest) (EngineResult, error) {
	normalized, err := NormalizeCaptureRequest(req)
	if err != nil {
		return EngineResult{}, err
	}

	payload := playwrightBridgeRequest{Request: normalized}
	payload.Options.BrowserName = e.opts.BrowserName
	payload.Options.BrowserPath = strings.TrimSpace(e.opts.BrowserPath)
	payload.Options.Headless = e.opts.Headless

	encoded, err := json.Marshal(payload)
	if err != nil {
		return EngineResult{}, wrapCaptureError("playwright_marshal_request", err)
	}

	cmd := exec.CommandContext(ctx, e.opts.NodeBinary, e.opts.ScriptPath)
	cmd.Dir = e.opts.RuntimeDir
	cmd.Stdin = strings.NewReader(string(encoded))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return EngineResult{}, newCaptureError(CodeCapture, "playwright_capture", strings.TrimSpace(string(output)), err)
	}

	var response playwrightBridgeResponse
	if unmarshalErr := json.Unmarshal(output, &response); unmarshalErr != nil {
		return EngineResult{}, wrapCaptureError("playwright_unmarshal_response", unmarshalErr)
	}
	bytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(response.BytesBase64))
	if err != nil {
		return EngineResult{}, wrapCaptureError("playwright_decode_image", err)
	}
	response.Artifact.Bytes = bytes
	return EngineResult{
		Artifact: response.Artifact,
		Browser:  response.Browser,
		Timing:   response.Timing,
		Warnings: cloneWarnings(response.Warnings),
	}, nil
}
