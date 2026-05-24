package webcap

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/goliatone/webcap/pkg/llms"
)

const semanticBudgetResizeMaxPasses = 8

type semanticProviderBudgetOptions struct {
	Image                        semanticImagePrepareOptions
	Provider                     string
	Model                        string
	Prompt                       string
	MaxOutputTokens              int
	StructuredJSON               bool
	MaxEncodedImageBytes         int64
	MaxCombinedEncodedImageBytes int64
	MaxRequestBodyBytes          int64
}

func satisfySemanticProviderBudgets(images []SemanticImagePayload, options semanticProviderBudgetOptions) ([]SemanticImagePayload, error) {
	var lastSize int64 = -1
	for pass := 0; pass <= semanticBudgetResizeMaxPasses; pass++ {
		actual, err := semanticProviderBudgetError(images, options)
		if err == nil {
			return images, nil
		}
		if !options.Image.ResizeImages {
			return nil, err
		}
		if actual > 0 && actual == lastSize {
			return nil, err
		}
		lastSize = actual
		limit, _ := errorLimitValues(err)
		scale := semanticScaleForBudget(limit, actual)
		if scale <= 0 || scale >= 1 {
			scale = 0.85
		}
		reason := errorLimitName(err)
		resized, resizeErr := resizeSemanticImagePayloads(images, options.Image, reason, limit, scale)
		if resizeErr != nil {
			return nil, resizeErr
		}
		images = resized
	}
	_, err := semanticProviderBudgetError(images, options)
	if err != nil {
		return nil, err
	}
	return images, nil
}

func semanticProviderBudgetError(images []SemanticImagePayload, options semanticProviderBudgetOptions) (int64, error) {
	if err := validateSemanticEncodedBudgets(images, options.MaxEncodedImageBytes, options.MaxCombinedEncodedImageBytes); err != nil {
		_, actual := errorLimitValues(err)
		return actual, err
	}
	bodySize, ok, err := semanticProviderRequestBodySize(images, options)
	if err != nil {
		return 0, err
	}
	if ok && options.MaxRequestBodyBytes > 0 && bodySize > options.MaxRequestBodyBytes {
		err := newCaptureError(CodeProviderPayloadTooLarge, "prepare_semantic_provider_request", "semantic diff provider payload exceeds request body budget", nil).
			WithMetadata("provider", options.Provider).
			WithMetadata("limit_name", "max_request_body_bytes").
			WithMetadata("limit_value", options.MaxRequestBodyBytes).
			WithMetadata("actual_value", bodySize)
		return bodySize, err
	}
	return bodySize, nil
}

func semanticProviderRequestBodySize(images []SemanticImagePayload, options semanticProviderBudgetOptions) (int64, bool, error) {
	req := llms.Request{
		Provider:        options.Provider,
		Model:           options.Model,
		Prompt:          options.Prompt,
		Images:          semanticImagesToLLMS(images),
		MaxOutputTokens: options.MaxOutputTokens,
		StructuredJSON:  options.StructuredJSON,
	}
	var payload []byte
	var err error
	switch normalizeSemanticProviderName(options.Provider) {
	case llms.ProviderOpenAI:
		payload, err = llms.BuildOpenAIRequest(req)
	case llms.ProviderAnthropic:
		payload, err = llms.BuildAnthropicRequest(req)
	default:
		return 0, false, nil
	}
	if err != nil {
		return 0, true, wrapCaptureError("prepare_semantic_provider_request", err)
	}
	return int64(len(payload)), true, nil
}

func resizeSemanticImagePayloads(images []SemanticImagePayload, options semanticImagePrepareOptions, reason string, limit int64, scale float64) ([]SemanticImagePayload, error) {
	if strings.TrimSpace(options.TempDir) == "" {
		return nil, newCaptureError(CodeValidation, "prepare_semantic_image", "semantic image resize requires a temporary directory", nil)
	}
	out := make([]SemanticImagePayload, 0, len(images))
	for _, image := range images {
		resized, err := resizeSemanticImagePayload(image, options, reason, limit, scale)
		if err != nil {
			return nil, err
		}
		out = append(out, resized)
	}
	return out, nil
}

func resizeSemanticImagePayload(image SemanticImagePayload, options semanticImagePrepareOptions, reason string, limit int64, scale float64) (SemanticImagePayload, error) {
	payload, err := readSemanticImageFile(image.Path)
	if err != nil {
		return SemanticImagePayload{}, wrapCaptureError("read_semantic_image", err)
	}
	options.ScaleFactor = scale
	resized, err := resizeSemanticImage(payload, image.MIMEType, options)
	if err != nil {
		return SemanticImagePayload{}, wrapCaptureError("resize_semantic_image", err)
	}
	width, height, err := semanticImageDimensions(resized)
	if err != nil {
		return SemanticImagePayload{}, newCaptureError(CodeValidation, "prepare_semantic_image", "semantic diff resized image could not be decoded", err)
	}
	metadata := image.Metadata
	if metadata.Role == "" {
		metadata.Role = image.Role
	}
	if metadata.OriginalPath == "" {
		metadata.OriginalPath = image.Path
	}
	if metadata.MIMEType == "" {
		metadata.MIMEType = image.MIMEType
	}
	metadata.ProviderByteSize = int64(len(resized))
	metadata.ProviderWidth = width
	metadata.ProviderHeight = height
	metadata.ResizeReason = appendSemanticResizeReason(metadata.ResizeReason, reason)
	if metadata.Limits == nil {
		metadata.Limits = semanticImageLimits(options)
	}
	if reason != "" && reason != "provider_budget" && limit > 0 {
		if metadata.Limits == nil {
			metadata.Limits = map[string]any{}
		}
		metadata.Limits[reason] = limit
	}
	copyPath := filepath.Join(options.TempDir, semanticProviderCopyName(image.Role, metadata.OriginalPath, image.MIMEType))
	if err := os.WriteFile(copyPath, resized, 0o600); err != nil {
		return SemanticImagePayload{}, wrapCaptureError("write_semantic_provider_image", err)
	}
	metadata.ProviderPath = copyPath
	return SemanticImagePayload{
		Role:       image.Role,
		Path:       copyPath,
		MIMEType:   image.MIMEType,
		Base64Data: base64.StdEncoding.EncodeToString(resized),
		ByteSize:   int64(len(resized)),
		Width:      width,
		Height:     height,
		Metadata:   metadata,
	}, nil
}

func appendSemanticResizeReason(existing, next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return existing
	}
	for part := range strings.SplitSeq(existing, ",") {
		if strings.TrimSpace(part) == next {
			return existing
		}
	}
	if strings.TrimSpace(existing) == "" {
		return next
	}
	return existing + "," + next
}

func semanticScaleForBudget(limit, actual int64) float64 {
	if limit <= 0 || actual <= 0 || actual <= limit {
		return 0.85
	}
	return sqrtFloat(float64(limit)/float64(actual)) * 0.92
}

func errorLimitName(err error) string {
	var captureErr *Error
	if !errors.As(err, &captureErr) || captureErr.Metadata == nil {
		return "provider_budget"
	}
	if value, ok := captureErr.Metadata["limit_name"].(string); ok && value != "" {
		return value
	}
	return "provider_budget"
}

func errorLimitValues(err error) (int64, int64) {
	var captureErr *Error
	if !errors.As(err, &captureErr) || captureErr.Metadata == nil {
		return 0, 0
	}
	return metadataInt64(captureErr.Metadata["limit_value"]), metadataInt64(captureErr.Metadata["actual_value"])
}

func metadataInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}
