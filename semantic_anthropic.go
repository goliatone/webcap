package webcap

import (
	"net/http"

	"github.com/goliatone/webcap/pkg/llms"
)

type AnthropicSemanticProviderOptions struct {
	CredentialResolver SemanticCredentialResolver
	HTTPClient         *http.Client
	BaseURL            string
}

func NewAnthropicSemanticDiffProvider(options AnthropicSemanticProviderOptions) SemanticDiffProvider {
	return NewLLMSSemanticDiffProvider(llms.NewAnthropicProvider(llms.AnthropicOptions{
		CredentialResolver: llmsCredentialResolver(options.CredentialResolver),
		HTTPClient:         options.HTTPClient,
		BaseURL:            options.BaseURL,
	}))
}
