package llms

import (
	"net/http"
	"time"
)

type OpenAIOptions struct {
	CredentialResolver CredentialResolver
	HTTPClient         *http.Client
	BaseURL            string
}

type AnthropicOptions struct {
	CredentialResolver CredentialResolver
	HTTPClient         *http.Client
	BaseURL            string
}

type CodexCLIOptions struct {
	CommandPath      string
	WorkingDir       string
	Profile          string
	Model            string
	ExtraArgs        []string
	UseOSS           bool
	LocalProvider    string
	Ephemeral        bool
	IgnoreRules      bool
	OutputSchemaPath string
	StderrLimit      int
	Timeout          time.Duration
}
