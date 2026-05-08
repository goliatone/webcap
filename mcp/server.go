package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	pkgwebcap "github.com/goliatone/webcap"
	commandwebcap "github.com/goliatone/webcap/commands/webcap"
)

type Server struct {
	name         string
	version      string
	capture      pkgwebcap.CaptureService
	diff         pkgwebcap.DiffService
	loadManifest func(string) (pkgwebcap.Manifest, error)
}

type Config struct {
	Name         string
	Version      string
	Capture      pkgwebcap.CaptureService
	Diff         pkgwebcap.DiffService
	LoadManifest func(string) (pkgwebcap.Manifest, error)
}

type Session struct {
	server            *Server
	protocolVersion   string
	initialized       bool
	clientInitialized bool
}

func NewServer(config Config) (*Server, error) {
	if config.Capture == nil {
		return nil, fmt.Errorf("webcap mcp capture service is required")
	}
	if config.Diff == nil {
		return nil, fmt.Errorf("webcap mcp diff service is required")
	}
	if config.LoadManifest == nil {
		config.LoadManifest = pkgwebcap.LoadManifest
	}
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = "webcap"
	}
	version := strings.TrimSpace(config.Version)
	if version == "" {
		version = "0.1.0"
	}
	return &Server{
		name:         name,
		version:      version,
		capture:      config.Capture,
		diff:         config.Diff,
		loadManifest: config.LoadManifest,
	}, nil
}

func (s *Server) NewSession() *Session {
	return &Session{
		server:          s,
		protocolVersion: defaultProtocolVersion,
	}
}

func (s *Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	if s == nil {
		return fmt.Errorf("webcap mcp server is not configured")
	}
	reader := bufio.NewReader(input)
	writer := bufio.NewWriter(output)
	session := s.NewSession()

	for {
		payload, err := readMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		resp := session.handle(ctx, payload)
		if resp == nil {
			continue
		}
		if err := writeMessage(writer, resp); err != nil {
			return err
		}
	}
}

func (s *Session) handle(ctx context.Context, payload []byte) *response {
	if s == nil || s.server == nil {
		return &response{
			JSONRPC: "2.0",
			Error: &responseError{
				Code:    errInternalError,
				Message: "webcap mcp session is not configured",
			},
		}
	}

	var req request
	if err := json.Unmarshal(payload, &req); err != nil {
		return &response{
			JSONRPC: "2.0",
			Error: &responseError{
				Code:    errParseError,
				Message: "invalid JSON payload",
			},
		}
	}
	if req.JSONRPC != "2.0" {
		return s.errorResponse(req.ID, errInvalidRequest, "jsonrpc must be 2.0", nil)
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		s.clientInitialized = true
		return nil
	case "notifications/cancelled":
		return nil
	case "ping":
		return s.resultResponse(req.ID, map[string]any{})
	}

	if !s.initialized {
		return s.errorResponse(req.ID, errInvalidRequest, "initialize must be the first request", nil)
	}
	if !s.clientInitialized {
		return s.errorResponse(req.ID, errInvalidRequest, "client must send notifications/initialized before other requests", nil)
	}

	switch req.Method {
	case "tools/list":
		return s.resultResponse(req.ID, listToolsResult{Tools: s.server.tools()})
	case "tools/call":
		return s.handleToolCall(ctx, req)
	default:
		return s.errorResponse(req.ID, errMethodNotFound, fmt.Sprintf("unsupported method %q", req.Method), nil)
	}
}

func (s *Session) handleInitialize(req request) *response {
	var params initializeParams
	if err := decodeJSON(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, errInvalidParams, "invalid initialize params", nil)
	}
	s.protocolVersion = supportedProtocolVersion(params.ProtocolVersion)
	s.initialized = true
	return s.resultResponse(req.ID, map[string]any{
		"protocolVersion": s.protocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": implementation{
			Name:    s.server.name,
			Title:   "Webcap",
			Version: s.server.version,
		},
		"instructions": "Use webcap tools to capture screenshots, run manifest batches, and compare image artifacts. Tool results return file paths and compact metadata, not base64 image payloads.",
	})
}

func (s *Session) handleToolCall(ctx context.Context, req request) *response {
	var params callToolParams
	if err := decodeJSON(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, errInvalidParams, "invalid tools/call params", nil)
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return s.errorResponse(req.ID, errInvalidParams, "tool name is required", nil)
	}

	result, err := s.server.callTool(ctx, name, params.Arguments)
	if err != nil {
		return s.errorResponse(req.ID, errInvalidParams, err.Error(), nil)
	}
	return s.resultResponse(req.ID, result)
}

