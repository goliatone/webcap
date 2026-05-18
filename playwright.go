package webcap

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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

type playwrightBridgeError struct {
	Message   string         `json:"message"`
	Code      ErrorCode      `json:"code"`
	Operation string         `json:"operation"`
	Metadata  map[string]any `json:"metadata,omitempty"`
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
	if effectiveOversizePolicy(normalized) == OversizePolicyTile {
		return EngineResult{}, newCaptureError(CodeUnsupported, "playwright_capture", "playwright does not support oversize_policy=tile yet", nil).
			WithMetadata("engine", e.Name()).
			WithMetadata("browser", e.opts.BrowserName).
			WithMetadata("mode", normalized.Mode()).
			WithMetadata("oversize_policy", effectiveOversizePolicy(normalized)).
			WithMetadata("guidance", "use the chromium engine for tiled captures or omit oversize_policy=tile")
	}

	payload := playwrightBridgeRequest{Request: normalized}
	payload.Options.BrowserName = e.opts.BrowserName
	payload.Options.BrowserPath = strings.TrimSpace(e.opts.BrowserPath)
	payload.Options.Headless = e.opts.Headless

	encoded, err := json.Marshal(payload)
	if err != nil {
		return EngineResult{}, wrapCaptureError("playwright_marshal_request", err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, normalized.TimeoutDuration())
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, e.opts.NodeBinary, e.opts.ScriptPath)
	cmd.Dir = e.opts.RuntimeDir
	cmd.Stdin = strings.NewReader(string(encoded))
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			return EngineResult{}, playwrightCaptureTimeoutError(normalized, err)
		}
		return EngineResult{}, playwrightCaptureError(strings.TrimSpace(string(output)), err)
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

func playwrightCaptureTimeoutError(req CaptureRequest, err error) error {
	if strings.TrimSpace(req.WaitForFunction) != "" {
		return newCaptureError(CodeTimeout, "wait_ready", "wait_for_function did not become truthy before timeout", err).
			WithMetadata("wait", "wait_for_function")
	}
	return newCaptureError(CodeTimeout, "playwright_capture", "capture timed out", err)
}

func playwrightCaptureError(output string, err error) error {
	message := strings.TrimSpace(output)
	var bridgeErr playwrightBridgeError
	if unmarshalErr := json.Unmarshal([]byte(message), &bridgeErr); unmarshalErr == nil && strings.TrimSpace(bridgeErr.Message) != "" {
		code := bridgeErr.Code
		if code == "" {
			code = CodeCapture
		}
		operation := strings.TrimSpace(bridgeErr.Operation)
		if operation == "" {
			operation = "playwright_capture"
		}
		captureErr := newCaptureError(code, operation, bridgeErr.Message, err)
		if len(bridgeErr.Metadata) > 0 {
			captureErr.Metadata = bridgeErr.Metadata
		}
		return captureErr
	}
	return newCaptureError(CodeCapture, "playwright_capture", message, err)
}
