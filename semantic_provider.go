package webcap

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/goliatone/webcap/pkg/llms"
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
		openAIOptions := opts.LLMs.OpenAI
		if openAIOptions.CredentialResolver == nil {
			openAIOptions.CredentialResolver = llmsCredentialResolver(opts.CredentialResolver)
		}
		if openAIOptions.HTTPClient == nil {
			openAIOptions.HTTPClient = opts.HTTPClient
		}
		if openAIOptions.BaseURL == "" {
			openAIOptions.BaseURL = opts.OpenAIBaseURL
		}
		providers["openai"] = NewOpenAISemanticDiffProvider(OpenAISemanticProviderOptions{
			CredentialResolver: semanticCredentialResolver(openAIOptions.CredentialResolver),
			HTTPClient:         openAIOptions.HTTPClient,
			BaseURL:            openAIOptions.BaseURL,
		})
	}
	if providers["anthropic"] == nil {
		anthropicOptions := opts.LLMs.Anthropic
		if anthropicOptions.CredentialResolver == nil {
			anthropicOptions.CredentialResolver = llmsCredentialResolver(opts.CredentialResolver)
		}
		if anthropicOptions.HTTPClient == nil {
			anthropicOptions.HTTPClient = opts.HTTPClient
		}
		if anthropicOptions.BaseURL == "" {
			anthropicOptions.BaseURL = opts.AnthropicBaseURL
		}
		providers["anthropic"] = NewAnthropicSemanticDiffProvider(AnthropicSemanticProviderOptions{
			CredentialResolver: semanticCredentialResolver(anthropicOptions.CredentialResolver),
			HTTPClient:         anthropicOptions.HTTPClient,
			BaseURL:            anthropicOptions.BaseURL,
		})
	}
	if providers["codex-cli"] == nil {
		providers["codex-cli"] = NewLLMSSemanticDiffProvider(llms.NewCodexCLIProvider(opts.LLMs.CodexCLI))
	}
	return providers
}

func semanticCredentialResolver(resolver llms.CredentialResolver) SemanticCredentialResolver {
	if resolver == nil {
		return nil
	}
	return func(ctx context.Context, provider string) (string, error) {
		return resolver(ctx, provider)
	}
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
