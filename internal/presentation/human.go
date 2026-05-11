package presentation

import (
	"fmt"
	"io"
	"strings"

	pkgwebcap "github.com/goliatone/webcap"
	"github.com/goliatone/webcap/pkg/agents/skills"
)

type WorkflowRunReportResult struct {
	Capture pkgwebcap.WorkflowCaptureResult `json:"capture"`
	Report  pkgwebcap.WorkflowReportResult  `json:"report"`
}

func (p Presenter) presentHuman(w io.Writer, value any) error {
	switch result := value.(type) {
	case pkgwebcap.CaptureResult:
		return writeCapture(w, result)
	case pkgwebcap.BatchResult:
		return writeBatch(w, result)
	case pkgwebcap.DiffResult:
		return writeDiff(w, result)
	case pkgwebcap.SemanticDiffResult:
		return writeSemanticDiff(w, result)
	case pkgwebcap.WorkflowCaptureResult:
		return writeWorkflowCapture(w, result)
	case pkgwebcap.WorkflowReportResult:
		return writeWorkflowReport(w, result)
	case WorkflowRunReportResult:
		return writeWorkflowRunReport(w, result)
	case skills.InstallResult:
		return writeSkillInstall(w, result)
	default:
		return writeJSON(w, value)
	}
}

func writeLines(w io.Writer, lines ...string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func writeWarnings(w io.Writer, warnings []pkgwebcap.CaptureWarning) error {
	if len(warnings) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "Warnings:"); err != nil {
		return err
	}
	for _, warning := range warnings {
		message := strings.TrimSpace(warning.Message)
		if warning.Code != "" {
			message = warning.Code + ": " + message
		}
		if _, err := fmt.Fprintf(w, "  - %s\n", message); err != nil {
			return err
		}
	}
	return nil
}
