package webcap

import (
	"fmt"
	"strings"
)

func normalizeWorkflowSemanticDiff(value WorkflowSemanticDiff) (WorkflowSemanticDiff, error) {
	value.Provider = normalizeSemanticProviderName(value.Provider)
	value.Model = strings.TrimSpace(value.Model)
	value.Mode = SemanticDiffMode(strings.TrimSpace(strings.ToLower(string(value.Mode))))
	if value.Mode == "" {
		value.Mode = SemanticDiffModeGeneral
	}
	if !isValidSemanticDiffMode(value.Mode) {
		return WorkflowSemanticDiff{}, newCaptureError(CodeValidation, "normalize_workflow_semantic_diff", fmt.Sprintf("unsupported semantic diff mode %q", value.Mode), nil)
	}
	value.Run = SemanticDiffRunPolicy(strings.TrimSpace(strings.ToLower(string(value.Run))))
	switch value.Run {
	case "", SemanticDiffRunChangedOnly:
		value.Run = SemanticDiffRunChangedOnly
	case SemanticDiffRunAlways, SemanticDiffRunNever:
	default:
		return WorkflowSemanticDiff{}, newCaptureError(CodeValidation, "normalize_workflow_semantic_diff", fmt.Sprintf("unsupported semantic diff run policy %q", value.Run), nil)
	}
	value.AdvisoryPolicy = SemanticDiffAdvisoryPolicy(strings.TrimSpace(strings.ToLower(string(value.AdvisoryPolicy))))
	switch value.AdvisoryPolicy {
	case "", SemanticDiffAdvisoryOnly:
		value.AdvisoryPolicy = SemanticDiffAdvisoryOnly
	case SemanticDiffAdvisoryEnforce:
	default:
		return WorkflowSemanticDiff{}, newCaptureError(CodeValidation, "normalize_workflow_semantic_diff", fmt.Sprintf("unsupported semantic diff advisory policy %q", value.AdvisoryPolicy), nil)
	}
	value.FailureSeverity = SemanticDiffSeverity(strings.TrimSpace(strings.ToLower(string(value.FailureSeverity))))
	if value.FailureSeverity != "" && !isValidSemanticSeverity(value.FailureSeverity) {
		return WorkflowSemanticDiff{}, newCaptureError(CodeValidation, "normalize_workflow_semantic_diff", fmt.Sprintf("unsupported semantic diff failure severity %q", value.FailureSeverity), nil)
	}
	value.Focus = normalizeStrings(value.Focus)
	value.Prompt = strings.TrimSpace(value.Prompt)
	value.PromptPath = strings.TrimSpace(value.PromptPath)
	value.Timeout = strings.TrimSpace(value.Timeout)
	if value.Timeout != "" {
		if _, err := ParseDurationOrDefault(value.Timeout, defaultSemanticDiffTimeout); err != nil {
			return WorkflowSemanticDiff{}, newCaptureError(CodeValidation, "normalize_workflow_semantic_diff", "invalid semantic diff timeout duration", err)
		}
	}
	if value.MaxOutputTokens < 0 {
		return WorkflowSemanticDiff{}, newCaptureError(CodeValidation, "normalize_workflow_semantic_diff", "semantic diff max output tokens must be >= 0", nil)
	}
	value.RawResponsePath = strings.TrimSpace(value.RawResponsePath)
	if value.RawResponsePath != "" && !value.PersistRawResponse {
		return WorkflowSemanticDiff{}, newCaptureError(CodeValidation, "normalize_workflow_semantic_diff", "semantic diff raw_response_path requires persist_raw_response", nil)
	}
	for i := range value.FailureVerdicts {
		value.FailureVerdicts[i] = SemanticDiffVerdict(strings.TrimSpace(strings.ToLower(string(value.FailureVerdicts[i]))))
		if !isValidSemanticVerdict(value.FailureVerdicts[i]) {
			return WorkflowSemanticDiff{}, newCaptureError(CodeValidation, "normalize_workflow_semantic_diff", fmt.Sprintf("unsupported semantic diff failure verdict %q", value.FailureVerdicts[i]), nil)
		}
	}
	if strings.TrimSpace(value.APIKey) != "" || strings.TrimSpace(value.OpenAIAPIKey) != "" || strings.TrimSpace(value.AnthropicAPIKey) != "" {
		return WorkflowSemanticDiff{}, newCaptureError(CodeValidation, "normalize_workflow_semantic_diff", "workflow semantic_diff must not contain provider API keys", nil)
	}
	return value, nil
}

func mergeWorkflowSemanticDiff(base, override WorkflowSemanticDiff) WorkflowSemanticDiff {
	out := base
	if override.Enabled != nil {
		enabled := *override.Enabled
		out.Enabled = &enabled
	}
	if override.Provider != "" {
		out.Provider = override.Provider
	}
	if override.Model != "" {
		out.Model = override.Model
	}
	if override.Mode != "" {
		out.Mode = override.Mode
	}
	if override.Run != "" {
		out.Run = override.Run
	}
	if len(override.Focus) > 0 {
		out.Focus = append([]string(nil), override.Focus...)
	}
	if override.Prompt != "" {
		out.Prompt = override.Prompt
	}
	if override.PromptPath != "" {
		out.PromptPath = override.PromptPath
	}
	if override.Timeout != "" {
		out.Timeout = override.Timeout
	}
	if override.MaxOutputTokens != 0 {
		out.MaxOutputTokens = override.MaxOutputTokens
	}
	if override.PersistRawResponse {
		out.PersistRawResponse = true
	}
	if override.RawResponsePath != "" {
		out.RawResponsePath = override.RawResponsePath
	}
	if override.AdvisoryPolicy != "" {
		out.AdvisoryPolicy = override.AdvisoryPolicy
	}
	if override.FailureSeverity != "" {
		out.FailureSeverity = override.FailureSeverity
	}
	if len(override.FailureVerdicts) > 0 {
		out.FailureVerdicts = append([]SemanticDiffVerdict(nil), override.FailureVerdicts...)
	}
	return out
}

func (value WorkflowSemanticDiff) enabled() bool {
	return value.Enabled != nil && *value.Enabled
}
