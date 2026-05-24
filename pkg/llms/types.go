package llms

import (
	"context"
	"strings"
	"time"
)

const (
	ProviderOpenAI     = "openai"
	ProviderAnthropic  = "anthropic"
	ProviderCodexCLI   = "codex-cli"
	ImageRoleCurrent   = "current"
	ImageRoleReference = "reference"
	ImageRoleDiff      = "diff"
)

type CredentialResolver func(ctx context.Context, provider string) (string, error)

type Provider interface {
	Name() string
	CompareImages(ctx context.Context, req Request) (Response, error)
}

type Request struct {
	Provider            string
	Model               string
	Prompt              string
	Images              []Image
	Timeout             time.Duration
	MaxOutputTokens     int
	StructuredJSON      bool
	MaxRequestBodyBytes int64
	Metadata            map[string]string
}

type Image struct {
	Role       string
	Path       string
	MIMEType   string
	Base64Data string
	ByteSize   int64
}

type Response struct {
	Provider string
	Model    string
	RawText  string
	Warnings []Warning
	Metadata map[string]string
	Usage    Usage
	Exit     Exit
	Timing   Timing
}

type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Exit struct {
	Code      int
	TimedOut  bool
	Cancelled bool
}

type Timing struct {
	StartedAt   time.Time
	CompletedAt time.Time
	Duration    time.Duration
}

type Options struct {
	OpenAI     OpenAIOptions
	Anthropic  AnthropicOptions
	CodexCLI   CodexCLIOptions
	Providers  map[string]Provider
	DefaultMap map[string]string
}

func NormalizeProviderName(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func CloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		normalized := NormalizeProviderName(key)
		if normalized == "" {
			continue
		}
		out[normalized] = strings.TrimSpace(value)
	}
	return out
}

func CloneMetadata(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(value)
	}
	return out
}

func CloneProviders(values map[string]Provider) map[string]Provider {
	out := map[string]Provider{}
	for name, provider := range values {
		if provider == nil {
			continue
		}
		key := NormalizeProviderName(name)
		if key == "" {
			key = NormalizeProviderName(provider.Name())
		}
		if key != "" {
			out[key] = provider
		}
	}
	return out
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
