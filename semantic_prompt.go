package webcap

import (
	"fmt"
	"os"
	"strings"
)

func buildSemanticDiffPrompt(req SemanticDiffRequest) (string, SemanticPromptMetadata, error) {
	promptSource := "default"
	parts := []string{
		"You are reviewing two UI screenshots. The current screenshot is the candidate output and the reference screenshot is the expected baseline.",
		"Describe meaningful user-visible differences. Ignore tiny anti-aliasing, sub-pixel, compression, and rendering noise unless it changes meaning or usability.",
	}

	switch req.Mode {
	case SemanticDiffModeGeneral:
		parts = append(parts, "Mode: general comparison. Review layout, copy, visibility, state, hierarchy, and obvious accessibility issues.")
	case SemanticDiffModeFocused:
		parts = append(parts, "Mode: focused comparison. Prioritize the caller-provided focus areas.")
	case SemanticDiffModeCopy:
		parts = append(parts, "Mode: copy comparison. Prioritize changed labels, headings, button text, validation messages, prices, counts, and other visible text.")
	case SemanticDiffModeLayout:
		parts = append(parts, "Mode: layout comparison. Prioritize spacing, alignment, responsive layout, visibility, clipping, stacking, and relative visual hierarchy.")
	case SemanticDiffModeAccessibility:
		parts = append(parts, "Mode: accessibility comparison. Prioritize contrast, readable text, focus or disabled states, semantic affordances, and visibly missing labels.")
	case SemanticDiffModeCustom:
		parts = append(parts, "Mode: custom comparison. Follow the caller instructions.")
	}

	if len(req.Focus) > 0 {
		focus := make([]string, 0, len(req.Focus))
		for _, item := range req.Focus {
			focus = append(focus, "- "+item)
		}
		parts = append(parts, "Focus areas:\n"+strings.Join(focus, "\n"))
		promptSource = "focus"
	}
	if req.PromptPath != "" {
		payload, err := os.ReadFile(req.PromptPath)
		if err != nil {
			return "", SemanticPromptMetadata{}, wrapCaptureError("read_semantic_prompt_file", err)
		}
		text := strings.TrimSpace(string(payload))
		if text != "" {
			parts = append(parts, "Caller prompt file instructions:\n"+text)
			promptSource = "prompt_file"
		}
	}
	if req.Prompt != "" {
		parts = append(parts, "Caller prompt instructions:\n"+req.Prompt)
		promptSource = "prompt"
	}
	if hasSemanticPixelContext(req.PixelContext) {
		parts = append(parts, formatSemanticPixelContext(req.PixelContext))
	}

	parts = append(parts, `Return only JSON with this shape:
{
  "summary": "short human-readable summary",
  "verdict": "no_meaningful_change | needs_review | regression | improvement | inconclusive",
  "severity": "info | minor | major | blocking",
  "differences": [
    {
      "area": "screen area or component",
      "description": "what changed and why it matters",
      "severity": "info | minor | major | blocking",
      "evidence": "visible evidence from the screenshots",
      "recommendation": "optional suggested action"
    }
  ]
}`)

	return strings.Join(parts, "\n\n"), SemanticPromptMetadata{
		Mode:                 req.Mode,
		Focus:                append([]string(nil), req.Focus...),
		PromptSource:         promptSource,
		StructuredOutput:     true,
		PixelContextIncluded: hasSemanticPixelContext(req.PixelContext),
	}, nil
}

func hasSemanticPixelContext(value SemanticPixelContext) bool {
	return strings.TrimSpace(value.PixelDiffImagePath) != "" ||
		value.ChangedPixels > 0 ||
		value.TotalPixels > 0 ||
		value.ChangedPercent > 0 ||
		len(value.Warnings) > 0
}

func formatSemanticPixelContext(value SemanticPixelContext) string {
	lines := []string{"Pixel diff context supplied by the caller:"}
	if value.PixelDiffImagePath != "" {
		lines = append(lines, fmt.Sprintf("- diff image path: %s", value.PixelDiffImagePath))
	}
	if value.TotalPixels > 0 {
		lines = append(lines, fmt.Sprintf("- changed pixels: %d of %d (%.4f%%)", value.ChangedPixels, value.TotalPixels, value.ChangedPercent))
	} else if value.ChangedPixels > 0 || value.ChangedPercent > 0 {
		lines = append(lines, fmt.Sprintf("- changed pixels: %d (%.4f%%)", value.ChangedPixels, value.ChangedPercent))
	}
	if value.Threshold > 0 {
		lines = append(lines, fmt.Sprintf("- pixel threshold: %.4f", value.Threshold))
	}
	for _, warning := range value.Warnings {
		lines = append(lines, fmt.Sprintf("- pixel warning: %s", strings.TrimSpace(warning.Message)))
	}
	return strings.Join(lines, "\n")
}
