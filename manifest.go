package webcap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadManifest(path string) (Manifest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Manifest{}, newCaptureError(CodeManifest, "load_manifest", "manifest path is required", nil)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, wrapCaptureError("load_manifest", err)
	}

	var manifest Manifest
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		err = json.Unmarshal(payload, &manifest)
	default:
		err = yaml.Unmarshal(payload, &manifest)
	}
	if err != nil {
		return Manifest{}, newCaptureError(CodeManifest, "load_manifest", "invalid manifest", err)
	}
	if len(manifest.Shots) == 0 {
		return Manifest{}, newCaptureError(CodeManifest, "load_manifest", "manifest must define at least one shot", nil)
	}
	return manifest, nil
}

func (m Manifest) Requests(outputDirOverride string) ([]CaptureRequest, error) {
	outputDir := strings.TrimSpace(outputDirOverride)
	if outputDir == "" {
		outputDir = strings.TrimSpace(m.OutputDir)
	}

	requests := make([]CaptureRequest, 0, len(m.Shots))
	for idx, shot := range m.Shots {
		req := CaptureRequest{
			URL:               shot.URL,
			OutputPath:        resolveShotOutput(outputDir, shot),
			MetadataPath:      resolveShotMetadata(outputDir, shot),
			FullPage:          shot.FullPage,
			Selector:          shot.Selector,
			Selectors:         shot.Selectors,
			SelectorAll:       shot.SelectorAll,
			SelectorsAll:      shot.SelectorsAll,
			Padding:           shot.Padding,
			Wait:              firstNonEmpty(shot.Wait, m.Wait),
			WaitFor:           firstNonEmpty(shot.WaitFor, m.WaitFor),
			JavaScript:        shot.JavaScript,
			ViewportPreset:    firstNonEmpty(shot.ViewportPreset, m.ViewportPreset),
			DevicePreset:      firstNonEmpty(shot.DevicePreset, m.DevicePreset),
			Readiness:         firstReadiness(shot.Readiness, m.Readiness),
			ReadinessIdle:     firstNonEmpty(shot.ReadinessIdle, m.ReadinessIdle),
			DisableAnimations: shot.DisableAnimations || m.DisableAnimations,
			ReducedMotion:     shot.ReducedMotion || m.ReducedMotion,
			WaitForFonts:      shot.WaitForFonts || m.WaitForFonts,
			Timeout:           firstNonEmpty(shot.Timeout, m.Timeout),
			OversizePolicy:    firstOversizePolicy(shot.OversizePolicy, m.OversizePolicy),
			Tile:              mergeTileOptions(m.Tile, shot.Tile),
		}
		if shot.Viewport != nil {
			req.Viewport = *shot.Viewport
		} else if m.Viewport != nil {
			req.Viewport = *m.Viewport
		}

		normalized, err := NormalizeCaptureRequest(req)
		if err != nil {
			return nil, newCaptureError(CodeManifest, "manifest_requests", fmt.Sprintf("shot %d is invalid", idx+1), err)
		}
		resolved, _, _, err := ResolvePersistedPaths(normalized, outputDir)
		if err != nil {
			return nil, newCaptureError(CodeManifest, "manifest_requests", fmt.Sprintf("shot %d output resolution failed", idx+1), err)
		}
		requests = append(requests, resolved)
	}
	return requests, nil
}

func firstOversizePolicy(values ...OversizePolicy) OversizePolicy {
	for _, value := range values {
		if strings.TrimSpace(string(value)) != "" {
			return value
		}
	}
	return ""
}

func mergeTileOptions(base, override CaptureTileOptions) CaptureTileOptions {
	out := base
	if override.fieldSet("max_width") || override.MaxWidth != 0 {
		out.MaxWidth = override.MaxWidth
	}
	if override.fieldSet("max_height") || override.MaxHeight != 0 {
		out.MaxHeight = override.MaxHeight
	}
	if override.fieldSet("max_pixels") || override.MaxPixels != 0 {
		out.MaxPixels = override.MaxPixels
	}
	if override.fieldSet("max_target_pixels") || override.MaxTargetPixels != 0 {
		out.MaxTargetPixels = override.MaxTargetPixels
	}
	if override.fieldSet("overlap") || override.Overlap != 0 {
		out.Overlap = override.Overlap
	}
	if override.fieldSet("stitch") || override.Stitch {
		out.Stitch = override.Stitch
	}
	if override.fieldSet("max_stitched_pixels") || override.MaxStitchedPixels != 0 {
		out.MaxStitchedPixels = override.MaxStitchedPixels
	}
	if override.fieldSet("cleanup_tiles") || override.CleanupTiles {
		out.CleanupTiles = override.CleanupTiles
	}
	return out
}

func resolveShotOutput(outputDir string, shot ManifestShot) string {
	if value := strings.TrimSpace(shot.Output); value != "" {
		if outputDir == "" || filepath.IsAbs(value) {
			return value
		}
		return filepath.Join(outputDir, value)
	}
	return ""
}

func resolveShotMetadata(outputDir string, shot ManifestShot) string {
	if value := strings.TrimSpace(shot.Metadata); value != "" {
		if outputDir == "" || filepath.IsAbs(value) {
			return value
		}
		return filepath.Join(outputDir, value)
	}
	return ""
}

func firstReadiness(values ...ReadinessMode) ReadinessMode {
	for _, value := range values {
		if strings.TrimSpace(string(value)) != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
