package webcap

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

const defaultOpenAIResponsesURL = "https://api.openai.com/v1/responses"

type OpenAISemanticProviderOptions struct {
	CredentialResolver SemanticCredentialResolver
	HTTPClient         *http.Client
	BaseURL            string
}

type openAISemanticDiffProvider struct {
	credentialResolver SemanticCredentialResolver
	httpClient         *http.Client
	baseURL            string
}

func NewOpenAISemanticDiffProvider(options OpenAISemanticProviderOptions) SemanticDiffProvider {
	baseURL := strings.TrimSpace(options.BaseURL)
	if baseURL == "" {
		baseURL = defaultOpenAIResponsesURL
	}
	return &openAISemanticDiffProvider{
		credentialResolver: options.CredentialResolver,
		httpClient:         semanticHTTPClient(options.HTTPClient),
		baseURL:            baseURL,
	}
}

func (p *openAISemanticDiffProvider) Name() string { return "openai" }

func (p *openAISemanticDiffProvider) CompareImages(ctx context.Context, req SemanticProviderRequest) (SemanticProviderResponse, error) {
	key, err := semanticCredential(ctx, p.credentialResolver, p.Name())
	if err != nil {
		return SemanticProviderResponse{}, err
	}
	body, err := buildOpenAISemanticRequest(req)
	if err != nil {
		return SemanticProviderResponse{}, err
	}
	payload, _, err := postSemanticJSON(ctx, p.httpClient, p.baseURL, key, nil, body)
	if err != nil {
		return SemanticProviderResponse{}, err
	}
	text, model, usage, err := parseOpenAISemanticResponse(payload)
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

func buildOpenAISemanticRequest(req SemanticProviderRequest) ([]byte, error) {
	content := []map[string]any{{"type": "input_text", "text": req.Prompt}}
	for _, image := range req.Images {
		content = append(content, map[string]any{
			"type":      "input_image",
			"image_url": "data:" + image.MIMEType + ";base64," + image.Base64Data,
		})
	}
	body := map[string]any{
		"model": req.Model,
		"input": []map[string]any{{
			"role":    "user",
			"content": content,
		}},
		"max_output_tokens": req.MaxOutputTokens,
		"store":             false,
	}
	if req.StructuredJSON {
		body["text"] = map[string]any{
			"format": map[string]any{"type": "json_object"},
		}
	}
	return json.Marshal(body)
}

type openAIResponsePayload struct {
	Model  string `json:"model"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func parseOpenAISemanticResponse(payload []byte) (string, string, SemanticProviderUsage, error) {
	var body openAIResponsePayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return "", "", SemanticProviderUsage{}, wrapCaptureError("parse_openai_semantic_response", err)
	}
	var parts []string
	for _, output := range body.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" || content.Type == "text" {
				if text := strings.TrimSpace(content.Text); text != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.Join(parts, "\n"), body.Model, SemanticProviderUsage{
		InputTokens:  body.Usage.InputTokens,
		OutputTokens: body.Usage.OutputTokens,
		TotalTokens:  body.Usage.TotalTokens,
	}, nil
}
