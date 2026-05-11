package webcap

import (
	"context"
	"fmt"
)

type commandHandler[M any, R any] struct {
	serviceName string
	run         func(context.Context, M) (R, error)
}

func newCommandHandler[M any, R any](serviceName string, run func(context.Context, M) (R, error)) *commandHandler[M, R] {
	return &commandHandler[M, R]{serviceName: serviceName, run: run}
}

func (h *commandHandler[M, R]) Execute(ctx context.Context, msg M) error {
	_, err := h.Handle(ctx, msg)
	return err
}

func (h *commandHandler[M, R]) Handle(ctx context.Context, msg M) (R, error) {
	if h == nil || h.run == nil {
		var zero R
		return zero, fmt.Errorf("webcap %s service is not configured", h.serviceName)
	}
	return h.run(ctx, msg)
}
