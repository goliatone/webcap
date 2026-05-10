package webcap

import (
	"context"
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
		Provider:        req.Provider,
		Model:           req.Model,
		Prompt:          req.Prompt,
		Images:          semanticImagesToLLMS(req.Images),
		Timeout:         req.Timeout,
		MaxOutputTokens: req.MaxOutputTokens,
		StructuredJSON:  req.StructuredJSON,
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
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "api key is required"):
		return newCaptureError(CodeValidation, "resolve_semantic_provider_credentials", err.Error(), err)
	case strings.Contains(message, "timed out") || strings.Contains(message, "deadline exceeded"):
		return newCaptureError(CodeTimeout, "semantic_llms_provider", "capture timed out", err)
	default:
		return wrapCaptureError("semantic_llms_provider", err)
	}
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
