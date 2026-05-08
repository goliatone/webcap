package webcap

import (
	"context"
	"fmt"

	command "github.com/goliatone/go-command"
	pkgwebcap "github.com/goliatone/webcap"
)

type CaptureShotMessage struct {
	Request pkgwebcap.CaptureRequest
}

func (CaptureShotMessage) Type() string { return "webcap::capture_shot" }

type CaptureShotHandler struct {
	Service pkgwebcap.CaptureService
}

func NewCaptureShotHandler(service pkgwebcap.CaptureService) *CaptureShotHandler {
	return &CaptureShotHandler{Service: service}
}

func (h *CaptureShotHandler) Execute(ctx context.Context, msg CaptureShotMessage) error {
	_, err := h.Handle(ctx, msg)
	return err
}

func (h *CaptureShotHandler) Handle(ctx context.Context, msg CaptureShotMessage) (pkgwebcap.CaptureResult, error) {
	if h == nil || h.Service == nil {
		return pkgwebcap.CaptureResult{}, fmt.Errorf("webcap capture service is not configured")
	}
	return h.Service.Capture(ctx, msg.Request)
}

var _ command.Commander[CaptureShotMessage] = (*CaptureShotHandler)(nil)

type CaptureBatchMessage struct {
	Manifest  pkgwebcap.Manifest
	OutputDir string
}

func (CaptureBatchMessage) Type() string { return "webcap::capture_batch" }

type CaptureBatchHandler struct {
	Service pkgwebcap.CaptureService
}

func NewCaptureBatchHandler(service pkgwebcap.CaptureService) *CaptureBatchHandler {
	return &CaptureBatchHandler{Service: service}
}

func (h *CaptureBatchHandler) Execute(ctx context.Context, msg CaptureBatchMessage) error {
	_, err := h.Handle(ctx, msg)
	return err
}

func (h *CaptureBatchHandler) Handle(ctx context.Context, msg CaptureBatchMessage) (pkgwebcap.BatchResult, error) {
	if h == nil || h.Service == nil {
		return pkgwebcap.BatchResult{}, fmt.Errorf("webcap capture service is not configured")
	}
	return h.Service.CaptureBatch(ctx, msg.Manifest, msg.OutputDir)
}

var _ command.Commander[CaptureBatchMessage] = (*CaptureBatchHandler)(nil)
