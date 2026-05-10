package webcap

import (
	"context"

	command "github.com/goliatone/go-command"
	pkgwebcap "github.com/goliatone/webcap"
)

type SemanticDiffMessage struct {
	Request pkgwebcap.SemanticDiffRequest
}

func (SemanticDiffMessage) Type() string { return "webcap::semantic_diff" }

type SemanticDiffHandler struct {
	*commandHandler[SemanticDiffMessage, pkgwebcap.SemanticDiffResult]
}

func NewSemanticDiffHandler(service pkgwebcap.SemanticDiffService) *SemanticDiffHandler {
	var run func(context.Context, SemanticDiffMessage) (pkgwebcap.SemanticDiffResult, error)
	if service != nil {
		run = func(ctx context.Context, msg SemanticDiffMessage) (pkgwebcap.SemanticDiffResult, error) {
			return service.SemanticDiff(ctx, msg.Request)
		}
	}
	return &SemanticDiffHandler{commandHandler: newCommandHandler("semantic diff", run)}
}

var _ command.Commander[SemanticDiffMessage] = (*SemanticDiffHandler)(nil)
