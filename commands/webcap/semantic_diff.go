package webcap

import (
	"context"
	"fmt"

	command "github.com/goliatone/go-command"
	pkgwebcap "github.com/goliatone/webcap"
)

type SemanticDiffMessage struct {
	Request pkgwebcap.SemanticDiffRequest
}

func (SemanticDiffMessage) Type() string { return "webcap::semantic_diff" }

type SemanticDiffHandler struct {
	Service pkgwebcap.SemanticDiffService
}

func NewSemanticDiffHandler(service pkgwebcap.SemanticDiffService) *SemanticDiffHandler {
	return &SemanticDiffHandler{Service: service}
}

func (h *SemanticDiffHandler) Execute(ctx context.Context, msg SemanticDiffMessage) error {
	_, err := h.Handle(ctx, msg)
	return err
}

func (h *SemanticDiffHandler) Handle(ctx context.Context, msg SemanticDiffMessage) (pkgwebcap.SemanticDiffResult, error) {
	if h == nil || h.Service == nil {
		return pkgwebcap.SemanticDiffResult{}, fmt.Errorf("webcap semantic diff service is not configured")
	}
	return h.Service.SemanticDiff(ctx, msg.Request)
}

var _ command.Commander[SemanticDiffMessage] = (*SemanticDiffHandler)(nil)
