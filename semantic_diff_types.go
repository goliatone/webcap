package webcap

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultSemanticDiffTimeout         = 60 * time.Second
	defaultSemanticDiffMaxImageBytes   = 8 * 1024 * 1024
	defaultSemanticDiffMaxOutputTokens = 1200
)

type SemanticDiffMode string

const (
	SemanticDiffModeGeneral       SemanticDiffMode = "general"
	SemanticDiffModeFocused       SemanticDiffMode = "focused"
	SemanticDiffModeCopy          SemanticDiffMode = "copy"
	SemanticDiffModeLayout        SemanticDiffMode = "layout"
	SemanticDiffModeAccessibility SemanticDiffMode = "accessibility"
	SemanticDiffModeCustom        SemanticDiffMode = "custom"
)

type SemanticDiffVerdict string

const (
	SemanticDiffVerdictNoMeaningfulChange SemanticDiffVerdict = "no_meaningful_change"
	SemanticDiffVerdictNeedsReview        SemanticDiffVerdict = "needs_review"
	SemanticDiffVerdictRegression         SemanticDiffVerdict = "regression"
	SemanticDiffVerdictImprovement        SemanticDiffVerdict = "improvement"
	SemanticDiffVerdictInconclusive       SemanticDiffVerdict = "inconclusive"
)

type SemanticDiffSeverity string

const (
	SemanticDiffSeverityInfo     SemanticDiffSeverity = "info"
	SemanticDiffSeverityMinor    SemanticDiffSeverity = "minor"
	SemanticDiffSeverityMajor    SemanticDiffSeverity = "major"
	SemanticDiffSeverityBlocking SemanticDiffSeverity = "blocking"
)

type SemanticDiffRunPolicy string

const (
	SemanticDiffRunNever       SemanticDiffRunPolicy = "never"
	SemanticDiffRunAlways      SemanticDiffRunPolicy = "always"
	SemanticDiffRunChangedOnly SemanticDiffRunPolicy = "changed_only"
)

type SemanticDiffAdvisoryPolicy string

const (
	SemanticDiffAdvisoryOnly    SemanticDiffAdvisoryPolicy = "advisory"
	SemanticDiffAdvisoryEnforce SemanticDiffAdvisoryPolicy = "enforce"
)

