package webcap

import (
	"context"
	"os"
	"strings"
	"time"
)

func (s *Service) SemanticDiff(ctx context.Context, req SemanticDiffRequest) (SemanticDiffResult, error) {
	if s == nil {
		return SemanticDiffResult{}, newCaptureError(CodeCapture, "semantic_diff", "webcap service is not configured", nil)
	}
	normalized, err := NormalizeSemanticDiffRequest(req)
	if err != nil {
		return SemanticDiffResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return SemanticDiffResult{}, wrapCaptureError("semantic_diff", err)
	}

	options := s.semanticDiff.normalized()
	if normalized.Provider == "" {
		normalized.Provider = options.DefaultProvider
	}
	if normalized.Provider == "" {
		return SemanticDiffResult{}, newCaptureError(CodeValidation, "semantic_diff", "semantic diff provider is required", nil)
	}
	provider := options.Providers[normalized.Provider]
	if provider == nil {
		return SemanticDiffResult{}, newCaptureError(CodeValidation, "semantic_diff", "unsupported semantic diff provider "+normalized.Provider, nil)
	}
	if normalized.Model == "" {
		normalized.Model = strings.TrimSpace(options.DefaultModels[normalized.Provider])
	}
	if normalized.Model == "" && (normalized.Provider == "openai" || normalized.Provider == "anthropic") {
		return SemanticDiffResult{}, newCaptureError(CodeValidation, "semantic_diff", "semantic diff model is required for built-in providers", nil)
	}
	if normalized.MaxOutputTokens == 0 {
		normalized.MaxOutputTokens = options.MaxOutputTokens
	}
	if normalized.MetadataPath == "" {
		normalized.MetadataPath = defaultSemanticMetadataPath(normalized)
	}
	persistRaw := normalized.PersistRawResponse || options.PersistRawResponses
	if persistRaw && normalized.RawResponsePath == "" {
		normalized.RawResponsePath = normalized.MetadataPath + ".raw.txt"
	}
	tempDir := ""
	if options.ResizeImages {
		var err error
		tempDir, err = os.MkdirTemp("", "webcap-semantic-*")
		if err != nil {
			return SemanticDiffResult{}, wrapCaptureError("prepare_semantic_image", err)
		}
		defer func() {
			_ = os.RemoveAll(tempDir)
		}()
	}
	imageOptions := semanticImagePrepareOptions{
		MaxRawBytes:      options.MaxImageBytes,
		MaxProviderBytes: options.MaxProviderImageBytes,
		MaxLongEdge:      options.MaxImageLongEdge,
		MaxPixels:        options.MaxImagePixels,
		MaxEncodedBytes:  options.MaxEncodedImageBytes,
		ResizeImages:     options.ResizeImages,
		TempDir:          tempDir,
	}

	prompt, promptMetadata, err := buildSemanticDiffPrompt(normalized)
	if err != nil {
		return SemanticDiffResult{}, err
	}

	currentPath, err := applySemanticImageRedactor(ctx, options.RedactImage, "current", normalized.CurrentPath)
	if err != nil {
		return SemanticDiffResult{}, err
	}
	referencePath, err := applySemanticImageRedactor(ctx, options.RedactImage, "reference", normalized.ReferencePath)
	if err != nil {
		return SemanticDiffResult{}, err
	}
	currentImage, err := prepareSemanticImagePayload(currentPath, "current", imageOptions)
	if err != nil {
		return SemanticDiffResult{}, err
	}
	referenceImage, err := prepareSemanticImagePayload(referencePath, "reference", imageOptions)
	if err != nil {
		return SemanticDiffResult{}, err
	}
	images := []SemanticImagePayload{currentImage, referenceImage}
	if normalized.PixelContext.PixelDiffImagePath != "" {
		pixelDiffPath, err := applySemanticImageRedactor(ctx, options.RedactImage, "pixel_diff", normalized.PixelContext.PixelDiffImagePath)
		if err != nil {
			return SemanticDiffResult{}, err
		}
		diffImage, err := prepareSemanticImagePayload(pixelDiffPath, "pixel_diff", imageOptions)
		if err != nil {
			return SemanticDiffResult{}, err
		}
		images = append(images, diffImage)
	}
	images, err = satisfySemanticProviderBudgets(images, semanticProviderBudgetOptions{
		Image:                        imageOptions,
		Provider:                     normalized.Provider,
		Model:                        normalized.Model,
		Prompt:                       prompt,
		MaxOutputTokens:              normalized.MaxOutputTokens,
		StructuredJSON:               true,
		MaxEncodedImageBytes:         options.MaxEncodedImageBytes,
		MaxCombinedEncodedImageBytes: options.MaxCombinedEncodedImageBytes,
		MaxRequestBodyBytes:          options.MaxRequestBodyBytes,
	})
	if err != nil {
		return SemanticDiffResult{}, err
	}

	timeout := normalized.timeout(options.DefaultTimeout)
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startedAt := s.now()
	providerResponse, err := provider.CompareImages(callCtx, SemanticProviderRequest{
		Provider:            normalized.Provider,
		Model:               normalized.Model,
		Prompt:              prompt,
		Images:              images,
		Timeout:             timeout,
		MaxOutputTokens:     normalized.MaxOutputTokens,
		StructuredJSON:      true,
		MaxRequestBodyBytes: options.MaxRequestBodyBytes,
	})
	if err != nil {
		return SemanticDiffResult{}, wrapCaptureError("semantic_diff_provider", err)
	}
	completedAt := s.now()
	if completedAt.Before(startedAt) {
		completedAt = time.Now().UTC()
	}

	parsed, parseWarnings := parseSemanticProviderResponse(providerResponse.RawText)
	result := SemanticDiffResult{
		CurrentPath:      normalized.CurrentPath,
		ReferencePath:    normalized.ReferencePath,
		Provider:         firstNonEmpty(providerResponse.Provider, normalized.Provider),
		Model:            firstNonEmpty(providerResponse.Model, normalized.Model),
		Summary:          parsed.Summary,
		Differences:      parsed.Differences,
		Verdict:          parsed.Verdict,
		Severity:         parsed.Severity,
		Prompt:           promptMetadata,
		PixelContext:     normalized.PixelContext,
		MetadataPath:     normalized.MetadataPath,
		Warnings:         append(append([]CaptureWarning(nil), providerResponse.Warnings...), parseWarnings...),
		ImageMetadata:    semanticImageMetadata(images),
		ProviderMetadata: cloneStringMap(providerResponse.Metadata),
		Usage:            providerResponse.Usage,
		StartedAt:        startedAt,
		CompletedAt:      completedAt,
		Duration:         completedAt.Sub(startedAt).String(),
	}
	if persistRaw {
		result.RawResponse = providerResponse.RawText
		result.RawResponsePath = normalized.RawResponsePath
		if err := writeFile(result.RawResponsePath, []byte(providerResponse.RawText+"\n")); err != nil {
			return SemanticDiffResult{}, wrapCaptureError("write_semantic_raw_response", err)
		}
	}
	if result.MetadataPath != "" {
		if err := writeDiffMetadata(result.MetadataPath, result); err != nil {
			return SemanticDiffResult{}, wrapCaptureError("write_semantic_metadata", err)
		}
	}
	return result, nil
}

func applySemanticImageRedactor(ctx context.Context, redactor SemanticImageRedactor, role, path string) (string, error) {
	if redactor == nil {
		return path, nil
	}
	redacted, err := redactor(ctx, SemanticImageRedactionInput{Role: role, Path: path})
	if err != nil {
		return "", wrapCaptureError("semantic_image_redaction", err)
	}
	redacted = strings.TrimSpace(redacted)
	if redacted == "" {
		return "", newCaptureError(CodeValidation, "semantic_image_redaction", "semantic image redactor returned an empty path", nil)
	}
	return redacted, nil
}

func semanticPixelContextFromDiffEntry(entry DiffEntry) SemanticPixelContext {
	return SemanticPixelContext{
		PixelDiffImagePath: entry.OutputPath,
		ChangedPixels:      entry.ChangedPixels,
		TotalPixels:        entry.TotalPixels,
		ChangedPercent:     entry.ChangedPercent,
		Threshold:          entry.Threshold,
		Warnings:           append([]CaptureWarning(nil), entry.Warnings...),
	}
}
