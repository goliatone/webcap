package presentation

import (
	"fmt"
	"io"

	pkgwebcap "github.com/goliatone/webcap"
)

func writeSemanticDiff(w io.Writer, result pkgwebcap.SemanticDiffResult) error {
	if err := writeLines(w,
		"Semantic diff complete",
		fmt.Sprintf("Verdict: %s", result.Verdict),
		fmt.Sprintf("Severity: %s", result.Severity),
		fmt.Sprintf("Provider: %s", result.Provider),
		fmt.Sprintf("Model: %s", result.Model),
		fmt.Sprintf("Summary: %s", result.Summary),
	); err != nil {
		return err
	}
	if result.MetadataPath != "" {
		if _, err := fmt.Fprintf(w, "Metadata: %s\n", result.MetadataPath); err != nil {
			return err
		}
	}
	if result.RawResponsePath != "" {
		if _, err := fmt.Fprintf(w, "Raw response: %s\n", result.RawResponsePath); err != nil {
			return err
		}
	}
	if result.Usage.TotalTokens > 0 {
		if _, err := fmt.Fprintf(w, "Tokens: %d\n", result.Usage.TotalTokens); err != nil {
			return err
		}
	}
	if len(result.Differences) > 0 {
		if _, err := fmt.Fprintln(w, "Differences:"); err != nil {
			return err
		}
		for i, diff := range result.Differences {
			if i >= 5 {
				_, err := fmt.Fprintf(w, "  ... %d more\n", len(result.Differences)-i)
				return err
			}
			if _, err := fmt.Fprintf(w, "  - %s [%s]\n", diff.Description, diff.Severity); err != nil {
				return err
			}
		}
	}
	return writeWarnings(w, result.Warnings)
}