type SemanticDiffRequest struct {
	CurrentPath        string               `json:"current_path" yaml:"current_path"`
	ReferencePath      string               `json:"reference_path" yaml:"reference_path"`
	Provider           string               `json:"provider,omitempty" yaml:"provider,omitempty"`
	Model              string               `json:"model,omitempty" yaml:"model,omitempty"`
	Mode               SemanticDiffMode     `json:"mode,omitempty" yaml:"mode,omitempty"`
	Prompt             string               `json:"prompt,omitempty" yaml:"prompt,omitempty"`
	PromptPath         string               `json:"prompt_path,omitempty" yaml:"prompt_path,omitempty"`
	Focus              []string             `json:"focus,omitempty" yaml:"focus,omitempty"`
	PixelContext       SemanticPixelContext `json:"pixel_context" yaml:"pixel_context,omitempty"`
	MetadataPath       string               `json:"metadata_path,omitempty" yaml:"metadata_path,omitempty"`
	RawResponsePath    string               `json:"raw_response_path,omitempty" yaml:"raw_response_path,omitempty"`
	Timeout            string               `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	MaxOutputTokens    int                  `json:"max_output_tokens,omitempty" yaml:"max_output_tokens,omitempty"`
	PersistRawResponse bool                 `json:"persist_raw_response,omitempty" yaml:"persist_raw_response,omitempty"`
}

type SemanticPixelContext struct {
	PixelDiffImagePath string           `json:"pixel_diff_image_path,omitempty" yaml:"pixel_diff_image_path,omitempty"`
	ChangedPixels      int              `json:"changed_pixels,omitempty" yaml:"changed_pixels,omitempty"`
	TotalPixels        int              `json:"total_pixels,omitempty" yaml:"total_pixels,omitempty"`
	ChangedPercent     float64          `json:"changed_percent,omitempty" yaml:"changed_percent,omitempty"`
	Threshold          float64          `json:"threshold,omitempty" yaml:"threshold,omitempty"`
	Warnings           []CaptureWarning `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type SemanticPromptMetadata struct {
	Mode                 SemanticDiffMode `json:"mode"`
	Focus                []string         `json:"focus,omitempty"`
	PromptSource         string           `json:"prompt_source"`
	StructuredOutput     bool             `json:"structured_output"`
	PixelContextIncluded bool             `json:"pixel_context_included"`
}

type SemanticDifference struct {
	Area           string               `json:"area,omitempty"`
	Description    string               `json:"description"`
	Severity       SemanticDiffSeverity `json:"severity"`
	Evidence       string               `json:"evidence,omitempty"`
	Recommendation string               `json:"recommendation,omitempty"`
}

type SemanticDiffResult struct {
	CurrentPath      string                  `json:"current_path"`
	ReferencePath    string                  `json:"reference_path"`
	Provider         string                  `json:"provider"`
	Model            string                  `json:"model,omitempty"`
	Summary          string                  `json:"summary"`
	Differences      []SemanticDifference    `json:"differences,omitempty"`
	Verdict          SemanticDiffVerdict     `json:"verdict"`
	Severity         SemanticDiffSeverity    `json:"severity"`
	Prompt           SemanticPromptMetadata  `json:"prompt"`
	PixelContext     SemanticPixelContext    `json:"pixel_context"`
	MetadataPath     string                  `json:"metadata_path,omitempty"`
	RawResponsePath  string                  `json:"raw_response_path,omitempty"`
	Warnings         []CaptureWarning        `json:"warnings,omitempty"`
	ImageMetadata    []SemanticImageMetadata `json:"image_metadata,omitempty"`
	ProviderMetadata map[string]string       `json:"provider_metadata,omitempty"`
	Usage            SemanticProviderUsage   `json:"usage"`
	StartedAt        time.Time               `json:"started_at"`
	CompletedAt      time.Time               `json:"completed_at"`
	Duration         string                  `json:"duration,omitempty"`
	RawResponse      string                  `json:"-"`
}

type SemanticImageMetadata struct {
	Role             string         `json:"role"`
	OriginalPath     string         `json:"original_path"`
	MIMEType         string         `json:"mime_type"`
	OriginalByteSize int64          `json:"original_byte_size"`
	OriginalWidth    int            `json:"original_width,omitempty"`
	OriginalHeight   int            `json:"original_height,omitempty"`
	ProviderPath     string         `json:"provider_path,omitempty"`
	ProviderByteSize int64          `json:"provider_byte_size"`
	ProviderWidth    int            `json:"provider_width,omitempty"`
	ProviderHeight   int            `json:"provider_height,omitempty"`
	ResizeReason     string         `json:"resize_reason,omitempty"`
	Limits           map[string]any `json:"limits,omitempty"`
}

type SemanticDiffService interface {
	SemanticDiff(ctx context.Context, req SemanticDiffRequest) (SemanticDiffResult, error)
}

type SemanticImageRedactionInput struct {
	Role string
	Path string
}

type SemanticImageRedactor func(ctx context.Context, input SemanticImageRedactionInput) (string, error)

func NormalizeSemanticDiffRequest(req SemanticDiffRequest) (SemanticDiffRequest, error) {
	req.CurrentPath = strings.TrimSpace(req.CurrentPath)
	req.ReferencePath = strings.TrimSpace(req.ReferencePath)
	req.Provider = normalizeSemanticProviderName(req.Provider)
	req.Model = strings.TrimSpace(req.Model)
	req.Mode = SemanticDiffMode(strings.TrimSpace(strings.ToLower(string(req.Mode))))
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.PromptPath = strings.TrimSpace(req.PromptPath)
	req.Focus = normalizeStrings(req.Focus)
	req.MetadataPath = strings.TrimSpace(req.MetadataPath)
	req.RawResponsePath = strings.TrimSpace(req.RawResponsePath)
	req.Timeout = strings.TrimSpace(req.Timeout)
	req.PixelContext = normalizeSemanticPixelContext(req.PixelContext)

	if req.CurrentPath == "" {
		return SemanticDiffRequest{}, newCaptureError(CodeValidation, "normalize_semantic_diff_request", "current image path is required", nil)
	}
	if req.ReferencePath == "" {
		return SemanticDiffRequest{}, newCaptureError(CodeValidation, "normalize_semantic_diff_request", "reference image path is required", nil)
	}
	if req.Mode == "" {
		req.Mode = SemanticDiffModeGeneral
	}
	if !isValidSemanticDiffMode(req.Mode) {
		return SemanticDiffRequest{}, newCaptureError(CodeValidation, "normalize_semantic_diff_request", fmt.Sprintf("unsupported semantic diff mode %q", req.Mode), nil)
	}
	if req.Mode == SemanticDiffModeCustom && req.Prompt == "" && req.PromptPath == "" {
		return SemanticDiffRequest{}, newCaptureError(CodeValidation, "normalize_semantic_diff_request", "custom semantic diff mode requires prompt or prompt_path", nil)
	}
	if req.MaxOutputTokens < 0 {
		return SemanticDiffRequest{}, newCaptureError(CodeValidation, "normalize_semantic_diff_request", "max output tokens must be >= 0", nil)
	}
	if req.Timeout != "" {
		if _, err := time.ParseDuration(req.Timeout); err != nil {
			return SemanticDiffRequest{}, newCaptureError(CodeValidation, "normalize_semantic_diff_request", "invalid semantic diff timeout duration", err)
		}
	}
	if req.RawResponsePath != "" && !req.PersistRawResponse {
		return SemanticDiffRequest{}, newCaptureError(CodeValidation, "normalize_semantic_diff_request", "raw_response_path requires persist_raw_response", nil)
	}
	return req, nil
}

func (req SemanticDiffRequest) timeout(defaultValue time.Duration) time.Duration {
	value, _ := ParseDurationOrDefault(req.Timeout, defaultValue)
	return value
}

func normalizeSemanticPixelContext(value SemanticPixelContext) SemanticPixelContext {
	value.PixelDiffImagePath = strings.TrimSpace(value.PixelDiffImagePath)
	if value.ChangedPixels < 0 {
		value.ChangedPixels = 0
	}
	if value.TotalPixels < 0 {
		value.TotalPixels = 0
	}
	if value.ChangedPercent < 0 {
		value.ChangedPercent = 0
	}
	if value.Threshold < 0 {
		value.Threshold = 0
	}
	value.Warnings = append([]CaptureWarning(nil), value.Warnings...)
	return value
}

func isValidSemanticDiffMode(value SemanticDiffMode) bool {
	switch value {
	case SemanticDiffModeGeneral, SemanticDiffModeFocused, SemanticDiffModeCopy, SemanticDiffModeLayout, SemanticDiffModeAccessibility, SemanticDiffModeCustom:
		return true
	default:
		return false
	}
}

func normalizeSemanticProviderName(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func defaultSemanticMetadataPath(req SemanticDiffRequest) string {
	current := sanitizeName(filepath.Base(req.CurrentPath))
	reference := sanitizeName(filepath.Base(req.ReferencePath))
	return filepath.Join(defaultOutputDirectory, fmt.Sprintf("%s-vs-%s.semantic.json", current, reference))
}
