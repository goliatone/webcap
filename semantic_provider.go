package webcap

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type SemanticCredentialResolver func(ctx context.Context, provider string) (string, error)

type SemanticDiffProvider interface {
	Name() string
	CompareImages(ctx context.Context, req SemanticProviderRequest) (SemanticProviderResponse, error)
}

type SemanticProviderRequest struct {
	Provider        string
	Model           string
	Prompt          string
	Images          []SemanticImagePayload
	Timeout         time.Duration
	MaxOutputTokens int
	StructuredJSON  bool
}

type SemanticProviderResponse struct {
	Provider string
	Model    string
	RawText  string
	Warnings []CaptureWarning
	Metadata map[string]string
	Usage    SemanticProviderUsage
}

type SemanticProviderUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

func (opts SemanticDiffOptions) normalized() SemanticDiffOptions {
	out := opts
	out.DefaultProvider = normalizeSemanticProviderName(out.DefaultProvider)
	if out.DefaultTimeout <= 0 {
		out.DefaultTimeout = defaultSemanticDiffTimeout
	}
	if out.MaxImageBytes <= 0 {
		out.MaxImageBytes = defaultSemanticDiffMaxImageBytes
	}
	if out.MaxOutputTokens <= 0 {
		out.MaxOutputTokens = defaultSemanticDiffMaxOutputTokens
	}
	if out.CredentialResolver == nil {
		out.CredentialResolver = SemanticEnvironmentCredentialResolver
	}
	if out.HTTPClient == nil {
		out.HTTPClient = http.DefaultClient
	}
	out.DefaultModels = cloneStringMap(out.DefaultModels)
	out.Providers = cloneSemanticProviderMap(out.Providers)
	out.Providers = withBuiltinSemanticProviders(out)
	return out
}

func withBuiltinSemanticProviders(opts SemanticDiffOptions) map[string]SemanticDiffProvider {
	providers := cloneSemanticProviderMap(opts.Providers)
	if providers["openai"] == nil {
		providers["openai"] = NewOpenAISemanticDiffProvider(OpenAISemanticProviderOptions{
			CredentialResolver: opts.CredentialResolver,
			HTTPClient:         opts.HTTPClient,
			BaseURL:            opts.OpenAIBaseURL,
		})
	}
	if providers["anthropic"] == nil {
		providers["anthropic"] = NewAnthropicSemanticDiffProvider(AnthropicSemanticProviderOptions{
			CredentialResolver: opts.CredentialResolver,
			HTTPClient:         opts.HTTPClient,
			BaseURL:            opts.AnthropicBaseURL,
		})
	}
	return providers
}

func cloneSemanticProviderMap(values map[string]SemanticDiffProvider) map[string]SemanticDiffProvider {
	out := map[string]SemanticDiffProvider{}
	for name, provider := range values {
		key := normalizeSemanticProviderName(name)
		if key == "" && provider != nil {
			key = normalizeSemanticProviderName(provider.Name())
		}
		if key != "" && provider != nil {
			out[key] = provider
		}
	}
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[normalizeSemanticProviderName(key)] = strings.TrimSpace(value)
	}
	return out
}

func SemanticEnvironmentCredentialResolver(_ context.Context, provider string) (string, error) {
	switch normalizeSemanticProviderName(provider) {
	case "openai":
		return strings.TrimSpace(os.Getenv("OPENAI_API_KEY")), nil
	case "anthropic":
		return strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")), nil
	default:
		return "", nil
	}
}

func semanticCredential(ctx context.Context, resolver SemanticCredentialResolver, provider string) (string, error) {
	if resolver == nil {
		resolver = SemanticEnvironmentCredentialResolver
	}
	key, err := resolver(ctx, provider)
	if err != nil {
		return "", wrapCaptureError("resolve_semantic_provider_credentials", err)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", newCaptureError(CodeValidation, "resolve_semantic_provider_credentials", fmt.Sprintf("%s API key is required", normalizeSemanticProviderName(provider)), nil)
	}
	return key, nil
}

func semanticHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return http.DefaultClient
}

func postSemanticJSON(ctx context.Context, client *http.Client, url, apiKey string, headers map[string]string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, wrapCaptureError("semantic_provider_request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := semanticHTTPClient(client).Do(req)
	if err != nil {
		return nil, 0, wrapCaptureError("semantic_provider_http", err)
	}
	defer resp.Body.Close()
	payload, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp.StatusCode, wrapCaptureError("semantic_provider_http", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, newCaptureError(CodeCapture, "semantic_provider_http", fmt.Sprintf("semantic provider returned HTTP %d", resp.StatusCode), nil)
	}
	return payload, resp.StatusCode, nil
}
