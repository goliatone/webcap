package webcap

import (
	"context"
	"errors"
	"strings"

	"github.com/goliatone/webcap/pkg/llms"
)

type llmsSemanticDiffProvider struct {
	provider llms.Provider
}

func NewLLMSSemanticDiffProvider(provider llms.Provider) SemanticDiffProvider {
	return &llmsSemanticDiffProvider{provider: provider}
}

func (p *llmsSemanticDiffProvider) Name() string {
	if p == nil || p.provider == nil {
		return ""
	}
	return p.provider.Name()
}

func (p *llmsSemanticDiffProvider) CompareImages(ctx context.Context, req SemanticProviderRequest) (SemanticProviderResponse, error) {
	if p == nil || p.provider == nil {
		return SemanticProviderResponse{}, newCaptureError(CodeValidation, "semantic_llms_provider", "LLM provider is required", nil)
	}
	resp, err := p.provider.CompareImages(ctx, llms.Request{
		Provider:            req.Provider,
		Model:               req.Model,
		Prompt:              req.Prompt,
		Images:              semanticImagesToLLMS(req.Images),
		Timeout:             req.Timeout,
		MaxOutputTokens:     req.MaxOutputTokens,
		StructuredJSON:      req.StructuredJSON,
		MaxRequestBodyBytes: req.MaxRequestBodyBytes,
	})
	if err != nil {
		return SemanticProviderResponse{}, mapLLMSError(err)
	}
	return SemanticProviderResponse{
		Provider: firstNonEmpty(resp.Provider, p.Name()),
		Model:    resp.Model,
		RawText:  resp.RawText,
		Warnings: llmsWarningsToCaptureWarnings(resp.Warnings),
		Metadata: cloneStringMap(resp.Metadata),
		Usage: SemanticProviderUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}, nil
}

func mapLLMSError(err error) error {
	if err == nil {
		return nil
	}
	var httpErr *llms.ProviderHTTPError
	if errors.As(err, &httpErr) {
		return semanticProviderHTTPError(httpErr)
	}
	var budgetErr *llms.PayloadBudgetError
	if errors.As(err, &budgetErr) {
		return newCaptureError(CodeProviderPayloadTooLarge, "semantic_llms_provider", "semantic provider payload exceeds request budget", err).
			WithMetadata("provider", budgetErr.Provider).
			WithMetadata("limit_name", budgetErr.LimitName).
			WithMetadata("limit_value", budgetErr.LimitValue).
			WithMetadata("actual_value", budgetErr.ActualValue)
	}
	var executionErr *llms.ExecutionError
	if errors.As(err, &executionErr) && (executionErr.TimedOut || executionErr.Cancelled) {
		return newCaptureError(CodeProviderTimeout, "semantic_llms_provider", "semantic provider timed out", err).
			WithMetadata("provider", executionErr.Provider).
			WithMetadata("timed_out", executionErr.TimedOut).
			WithMetadata("cancelled", executionErr.Cancelled)
	}
	if errors.As(err, &executionErr) {
		code := CodeProviderExecutionFailed
		message := "semantic provider execution failed"
		if codexInvalidSchema(executionErr) {
			code = CodeProviderInvalidRequest
			message = "semantic provider rejected the structured output schema"
		}
		return newCaptureError(code, "semantic_llms_provider", message, err).
			WithMetadata("provider", executionErr.Provider).
			WithMetadata("command", executionErr.Command).
			WithMetadata("exit_code", executionErr.ExitCode).
			WithMetadata("stdout", executionErr.Stdout).
			WithMetadata("stderr", executionErr.Stderr).
			WithMetadata("timed_out", executionErr.TimedOut).
			WithMetadata("cancelled", executionErr.Cancelled)
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "api key is required"):
		return newCaptureError(CodeProviderAuth, "resolve_semantic_provider_credentials", err.Error(), err)
	case strings.Contains(message, "timed out") || strings.Contains(message, "deadline exceeded"):
		return newCaptureError(CodeProviderTimeout, "semantic_llms_provider", "semantic provider timed out", err)
	default:
		return wrapCaptureError("semantic_llms_provider", err)
	}
}

func semanticProviderHTTPError(err *llms.ProviderHTTPError) *Error {
	code := semanticProviderHTTPCode(err)
	message := semanticProviderHTTPMessage(code, err)
	captureErr := newCaptureError(code, "semantic_llms_provider", message, err)
	for key, value := range err.Metadata() {
		captureErr.WithMetadata(key, value)
	}
	return captureErr
}

func semanticProviderHTTPCode(err *llms.ProviderHTTPError) ErrorCode {
	errorText := strings.ToLower(strings.Join([]string{err.ErrorType, err.ErrorCode, err.ErrorMessage}, " "))
	switch {
	case err.StatusCode == 429 || strings.Contains(errorText, "rate_limit"):
		return CodeProviderRateLimited
	case err.StatusCode == 413:
		return CodeProviderPayloadTooLarge
	case strings.Contains(errorText, "quota") || strings.Contains(errorText, "billing"):
		return CodeProviderQuota
	case err.StatusCode == 401:
		return CodeProviderAuth
	case err.StatusCode == 403:
		return CodeProviderQuota
	case err.StatusCode == 400:
		return CodeProviderInvalidRequest
	case err.StatusCode >= 500:
		return CodeProviderUnavailable
	default:
		return CodeProviderInvalidRequest
	}
}

func semanticProviderHTTPMessage(code ErrorCode, err *llms.ProviderHTTPError) string {
	provider := strings.TrimSpace(err.Provider)
	if provider == "" {
		provider = "provider"
	}
	switch code {
	case CodeProviderRateLimited:
		if err.RetryAfter != "" {
			return provider + " rate limited the semantic diff request; retry after " + err.RetryAfter
		}
		return provider + " rate limited the semantic diff request"
	case CodeProviderAuth:
		return provider + " rejected semantic diff credentials"
	case CodeProviderQuota:
		return provider + " rejected semantic diff because quota, billing, or permissions are exhausted"
	case CodeProviderPayloadTooLarge:
		return provider + " rejected semantic diff because the provider payload is too large"
	case CodeProviderUnavailable:
		return provider + " semantic diff provider is temporarily unavailable"
	default:
		return provider + " rejected the semantic diff request"
	}
}

func codexInvalidSchema(err *llms.ExecutionError) bool {
	text := strings.ToLower(err.Stdout + "\n" + err.Stderr + "\n" + err.Error())
	return strings.Contains(text, "invalid_json_schema") || strings.Contains(text, "invalid json schema")
}

func semanticImagesToLLMS(images []SemanticImagePayload) []llms.Image {
	out := make([]llms.Image, 0, len(images))
	for _, image := range images {
		role := image.Role
		if role == "pixel_diff" {
			role = llms.ImageRoleDiff
		}
		out = append(out, llms.Image{
			Role:       role,
			Path:       image.Path,
			MIMEType:   image.MIMEType,
			Base64Data: image.Base64Data,
			ByteSize:   image.ByteSize,
		})
	}
	return out
}

func llmsWarningsToCaptureWarnings(warnings []llms.Warning) []CaptureWarning {
	out := make([]CaptureWarning, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, CaptureWarning{
			Code:    warning.Code,
			Message: warning.Message,
		})
	}
	return out
}
