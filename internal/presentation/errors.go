package presentation

import (
	"errors"
	"fmt"
	"io"
	"strings"

	pkgwebcap "github.com/goliatone/webcap"
	"github.com/goliatone/webcap/pkg/agents/skills"
	"github.com/goliatone/webcap/pkg/llms"
)

type ErrorEnvelope struct {
	Message   string         `json:"message"`
	Code      string         `json:"code,omitempty"`
	Operation string         `json:"operation,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func ErrorEnvelopeFrom(err error) ErrorEnvelope {
	envelope := ErrorEnvelope{Message: strings.TrimSpace(fmt.Sprint(err))}
	if envelope.Message == "" {
		envelope.Message = "unknown error"
	}

	var partialErr *pkgwebcap.PartialCaptureError
	if errors.As(err, &partialErr) {
		envelope.Code = string(pkgwebcap.CodePartialCapture)
		envelope.Operation = partialErr.Operation
		envelope.Metadata = map[string]any{
			"failed_tile_index": partialErr.FailedTileIndex,
			"completed_count":   partialErr.CompletedCount,
			"total_count":       partialErr.TotalCount,
		}
		if partialErr.Result != nil {
			envelope.Metadata["result"] = partialErr.Result
		}
		return envelope
	}

	var captureErr *pkgwebcap.Error
	if errors.As(err, &captureErr) {
		envelope.Message = captureErr.Message
		envelope.Code = string(captureErr.Code)
		envelope.Operation = captureErr.Operation
		envelope.Metadata = captureErr.Metadata
		return envelope
	}

	var conflict skills.ConflictError
	if errors.As(err, &conflict) {
		envelope.Code = "skill_conflict"
		envelope.Operation = "skill_install"
		envelope.Metadata = map[string]any{"path": conflict.Path}
		return envelope
	}

	var execErr *llms.ExecutionError
	if errors.As(err, &execErr) {
		envelope.Code = "provider_execution_error"
		envelope.Operation = strings.TrimSpace(execErr.Provider)
		envelope.Metadata = map[string]any{
			"command":   execErr.Command,
			"exit_code": execErr.ExitCode,
			"timed_out": execErr.TimedOut,
			"cancelled": execErr.Cancelled,
		}
		return envelope
	}

	return envelope
}

func writeHumanError(w io.Writer, err error) error {
	var partialErr *pkgwebcap.PartialCaptureError
	if errors.As(err, &partialErr) {
		return writePartialCaptureError(w, err, partialErr)
	}
	_, writeErr := fmt.Fprintf(w, "Error: %s\n", strings.TrimSpace(fmt.Sprint(err)))
	return writeErr
}

func writePartialCaptureError(w io.Writer, err error, partialErr *pkgwebcap.PartialCaptureError) error {
	if _, writeErr := fmt.Fprintf(w, "Error: %s\n", strings.TrimSpace(fmt.Sprint(err))); writeErr != nil {
		return writeErr
	}
	if _, writeErr := fmt.Fprintf(w, "Failed tile: %d\n", partialErr.FailedTileIndex); writeErr != nil {
		return writeErr
	}
	if _, writeErr := fmt.Fprintf(w, "Tiles: %d/%d completed\n", partialErr.CompletedCount, partialErr.TotalCount); writeErr != nil {
		return writeErr
	}
	if partialErr.Result == nil {
		return nil
	}
	return writePartialCaptureResult(w, *partialErr.Result)
}

func writePartialCaptureResult(w io.Writer, result pkgwebcap.CaptureResult) error {
	if result.MetadataPath != "" {
		if _, writeErr := fmt.Fprintf(w, "Metadata: %s\n", result.MetadataPath); writeErr != nil {
			return writeErr
		}
	}
	return writePartialCaptureTiling(w, result.Tiling)
}

func writePartialCaptureTiling(w io.Writer, tiling *pkgwebcap.CaptureTiling) error {
	if tiling == nil {
		return nil
	}
	if tiling.Status != "" {
		if _, writeErr := fmt.Fprintf(w, "Tiling: %s\n", tiling.Status); writeErr != nil {
			return writeErr
		}
	}
	return writeFirstCompletedTile(w, tiling.Tiles)
}

func writeFirstCompletedTile(w io.Writer, tiles []pkgwebcap.CaptureTile) error {
	for _, tile := range tiles {
		if tile.Status != pkgwebcap.CaptureTileCompleted || tile.OutputPath == "" {
			continue
		}
		_, writeErr := fmt.Fprintf(w, "First tile: %s\n", tile.OutputPath)
		return writeErr
	}
	return nil
}
