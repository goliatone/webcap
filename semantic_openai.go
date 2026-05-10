package webcap

import (
	"context"
	"net/http"

	"github.com/goliatone/webcap/pkg/llms"
)

type OpenAISemanticProviderOptions struct {
	CredentialResolver SemanticCredentialResolver
	HTTPClient         *http.Client
	BaseURL            string
}

func NewOpenAISemanticDiffProvider(options OpenAISemanticProviderOptions) SemanticDiffProvider {
	return NewLLMSSemanticDiffProvider(llms.NewOpenAIProvider(llms.OpenAIOptions{
		CredentialResolver: llmsCredentialResolver(options.CredentialResolver),
		HTTPClient:         options.HTTPClient,
		BaseURL:            options.BaseURL,
	}))
}

func llmsCredentialResolver(resolver SemanticCredentialResolver) llms.CredentialResolver {
	if resolver == nil {
		return nil
	}
	return func(ctx context.Context, provider string) (string, error) {
		return resolver(ctx, provider)
	}
}
