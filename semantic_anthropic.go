package webcap

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

const defaultAnthropicMessagesURL = "https://api.anthropic.com/v1/messages"

type AnthropicSemanticProviderOptions struct {
	CredentialResolver SemanticCredentialResolver
	HTTPClient         *http.Client
	BaseURL            string
}

type anthropicSemanticDiffProvider struct {
	credentialResolver SemanticCredentialResolver
	httpClient         *http.Client
	baseURL            string
}

func NewAnthropicSemanticDiffProvider(options AnthropicSemanticProviderOptions) SemanticDiffProvider {
	baseURL := strings.TrimSpace(options.BaseURL)
	if baseURL == "" {
		baseURL = defaultAnthropicMessagesURL
	}
	return &anthropicSemanticDiffProvider{
		credentialResolver: options.CredentialResolver,
		httpClient:         semanticHTTPClient(options.HTTPClient),
		baseURL:            baseURL,
	}
}

func (p *anthropicSemanticDiffProvider) Name() string { return "anthropic" }

func (p *anthropicSemanticDiffProvider) CompareImages(ctx context.Context, req SemanticProviderRequest) (SemanticProviderResponse, error) {
	key, err := semanticCredential(ctx, p.credentialResolver, p.Name())
	if err != nil {
		return SemanticProviderResponse{}, err
	}
	body, err := buildAnthropicSemanticRequest(req)
	if err != nil {
		return SemanticProviderResponse{}, err
	}
	payload, _, err := postSemanticJSON(ctx, p.httpClient, p.baseURL, "", map[string]string{
		"x-api-key":         key,
		"anthropic-version": "2023-06-01",
	}, body)
	if err != nil {
		return SemanticProviderResponse{}, err
	}
	text, model, usage, err := parseAnthropicSemanticResponse(payload)
	if err != nil {
		return SemanticProviderResponse{}, err
	}
	return SemanticProviderResponse{
		Provider: p.Name(),
		Model:    firstNonEmpty(model, req.Model),
		RawText:  text,
		Usage:    usage,
	}, nil
}

func buildAnthropicSemanticRequest(req SemanticProviderRequest) ([]byte, error) {
	content := []map[string]any{{"type": "text", "text": req.Prompt}}
	for _, image := range req.Images {
		content = append(content, map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": image.MIMEType,
				"data":       image.Base64Data,
			},
		})
	}
	body := map[string]any{
		"model":      req.Model,
		"max_tokens": req.MaxOutputTokens,
		"messages": []map[string]any{{
			"role":    "user",
			"content": content,
		}},
	}
	return json.Marshal(body)
}

type anthropicResponsePayload struct {
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func parseAnthropicSemanticResponse(payload []byte) (string, string, SemanticProviderUsage, error) {
	var body anthropicResponsePayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return "", "", SemanticProviderUsage{}, wrapCaptureError("parse_anthropic_semantic_response", err)
	}
	var parts []string
	for _, content := range body.Content {
		if content.Type == "text" {
			if text := strings.TrimSpace(content.Text); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n"), body.Model, SemanticProviderUsage{
		InputTokens:  body.Usage.InputTokens,
		OutputTokens: body.Usage.OutputTokens,
		TotalTokens:  body.Usage.InputTokens + body.Usage.OutputTokens,
	}, nil
}
