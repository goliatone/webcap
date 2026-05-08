package webcap

import (
	"context"
	"fmt"

	command "github.com/goliatone/go-command"
	pkgwebcap "github.com/goliatone/webcap"
)

type DiffMessage struct {
	Request pkgwebcap.DiffRequest
}

func (DiffMessage) Type() string { return "webcap::diff" }

type DiffHandler struct {
	Service pkgwebcap.DiffService
}

func NewDiffHandler(service pkgwebcap.DiffService) *DiffHandler {
	return &DiffHandler{Service: service}
}

func (h *DiffHandler) Execute(ctx context.Context, msg DiffMessage) error {
	_, err := h.Handle(ctx, msg)
	return err
}

func (h *DiffHandler) Handle(ctx context.Context, msg DiffMessage) (pkgwebcap.DiffResult, error) {
	if h == nil || h.Service == nil {
		return pkgwebcap.DiffResult{}, fmt.Errorf("webcap diff service is not configured")
	}
	return h.Service.Diff(ctx, msg.Request)
}

var _ command.Commander[DiffMessage] = (*DiffHandler)(nil)
