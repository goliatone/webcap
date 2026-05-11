package presentation

import (
	"fmt"
	"io"

	pkgwebcap "github.com/goliatone/webcap"
)

func writeCapture(w io.Writer, result pkgwebcap.CaptureResult) error {
	if err := writeLines(w,
		"Capture complete",
		fmt.Sprintf("Output: %s", result.OutputPath),
		fmt.Sprintf("Bytes: %d", result.ByteSize),
		fmt.Sprintf("Engine: %s", firstNonEmpty(result.Engine, result.Browser.Engine)),
		fmt.Sprintf("Viewport: %dx%d", result.Artifact.Viewport.Width, result.Artifact.Viewport.Height),
		fmt.Sprintf("Readiness: %s", result.ResolvedConfig.Readiness),
		fmt.Sprintf("Duration: %s", result.Timing.TotalDuration),
	); err != nil {
		return err
	}
	if result.MetadataPath != "" {
		if _, err := fmt.Fprintf(w, "Metadata: %s\n", result.MetadataPath); err != nil {
			return err
		}
	}
	if result.Tiling != nil {
		if _, err := fmt.Fprintf(w, "Tiling: %s (%d/%d tiles)\n", result.Tiling.Status, result.Tiling.CompletedCount, result.Tiling.TileCount); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Target: %.0fx%.0f\n", result.Tiling.TargetBounds.Width, result.Tiling.TargetBounds.Height); err != nil {
			return err
		}
		if len(result.Tiling.Tiles) > 0 && result.Tiling.Tiles[0].OutputPath != "" {
			if _, err := fmt.Fprintf(w, "First tile: %s\n", result.Tiling.Tiles[0].OutputPath); err != nil {
				return err
			}
		}
		if result.Tiling.StitchedPath != "" {
			if _, err := fmt.Fprintf(w, "Stitched: %s\n", result.Tiling.StitchedPath); err != nil {
				return err
			}
		}
	}
	if !result.CapturedAt.IsZero() {
		if _, err := fmt.Fprintf(w, "Captured: %s\n", result.CapturedAt.Format("2006-01-02T15:04:05Z07:00")); err != nil {
			return err
		}
	}
	return writeWarnings(w, result.Warnings)
}

func writeBatch(w io.Writer, result pkgwebcap.BatchResult) error {
	if err := writeLines(w,
		"Batch capture complete",
		fmt.Sprintf("Captures: %d", len(result.Results)),
	); err != nil {
		return err
	}
	for i, item := range result.Results {
		if _, err := fmt.Fprintf(w, "  %d. %s", i+1, item.OutputPath); err != nil {
			return err
		}
		if item.MetadataPath != "" {
			if _, err := fmt.Fprintf(w, " metadata=%s", item.MetadataPath); err != nil {
				return err
			}
		}
		if len(item.Warnings) > 0 {
			if _, err := fmt.Fprintf(w, " (%d warnings)", len(item.Warnings)); err != nil {
				return err
			}
		}
		if item.Tiling != nil {
			if _, err := fmt.Fprintf(w, " tiled=%s tiles=%d/%d", item.Tiling.Status, item.Tiling.CompletedCount, item.Tiling.TileCount); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}
