package llms

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func EnvironmentCredentialResolver(_ context.Context, provider string) (string, error) {
	switch NormalizeProviderName(provider) {
	case ProviderOpenAI:
		return strings.TrimSpace(os.Getenv("OPENAI_API_KEY")), nil
	case ProviderAnthropic:
		return strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")), nil
	default:
		return "", nil
	}
}

func ResolveCredential(ctx context.Context, resolver CredentialResolver, provider string) (string, error) {
	if resolver == nil {
		resolver = EnvironmentCredentialResolver
	}
	key, err := resolver(ctx, provider)
	if err != nil {
		return "", fmt.Errorf("resolve %s credentials: %w", NormalizeProviderName(provider), err)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("%s API key is required", NormalizeProviderName(provider))
	}
	return key, nil
}

func HTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return http.DefaultClient
}
