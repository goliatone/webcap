package webcap

import (
	"encoding/json"
	"strings"
)

type semanticParsePayload struct {
	Summary     string               `json:"summary"`
	Verdict     SemanticDiffVerdict  `json:"verdict"`
	Severity    SemanticDiffSeverity `json:"severity"`
	Differences []SemanticDifference `json:"differences"`
}

func parseSemanticProviderResponse(raw string) (semanticParsePayload, []CaptureWarning) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return semanticParsePayload{
			Summary:  "Provider returned an empty semantic diff response.",
			Verdict:  SemanticDiffVerdictInconclusive,
			Severity: SemanticDiffSeverityInfo,
		}, []CaptureWarning{{Code: string(CodeCapture), Message: "semantic provider response was empty"}}
	}

	jsonText := extractSemanticJSON(raw)
	var payload semanticParsePayload
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return semanticParsePayload{
			Summary:  raw,
			Verdict:  SemanticDiffVerdictInconclusive,
			Severity: SemanticDiffSeverityInfo,
		}, []CaptureWarning{{Code: string(CodeCapture), Message: "semantic provider response was not parseable JSON"}}
	}

	warnings := []CaptureWarning{}
	payload.Summary = strings.TrimSpace(payload.Summary)
	if payload.Summary == "" {
		payload.Summary = "Semantic provider did not include a summary."
		warnings = append(warnings, CaptureWarning{Code: string(CodeCapture), Message: "semantic provider response omitted summary"})
	}
	if !isValidSemanticVerdict(payload.Verdict) {
		payload.Verdict = SemanticDiffVerdictInconclusive
		warnings = append(warnings, CaptureWarning{Code: string(CodeCapture), Message: "semantic provider response used an unknown verdict"})
	}
	if !isValidSemanticSeverity(payload.Severity) {
		payload.Severity = SemanticDiffSeverityInfo
		warnings = append(warnings, CaptureWarning{Code: string(CodeCapture), Message: "semantic provider response used an unknown severity"})
	}
	for i := range payload.Differences {
		payload.Differences[i].Description = strings.TrimSpace(payload.Differences[i].Description)
		if payload.Differences[i].Description == "" {
			payload.Differences[i].Description = "Provider reported a difference without a description."
		}
		if !isValidSemanticSeverity(payload.Differences[i].Severity) {
			payload.Differences[i].Severity = payload.Severity
			warnings = append(warnings, CaptureWarning{Code: string(CodeCapture), Message: "semantic provider response used an unknown difference severity"})
		}
	}
	return payload, warnings
}

func extractSemanticJSON(raw string) string {
	text := strings.TrimSpace(raw)
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 {
			lines = lines[1 : len(lines)-1]
			text = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}

func isValidSemanticVerdict(value SemanticDiffVerdict) bool {
	switch value {
	case SemanticDiffVerdictNoMeaningfulChange, SemanticDiffVerdictNeedsReview, SemanticDiffVerdictRegression, SemanticDiffVerdictImprovement, SemanticDiffVerdictInconclusive:
		return true
	default:
		return false
	}
}

func isValidSemanticSeverity(value SemanticDiffSeverity) bool {
	switch value {
	case SemanticDiffSeverityInfo, SemanticDiffSeverityMinor, SemanticDiffSeverityMajor, SemanticDiffSeverityBlocking:
		return true
	default:
		return false
	}
}
