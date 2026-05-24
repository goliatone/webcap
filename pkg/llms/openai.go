package llms

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const defaultOpenAIResponsesURL = "https://api.openai.com/v1/responses"

type OpenAIProvider struct {
	credentialResolver CredentialResolver
	httpClient         *http.Client
	baseURL            string
}

func NewOpenAIProvider(options OpenAIOptions) *OpenAIProvider {
	baseURL := strings.TrimSpace(options.BaseURL)
	if baseURL == "" {
		baseURL = defaultOpenAIResponsesURL
	}
	return &OpenAIProvider{
		credentialResolver: options.CredentialResolver,
		httpClient:         HTTPClient(options.HTTPClient),
		baseURL:            baseURL,
	}
}

func (p *OpenAIProvider) Name() string { return ProviderOpenAI }

func (p *OpenAIProvider) CompareImages(ctx context.Context, req Request) (Response, error) {
	key, err := ResolveCredential(ctx, p.credentialResolver, p.Name())
	if err != nil {
		return Response{}, err
	}
	body, err := BuildOpenAIRequest(req)
	if err != nil {
		return Response{}, err
	}
	payload, _, err := postJSON(ctx, p.httpClient, p.Name(), p.baseURL, key, nil, body)
	if err != nil {
		return Response{}, err
	}
	text, model, usage, err := ParseOpenAIResponse(payload)
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

func BuildOpenAIRequest(req Request) ([]byte, error) {
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
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	if req.MaxRequestBodyBytes > 0 && int64(len(payload)) > req.MaxRequestBodyBytes {
		return nil, &PayloadBudgetError{
			Provider:    ProviderOpenAI,
			LimitName:   "max_request_body_bytes",
			LimitValue:  req.MaxRequestBodyBytes,
			ActualValue: int64(len(payload)),
		}
	}
	return payload, nil
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

func ParseOpenAIResponse(payload []byte) (string, string, Usage, error) {
	var body openAIResponsePayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return "", "", Usage{}, err
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
	usage := Usage{
		InputTokens:  body.Usage.InputTokens,
		OutputTokens: body.Usage.OutputTokens,
		TotalTokens:  body.Usage.TotalTokens,
	}
	text := strings.Join(parts, "\n")
	if text == "" {
		return "", body.Model, usage, errors.New("openai response contained no text")
	}
	return text, body.Model, usage, nil
}
