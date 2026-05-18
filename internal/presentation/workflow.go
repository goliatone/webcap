package presentation

import (
	"fmt"
	"io"

	pkgwebcap "github.com/goliatone/webcap"
)

func writeWorkflowCapture(w io.Writer, result pkgwebcap.WorkflowCaptureResult) error {
	if err := writeLines(w,
		"Workflow capture complete",
		fmt.Sprintf("Scenario: %s", result.ScenarioID),
		fmt.Sprintf("Scenario path: %s", result.ScenarioPath),
		fmt.Sprintf("Current dir: %s", result.CurrentDir),
		fmt.Sprintf("Captures: %d", len(result.Results)),
	); err != nil {
		return err
	}
	return writeWorkflowScreens(w, result.Results)
}

func writeWorkflowReport(w io.Writer, result pkgwebcap.WorkflowReportResult) error {
	if err := writeLines(w,
		"Workflow report complete",
		fmt.Sprintf("Scenario: %s", result.ScenarioID),
		fmt.Sprintf("Status: %s", firstNonEmpty(result.Status.Label, result.Status.Level)),
		fmt.Sprintf("Report: %s", result.ReportPath),
		fmt.Sprintf("Metadata: %s", result.MetadataPath),
		fmt.Sprintf("Diff dir: %s", result.DiffDir),
		fmt.Sprintf("Stories: %d", len(result.Stories)),
		fmt.Sprintf("Entries: %d", len(result.Entries)),
		fmt.Sprintf("Needs attention: %d", workflowAttentionCount(result.Entries)),
	); err != nil {
		return err
	}
	return nil
}

func writeWorkflowRunReport(w io.Writer, result WorkflowRunReportResult) error {
	if err := writeLines(w,
		"Workflow capture and report complete",
		fmt.Sprintf("Scenario: %s", result.Capture.ScenarioID),
		fmt.Sprintf("Captures: %d", len(result.Capture.Results)),
		fmt.Sprintf("Status: %s", firstNonEmpty(result.Report.Status.Label, result.Report.Status.Level)),
		fmt.Sprintf("Report: %s", result.Report.ReportPath),
		fmt.Sprintf("Metadata: %s", result.Report.MetadataPath),
		fmt.Sprintf("Needs attention: %d", workflowAttentionCount(result.Report.Entries)),
	); err != nil {
		return err
	}
	return nil
}

func workflowAttentionCount(entries []pkgwebcap.WorkflowReportEntry) int {
	count := 0
	for _, entry := range entries {
		switch entry.Status.Level {
		case "", "success", "pass", "passed", "ok":
			continue
		default:
			count++
		}
	}
	return count
}

func writeWorkflowScreens(w io.Writer, screens []pkgwebcap.WorkflowScreenCaptureResult) error {
	if len(screens) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "Screens:"); err != nil {
		return err
	}
	for _, screen := range screens {
		if err := writeWorkflowScreen(w, screen); err != nil {
			return err
		}
	}
	return nil
}

func writeWorkflowScreen(w io.Writer, screen pkgwebcap.WorkflowScreenCaptureResult) error {
	if _, err := fmt.Fprintf(w, "  - %s %s -> %s", screen.ScreenID, screen.Label, screen.OutputPath); err != nil {
		return err
	}
	if err := writeWorkflowScreenMetadata(w, screen); err != nil {
		return err
	}
	if err := writeWorkflowScreenWarnings(w, screen); err != nil {
		return err
	}
	if err := writeWorkflowScreenTiling(w, screen.Capture.Tiling); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w)
	return err
}

func writeWorkflowScreenMetadata(w io.Writer, screen pkgwebcap.WorkflowScreenCaptureResult) error {
	metadataPath := firstNonEmpty(screen.MetadataPath, screen.Capture.MetadataPath)
	if metadataPath == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, " metadata=%s", metadataPath)
	return err
}

func writeWorkflowScreenWarnings(w io.Writer, screen pkgwebcap.WorkflowScreenCaptureResult) error {
	if len(screen.Capture.Warnings) == 0 {
		return nil
	}
	_, err := fmt.Fprintf(w, " (%d warnings)", len(screen.Capture.Warnings))
	return err
}

func writeWorkflowScreenTiling(w io.Writer, tiling *pkgwebcap.CaptureTiling) error {
	if tiling == nil {
		return nil
	}
	_, err := fmt.Fprintf(w, " tiled=%s tiles=%d/%d", tiling.Status, tiling.CompletedCount, tiling.TileCount)
	return err
}
