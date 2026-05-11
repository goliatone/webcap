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
	_, writeErr := fmt.Fprintf(w, "Error: %s\n", strings.TrimSpace(fmt.Sprint(err)))
	return writeErr
}