func (s *Session) resultResponse(id json.RawMessage, result any) *response {
	return &response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func (s *Session) errorResponse(id json.RawMessage, code int, message string, data any) *response {
	return &response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &responseError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func (s *Server) callTool(ctx context.Context, name string, raw json.RawMessage) (callToolResult, error) {
	switch name {
	case "capture_page":
		return s.capturePage(ctx, raw)
	case "capture_section":
		return s.captureSection(ctx, raw)
	case "capture_manifest":
		return s.captureManifest(ctx, raw)
	case "compare_images":
		return s.compareImages(ctx, raw)
	default:
		return callToolResult{}, fmt.Errorf("unknown tool %q", name)
	}
}

func (s *Server) capturePage(ctx context.Context, raw json.RawMessage) (callToolResult, error) {
	var args capturePageArguments
	if err := decodeJSON(raw, &args); err != nil {
		return errorToolResult("capture_page", fmt.Errorf("invalid arguments"))
	}
	if hasSectionSelectors(args.Selector, args.Selectors, args.SelectorAll, args.SelectorsAll) {
		return errorToolResult("capture_page", fmt.Errorf("capture_page does not accept selector arguments"))
	}
	handler := commandwebcap.NewCaptureShotHandler(s.capture)
	result, err := handler.Handle(ctx, commandwebcap.CaptureShotMessage{
		Request: args.captureRequest(),
	})
	if err != nil {
		return errorToolResult("capture_page", err)
	}
	summary := summarizeCaptureResult(result)
	return successToolResult(summary, fmt.Sprintf("Captured %s screenshot to %s", summary.Mode, summary.OutputPath)), nil
}

func (s *Server) captureSection(ctx context.Context, raw json.RawMessage) (callToolResult, error) {
	var args captureSectionArguments
	if err := decodeJSON(raw, &args); err != nil {
		return errorToolResult("capture_section", fmt.Errorf("invalid arguments"))
	}
	if !hasSectionSelectors(args.Selector, args.Selectors, args.SelectorAll, args.SelectorsAll) {
		return errorToolResult("capture_section", fmt.Errorf("capture_section requires selector, selectors, selector_all, or selectors_all"))
	}
	handler := commandwebcap.NewCaptureShotHandler(s.capture)
	result, err := handler.Handle(ctx, commandwebcap.CaptureShotMessage{
		Request: args.captureRequest(),
	})
	if err != nil {
		return errorToolResult("capture_section", err)
	}
	summary := summarizeCaptureResult(result)
	return successToolResult(summary, fmt.Sprintf("Captured %d matched section(s) to %s", summary.MatchCount, summary.OutputPath)), nil
}

func (s *Server) captureManifest(ctx context.Context, raw json.RawMessage) (callToolResult, error) {
	var args captureManifestArguments
	if err := decodeJSON(raw, &args); err != nil {
		return errorToolResult("capture_manifest", fmt.Errorf("invalid arguments"))
	}
	if strings.TrimSpace(args.ManifestPath) == "" {
		return errorToolResult("capture_manifest", fmt.Errorf("manifest_path is required"))
	}
	manifest, err := s.loadManifest(args.ManifestPath)
	if err != nil {
		return errorToolResult("capture_manifest", err)
	}
	handler := commandwebcap.NewCaptureBatchHandler(s.capture)
	result, err := handler.Handle(ctx, commandwebcap.CaptureBatchMessage{
		Manifest:  manifest,
		OutputDir: strings.TrimSpace(args.OutputDir),
	})
	if err != nil {
		return errorToolResult("capture_manifest", err)
	}
	summary := summarizeBatchResult(result)
	return successToolResult(summary, fmt.Sprintf("Captured %d manifest artifact(s)", summary.Count)), nil
}

func (s *Server) compareImages(ctx context.Context, raw json.RawMessage) (callToolResult, error) {
	var args compareImagesArguments
	if err := decodeJSON(raw, &args); err != nil {
		return errorToolResult("compare_images", fmt.Errorf("invalid arguments"))
	}
	handler := commandwebcap.NewDiffHandler(s.diff)
	result, err := handler.Handle(ctx, commandwebcap.DiffMessage{
		Request: args.diffRequest(),
	})
	if err != nil {
		return errorToolResult("compare_images", err)
	}
	summary := summarizeDiffResult(result)
	return successToolResult(summary, fmt.Sprintf("Diff completed with %d changed file(s)", summary.Summary.ChangedFiles)), nil
}
