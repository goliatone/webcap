package webcap

import (
	"context"
	"fmt"

	command "github.com/goliatone/go-command"
	pkgwebcap "github.com/goliatone/webcap"
)

type CaptureScenarioMessage struct {
	Scenario pkgwebcap.WorkflowScenario
}

func (CaptureScenarioMessage) Type() string { return "webcap::capture_scenario" }

type CaptureScenarioHandler struct {
	Service pkgwebcap.WorkflowService
}

func NewCaptureScenarioHandler(service pkgwebcap.WorkflowService) *CaptureScenarioHandler {
	return &CaptureScenarioHandler{Service: service}
}

func (h *CaptureScenarioHandler) Execute(ctx context.Context, msg CaptureScenarioMessage) error {
	_, err := h.Handle(ctx, msg)
	return err
}

func (h *CaptureScenarioHandler) Handle(ctx context.Context, msg CaptureScenarioMessage) (pkgwebcap.WorkflowCaptureResult, error) {
	if h == nil || h.Service == nil {
		return pkgwebcap.WorkflowCaptureResult{}, fmt.Errorf("webcap workflow service is not configured")
	}
	return h.Service.CaptureScenario(ctx, msg.Scenario)
}

var _ command.Commander[CaptureScenarioMessage] = (*CaptureScenarioHandler)(nil)

type WorkflowReportMessage struct {
	Request pkgwebcap.WorkflowReportRequest
}

func (WorkflowReportMessage) Type() string { return "webcap::workflow_report" }

type WorkflowReportHandler struct {
	Service pkgwebcap.WorkflowService
}

func NewWorkflowReportHandler(service pkgwebcap.WorkflowService) *WorkflowReportHandler {
	return &WorkflowReportHandler{Service: service}
}

func (h *WorkflowReportHandler) Execute(ctx context.Context, msg WorkflowReportMessage) error {
	_, err := h.Handle(ctx, msg)
	return err
}

func (h *WorkflowReportHandler) Handle(ctx context.Context, msg WorkflowReportMessage) (pkgwebcap.WorkflowReportResult, error) {
	if h == nil || h.Service == nil {
		return pkgwebcap.WorkflowReportResult{}, fmt.Errorf("webcap workflow service is not configured")
	}
	return h.Service.GenerateWorkflowReport(ctx, msg.Request)
}

var _ command.Commander[WorkflowReportMessage] = (*WorkflowReportHandler)(nil)
