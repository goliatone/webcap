package presentation

import (
	"fmt"
	"io"

	pkgwebcap "github.com/goliatone/webcap"
)

func writeDiff(w io.Writer, result pkgwebcap.DiffResult) error {
	status := "unchanged"
	if result.Summary.ChangedFiles > 0 || result.Summary.MissingBaseFiles > 0 || result.Summary.MissingCompareFiles > 0 {
		status = "changed"
	}
	if result.Entry != nil && result.Entry.Changed {
		status = "changed"
	}
	if err := writeLines(w,
		"Diff complete",
		fmt.Sprintf("Status: %s", status),
		fmt.Sprintf("Mode: %s", result.Mode),
		fmt.Sprintf("Output: %s", result.OutputPath),
		fmt.Sprintf("Threshold: %.4g", result.Threshold),
		fmt.Sprintf("Changed files: %d", result.Summary.ChangedFiles),
		fmt.Sprintf("Missing base files: %d", result.Summary.MissingBaseFiles),
		fmt.Sprintf("Missing compare files: %d", result.Summary.MissingCompareFiles),
		fmt.Sprintf("Changed pixels: %d", result.Summary.TotalChangedPixels),
	); err != nil {
		return err
	}
	if result.MetadataPath != "" {
		if _, err := fmt.Fprintf(w, "Metadata: %s\n", result.MetadataPath); err != nil {
			return err
		}
	}
	if len(result.Entries) > 0 {
		if _, err := fmt.Fprintln(w, "Entries:"); err != nil {
			return err
		}
		for i, entry := range result.Entries {
			if i >= 5 {
				_, err := fmt.Fprintf(w, "  ... %d more\n", len(result.Entries)-i)
				return err
			}
			if _, err := fmt.Fprintf(w, "  - %s changed=%t missing_base=%t missing_compare=%t\n", entry.RelativePath, entry.Changed, entry.MissingBase, entry.MissingCompare); err != nil {
				return err
			}
		}
	}
	return writeWarnings(w, result.Summary.Warnings)
}
