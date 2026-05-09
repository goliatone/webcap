package webcap

import "testing"

func TestParseSemanticProviderResponseJSONAndMarkdown(t *testing.T) {
	payload, warnings := parseSemanticProviderResponse("```json\n{\"summary\":\"No issue\",\"verdict\":\"no_meaningful_change\",\"severity\":\"info\",\"differences\":[]}\n```")
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	if payload.Summary != "No issue" || payload.Verdict != SemanticDiffVerdictNoMeaningfulChange || payload.Severity != SemanticDiffSeverityInfo {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestParseSemanticProviderResponseFallbacks(t *testing.T) {
	payload, warnings := parseSemanticProviderResponse("plain text summary")
	if len(warnings) == 0 {
		t.Fatal("expected parse warning")
	}
	if payload.Summary != "plain text summary" || payload.Verdict != SemanticDiffVerdictInconclusive || payload.Severity != SemanticDiffSeverityInfo {
		t.Fatalf("unexpected fallback payload: %#v", payload)
	}

	payload, warnings = parseSemanticProviderResponse(`{"summary":"x","verdict":"strange","severity":"huge","differences":[{"description":"bad","severity":"huge"}]}`)
	if len(warnings) < 2 {
		t.Fatalf("expected unknown value warnings, got %#v", warnings)
	}
	if payload.Verdict != SemanticDiffVerdictInconclusive || payload.Severity != SemanticDiffSeverityInfo || payload.Differences[0].Severity != SemanticDiffSeverityInfo {
		t.Fatalf("unexpected normalized payload: %#v", payload)
	}
}
