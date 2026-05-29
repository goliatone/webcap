package webcap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
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
	engine       Engine
	now          func() time.Time
	workflow     WorkflowOptions
	semanticDiff SemanticDiffOptions
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
		engine:       engine,
		now:          now,
		workflow:     opts.Workflow.normalized(),
		semanticDiff: opts.SemanticDiff.normalized(),
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
		var partialErr *PartialCaptureError
		if !errors.As(err, &partialErr) {
			return CaptureResult{}, wrapCaptureError("capture", err)
		}
		result, persistErr := s.persistEngineResult(resolved, outputBaseName, outputGenerated, engineResult)
		if persistErr != nil {
			return result, persistErr
		}
		return result, partialErr.WithResult(result)
	}

	return s.persistEngineResult(resolved, outputBaseName, outputGenerated, engineResult)
}

func (s *Service) persistEngineResult(resolved CaptureRequest, outputBaseName string, outputGenerated bool, engineResult EngineResult) (CaptureResult, error) {
	if engineResult.Tiling == nil {
		if err := writeFile(resolved.OutputPath, engineResult.Artifact.Bytes); err != nil {
			return CaptureResult{}, wrapCaptureError("write_image", err)
		}
	} else if err := persistTiles(resolved, engineResult.Tiling); err != nil {
		return CaptureResult{}, wrapCaptureError("write_image", err)
	}
	tileOptions := effectiveTileOptions(resolved.Tile)
	if engineResult.Tiling != nil && tileOptions.Stitch && engineResult.Tiling.Status == CaptureTilingComplete {
		if err := stitchTiles(resolved, engineResult.Tiling); err != nil {
			return CaptureResult{}, err
		}
	}

	result := CaptureResult{
		OutputPath:     resolved.OutputPath,
		MetadataPath:   resolved.MetadataPath,
		ByteSize:       captureByteSize(engineResult),
		Artifact:       engineResult.Artifact,
		CapturedAt:     engineResult.Timing.CapturedAt,
		Engine:         s.engine.Name(),
		Browser:        engineResult.Browser,
		Timing:         engineResult.Timing,
		Warnings:       cloneWarnings(engineResult.Warnings),
		Guards:         cloneGuardOutcomes(engineResult.Guards),
		Tiling:         engineResult.Tiling,
		Normalization:  resolved.Normalization(outputGenerated, outputBaseName, filepath.Dir(resolved.OutputPath)),
		ResolvedConfig: resolved.Redacted(),
	}
	if result.Tiling != nil {
		result.Tiling.MetadataPath = resolved.MetadataPath
		result.Warnings = append(result.Warnings, result.Tiling.Warnings...)
		if result.Tiling.StitchedPath != "" {
			result.OutputPath = result.Tiling.StitchedPath
			if info, statErr := os.Stat(result.OutputPath); statErr == nil {
				result.ByteSize = int(info.Size())
			}
		}
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

func captureByteSize(engineResult EngineResult) int {
	if engineResult.Tiling == nil {
		return len(engineResult.Artifact.Bytes)
	}
	total := 0
	for _, tile := range engineResult.Tiling.Tiles {
		if tile.Status == CaptureTileCompleted {
			total += tile.ByteSize
		}
	}
	return total
}

func persistTiles(req CaptureRequest, tiling *CaptureTiling) error {
	if tiling == nil {
		return nil
	}
	for idx := range tiling.Tiles {
		if tiling.Tiles[idx].Status != CaptureTileCompleted {
			continue
		}
		path := tileOutputPath(req.OutputPath, tiling.Tiles[idx].Index)
		if err := writeFile(path, tiling.Tiles[idx].Bytes); err != nil {
			return err
		}
		tiling.Tiles[idx].OutputPath = path
		tiling.Tiles[idx].ByteSize = len(tiling.Tiles[idx].Bytes)
		tiling.Tiles[idx].Bytes = nil
	}
	tiling.CompletedCount = 0
	tiling.FailedCount = 0
	for _, tile := range tiling.Tiles {
		switch tile.Status {
		case CaptureTileCompleted:
			tiling.CompletedCount++
		case CaptureTileFailed:
			tiling.FailedCount++
		}
	}
	tiling.TileCount = len(tiling.Tiles)
	if tiling.FailedCount > 0 {
		tiling.Status = CaptureTilingPartial
	}
	return nil
}

func tileOutputPath(outputPath string, index int) string {
	ext := filepath.Ext(outputPath)
	if ext == "" {
		ext = "." + defaultImageFormat
	}
	base := outputPath[:len(outputPath)-len(filepath.Ext(outputPath))]
	return base + ".tile-" + leftPadInt(index, 4) + ext
}

func leftPadInt(value, width int) string {
	raw := strconv.Itoa(value)
	for len(raw) < width {
		raw = "0" + raw
	}
	return raw
}

func stitchTiles(req CaptureRequest, tiling *CaptureTiling) error {
	if tiling == nil {
		return nil
	}
	targetPixels := scaledPixels(tiling.TargetBounds.Width, tiling.TargetBounds.Height, tiling.Limits.ScaleFactor)
	tileOptions := effectiveTileOptions(req.Tile)
	if targetPixels > tileOptions.MaxStitchedPixels {
		return newCaptureError(CodeOversize, "stitch_tiles", "stitched output exceeds max_stitched_pixels", nil).
			WithMetadata("target_bounds", tiling.TargetBounds).
			WithMetadata("max_stitched_pixels", tileOptions.MaxStitchedPixels)
	}
	scale := tiling.Limits.ScaleFactor
	if scale <= 0 {
		scale = defaultScaleFactor
	}
	dst := image.NewRGBA(image.Rect(0, 0, int(tiling.TargetBounds.Width*scale), int(tiling.TargetBounds.Height*scale)))
	for _, tile := range tiling.Tiles {
		if tile.Status != CaptureTileCompleted || tile.OutputPath == "" {
			continue
		}
		payload, err := os.ReadFile(tile.OutputPath)
		if err != nil {
			return wrapCaptureError("stitch_read_tile", err)
		}
		img, err := png.Decode(bytes.NewReader(payload))
		if err != nil {
			return wrapCaptureError("stitch_decode_tile", err)
		}
		dest := tile.SourceBounds
		if tile.DestinationBounds != nil {
			dest = *tile.DestinationBounds
		}
		destRect := image.Rect(int(dest.X*scale), int(dest.Y*scale), int((dest.X+dest.Width)*scale), int((dest.Y+dest.Height)*scale))
		sourcePoint := image.Point{}
		if tile.DestinationBounds != nil {
			sourcePoint = image.Point{
				X: int((dest.X - (tile.SourceBounds.X - tiling.TargetBounds.X)) * scale),
				Y: int((dest.Y - (tile.SourceBounds.Y - tiling.TargetBounds.Y)) * scale),
			}
		}
		draw.Draw(dst, destRect, img, sourcePoint, draw.Src)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return wrapCaptureError("stitch_encode", err)
	}
	if err := writeFile(req.OutputPath, out.Bytes()); err != nil {
		return err
	}
	tiling.StitchedPath = req.OutputPath
	if req.Tile.CleanupTiles {
		for _, tile := range tiling.Tiles {
			if tile.OutputPath != "" {
				_ = os.Remove(tile.OutputPath)
			}
		}
	}
	return nil
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
			var partialErr *PartialCaptureError
			if errors.As(err, &partialErr) && partialErr.Result != nil {
				results = append(results, *partialErr.Result)
				return BatchResult{Results: results}, err
			}
			return BatchResult{Results: results}, err
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
