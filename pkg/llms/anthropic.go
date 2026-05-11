package llms

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const defaultAnthropicMessagesURL = "https://api.anthropic.com/v1/messages"

type AnthropicProvider struct {
	credentialResolver CredentialResolver
	httpClient         *http.Client
	baseURL            string
}

func NewAnthropicProvider(options AnthropicOptions) *AnthropicProvider {
	baseURL := strings.TrimSpace(options.BaseURL)
	if baseURL == "" {
		baseURL = defaultAnthropicMessagesURL
	}
	return &AnthropicProvider{
		credentialResolver: options.CredentialResolver,
		httpClient:         HTTPClient(options.HTTPClient),
		baseURL:            baseURL,
	}
}

func (p *AnthropicProvider) Name() string { return ProviderAnthropic }

func (p *AnthropicProvider) CompareImages(ctx context.Context, req Request) (Response, error) {
	key, err := ResolveCredential(ctx, p.credentialResolver, p.Name())
	if err != nil {
		return Response{}, err
	}
	body, err := BuildAnthropicRequest(req)
	if err != nil {
		return Response{}, err
	}
	payload, _, err := postJSON(ctx, p.httpClient, p.baseURL, "", map[string]string{
		"x-api-key":         key,
		"anthropic-version": "2023-06-01",
	}, body)
	if err != nil {
		return Response{}, err
	}
	text, model, usage, err := ParseAnthropicResponse(payload)
	if err != nil {
		return Response{}, err
	}
	return Response{
		Provider: p.Name(),
		Model:    FirstNonEmpty(model, req.Model),
		RawText:  text,
		Usage:    usage,
	}, nil
}

func BuildAnthropicRequest(req Request) ([]byte, error) {
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

func ParseAnthropicResponse(payload []byte) (string, string, Usage, error) {
	var body anthropicResponsePayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return "", "", Usage{}, err
	}
	var parts []string
	for _, content := range body.Content {
		if content.Type == "text" {
			if text := strings.TrimSpace(content.Text); text != "" {
				parts = append(parts, text)
			}
		}
	}
	usage := Usage{
		InputTokens:  body.Usage.InputTokens,
		OutputTokens: body.Usage.OutputTokens,
		TotalTokens:  body.Usage.InputTokens + body.Usage.OutputTokens,
	}
	text := strings.Join(parts, "\n")
	if text == "" {
		return "", body.Model, usage, errors.New("anthropic response contained no text")
	}
	return text, body.Model, usage, nil
}
