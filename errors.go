package webcap

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type ErrorCode string

const (
	CodeValidation       ErrorCode = "validation_error"
	CodeManifest         ErrorCode = "manifest_error"
	CodeBrowserStartup   ErrorCode = "browser_startup_error"
	CodeNavigation       ErrorCode = "navigation_error"
	CodeSelectorNotFound ErrorCode = "selector_not_found"
	CodeNoVisibleMatches ErrorCode = "no_visible_matches"
	CodeTimeout          ErrorCode = "timeout_error"
	CodeWrite            ErrorCode = "write_error"
	CodeCapture          ErrorCode = "capture_error"
)

type Error struct {
	Code      ErrorCode      `json:"code"`
	Operation string         `json:"operation,omitempty"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Err       error          `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *Error) WithMetadata(key string, value any) *Error {
	if e == nil {
		return nil
	}
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
	e.Metadata[key] = value
	return e
}

func newCaptureError(code ErrorCode, operation, message string, err error) *Error {
	message = strings.TrimSpace(message)
	if message == "" && err != nil {
		message = err.Error()
	}
	return &Error{
		Code:      code,
		Operation: strings.TrimSpace(operation),
		Message:   message,
		Err:       err,
	}
}

func wrapCaptureError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var captureErr *Error
	if errors.As(err, &captureErr) {
		if captureErr.Operation == "" {
			captureErr.Operation = strings.TrimSpace(operation)
		}
		return captureErr
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return newCaptureError(CodeTimeout, operation, "capture timed out", err)
	}
	return classifyCaptureError(operation, err)
}

func classifyCaptureError(operation string, err error) error {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "selector not found"):
		return newCaptureError(CodeSelectorNotFound, operation, err.Error(), err)
	case strings.Contains(message, "no visible matches found"):
		return newCaptureError(CodeNoVisibleMatches, operation, err.Error(), err)
	case strings.Contains(message, "context deadline exceeded"):
		return newCaptureError(CodeTimeout, operation, "capture timed out", err)
	case strings.Contains(message, "failed to start"):
		return newCaptureError(CodeBrowserStartup, operation, err.Error(), err)
	case strings.Contains(message, "net::") || strings.Contains(message, "invalid url") || strings.Contains(message, "cannot navigate"):
		return newCaptureError(CodeNavigation, operation, err.Error(), err)
	default:
		return newCaptureError(CodeCapture, operation, err.Error(), err)
	}
}

func errorWarning(err error) CaptureWarning {
	if captureErr, ok := errors.AsType[*Error](err); ok {
		return CaptureWarning{
			Code:    string(captureErr.Code),
			Message: captureErr.Message,
		}
	}
	return CaptureWarning{
		Code:    string(CodeCapture),
		Message: fmt.Sprintf("%v", err),
	}
}
