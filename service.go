package webcap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Engine interface {
	Name() string
	Capture(ctx context.Context, req CaptureRequest) (EngineResult, error)
}

type CaptureService interface {
	CaptureArtifact(ctx context.Context, req CaptureRequest) (CaptureArtifact, error)
	Capture(ctx context.Context, req CaptureRequest) (CaptureResult, error)
	CaptureBatch(ctx context.Context, manifest Manifest, outputDir string) (BatchResult, error)
}

type Service struct {
	engine   Engine
	now      func() time.Time
	workflow WorkflowOptions
}

func NewService(engine Engine) *Service {
	return NewServiceWithOptions(engine, Options{})
}

func NewServiceWithOptions(engine Engine, opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		engine:   engine,
		now:      now,
		workflow: opts.Workflow.normalized(),
	}
}

func (s *Service) CaptureArtifact(ctx context.Context, req CaptureRequest) (CaptureArtifact, error) {
	if s == nil || s.engine == nil {
		return CaptureArtifact{}, newCaptureError(CodeCapture, "capture_artifact", "webcap engine is not configured", nil)
	}
	normalized, err := NormalizeCaptureRequest(req)
	if err != nil {
		return CaptureArtifact{}, err
	}
	result, err := s.engine.Capture(ctx, normalized)
	if err != nil {
		return CaptureArtifact{}, err
	}
	return result.Artifact, nil
}

func (s *Service) Capture(ctx context.Context, req CaptureRequest) (CaptureResult, error) {
	if s == nil || s.engine == nil {
		return CaptureResult{}, newCaptureError(CodeCapture, "capture", "webcap engine is not configured", nil)
	}
	normalized, err := NormalizeCaptureRequest(req)
	if err != nil {
		return CaptureResult{}, err
	}
	resolved, outputBaseName, outputGenerated, err := ResolvePersistedPaths(normalized, "")
	if err != nil {
		return CaptureResult{}, wrapCaptureError("resolve_output_paths", err)
	}

	engineResult, err := s.engine.Capture(ctx, resolved)
	if err != nil {
		return CaptureResult{}, wrapCaptureError("capture", err)
	}

	if err := writeFile(resolved.OutputPath, engineResult.Artifact.Bytes); err != nil {
		return CaptureResult{}, wrapCaptureError("write_image", err)
	}

	result := CaptureResult{
		OutputPath:     resolved.OutputPath,
		MetadataPath:   resolved.MetadataPath,
		ByteSize:       len(engineResult.Artifact.Bytes),
		Artifact:       engineResult.Artifact,
		CapturedAt:     engineResult.Timing.CapturedAt,
		Engine:         s.engine.Name(),
		Browser:        engineResult.Browser,
		Timing:         engineResult.Timing,
		Warnings:       cloneWarnings(engineResult.Warnings),
		Normalization:  resolved.Normalization(outputGenerated, outputBaseName, filepath.Dir(resolved.OutputPath)),
		ResolvedConfig: resolved,
	}
	if result.CapturedAt.IsZero() {
		result.CapturedAt = s.now()
	}

	if resolved.MetadataPath != "" {
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return CaptureResult{}, wrapCaptureError("marshal_metadata", err)
		}
		if err := writeFile(resolved.MetadataPath, append(encoded, '\n')); err != nil {
			return CaptureResult{}, wrapCaptureError("write_metadata", err)
		}
	}

	return result, nil
}

func (s *Service) CaptureBatch(ctx context.Context, manifest Manifest, outputDir string) (BatchResult, error) {
	requests, err := manifest.Requests(outputDir)
	if err != nil {
		return BatchResult{}, err
	}
	results := make([]CaptureResult, 0, len(requests))
	for _, req := range requests {
		result, err := s.Capture(ctx, req)
		if err != nil {
			return BatchResult{}, err
		}
		results = append(results, result)
	}
	return BatchResult{Results: results}, nil
}

func writeFile(path string, payload []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return newCaptureError(CodeWrite, "write_file", "create output directory failed", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return newCaptureError(CodeWrite, "write_file", "write output file failed", err)
	}
	return nil
}
