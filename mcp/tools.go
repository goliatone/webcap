package mcp

import (
	"errors"
	"fmt"
	"strings"
	"time"

	pkgwebcap "github.com/goliatone/webcap"
)

type capturePageArguments struct {
	URL               string                       `json:"url"`
	OutputPath        string                       `json:"output_path,omitempty"`
	MetadataPath      string                       `json:"metadata_path,omitempty"`
	FullPage          bool                         `json:"full_page,omitempty"`
	Wait              string                       `json:"wait,omitempty"`
	WaitFor           string                       `json:"wait_for,omitempty"`
	WaitForFunction   string                       `json:"wait_for_function,omitempty"`
	JavaScript        string                       `json:"javascript,omitempty"`
	Viewport          pkgwebcap.Viewport           `json:"viewport"`
	ViewportPreset    string                       `json:"viewport_preset,omitempty"`
	DevicePreset      string                       `json:"device_preset,omitempty"`
	UserAgent         string                       `json:"user_agent,omitempty"`
	Readiness         pkgwebcap.ReadinessMode      `json:"readiness,omitempty"`
	ReadinessIdle     string                       `json:"readiness_idle,omitempty"`
	DisableAnimations bool                         `json:"disable_animations,omitempty"`
	ReducedMotion     bool                         `json:"reduced_motion,omitempty"`
	WaitForFonts      bool                         `json:"wait_for_fonts,omitempty"`
	Timeout           string                       `json:"timeout,omitempty"`
	Auth              pkgwebcap.CaptureAuth        `json:"auth,omitempty"`
	Guards            pkgwebcap.CaptureGuards      `json:"guards,omitempty"`
	Selector          string                       `json:"selector,omitempty"`
	Selectors         []string                     `json:"selectors,omitempty"`
	SelectorAll       string                       `json:"selector_all,omitempty"`
	SelectorsAll      []string                     `json:"selectors_all,omitempty"`
	OversizePolicy    pkgwebcap.OversizePolicy     `json:"oversize_policy,omitempty"`
	Tile              pkgwebcap.CaptureTileOptions `json:"tile"`
}

type captureSectionArguments struct {
	URL               string                       `json:"url"`
	OutputPath        string                       `json:"output_path,omitempty"`
	MetadataPath      string                       `json:"metadata_path,omitempty"`
	Selector          string                       `json:"selector,omitempty"`
	Selectors         []string                     `json:"selectors,omitempty"`
	SelectorAll       string                       `json:"selector_all,omitempty"`
	SelectorsAll      []string                     `json:"selectors_all,omitempty"`
	Padding           int                          `json:"padding,omitempty"`
	Wait              string                       `json:"wait,omitempty"`
	WaitFor           string                       `json:"wait_for,omitempty"`
	WaitForFunction   string                       `json:"wait_for_function,omitempty"`
	JavaScript        string                       `json:"javascript,omitempty"`
	Viewport          pkgwebcap.Viewport           `json:"viewport"`
	ViewportPreset    string                       `json:"viewport_preset,omitempty"`
	DevicePreset      string                       `json:"device_preset,omitempty"`
	UserAgent         string                       `json:"user_agent,omitempty"`
	Readiness         pkgwebcap.ReadinessMode      `json:"readiness,omitempty"`
	ReadinessIdle     string                       `json:"readiness_idle,omitempty"`
	DisableAnimations bool                         `json:"disable_animations,omitempty"`
	ReducedMotion     bool                         `json:"reduced_motion,omitempty"`
	WaitForFonts      bool                         `json:"wait_for_fonts,omitempty"`
	Timeout           string                       `json:"timeout,omitempty"`
	Auth              pkgwebcap.CaptureAuth        `json:"auth,omitempty"`
	Guards            pkgwebcap.CaptureGuards      `json:"guards,omitempty"`
	OversizePolicy    pkgwebcap.OversizePolicy     `json:"oversize_policy,omitempty"`
	Tile              pkgwebcap.CaptureTileOptions `json:"tile"`
}

type captureManifestArguments struct {
	ManifestPath string `json:"manifest_path"`
	OutputDir    string `json:"output_dir,omitempty"`
}

type compareImagesArguments struct {
	BasePath     string  `json:"base_path"`
	ComparePath  string  `json:"compare_path"`
	OutputPath   string  `json:"output_path,omitempty"`
	MetadataPath string  `json:"metadata_path,omitempty"`
	Threshold    float64 `json:"threshold,omitempty"`
}

type semanticDiffArguments struct {
	CurrentPath        string                     `json:"current_path"`
	ReferencePath      string                     `json:"reference_path"`
	Provider           string                     `json:"provider,omitempty"`
	Model              string                     `json:"model,omitempty"`
	Mode               pkgwebcap.SemanticDiffMode `json:"mode,omitempty"`
	Prompt             string                     `json:"prompt,omitempty"`
	PromptPath         string                     `json:"prompt_path,omitempty"`
	Focus              []string                   `json:"focus,omitempty"`
	MetadataPath       string                     `json:"metadata_path,omitempty"`
	Timeout            string                     `json:"timeout,omitempty"`
	MaxOutputTokens    int                        `json:"max_output_tokens,omitempty"`
	PixelDiffImagePath string                     `json:"pixel_diff_image_path,omitempty"`
	ChangedPixels      int                        `json:"changed_pixels,omitempty"`
	TotalPixels        int                        `json:"total_pixels,omitempty"`
	ChangedPercent     float64                    `json:"changed_percent,omitempty"`
	Threshold          float64                    `json:"threshold,omitempty"`
}

type captureToolResult struct {
	OutputPath   string                     `json:"output_path"`
	MetadataPath string                     `json:"metadata_path,omitempty"`
	ByteSize     int                        `json:"byte_size"`
	CapturedAt   time.Time                  `json:"captured_at"`
	Engine       string                     `json:"engine"`
	URL          string                     `json:"url"`
	Mode         pkgwebcap.CaptureMode      `json:"mode"`
	Selectors    []string                   `json:"selectors,omitempty"`
	MatchCount   int                        `json:"match_count,omitempty"`
	Bounds       *pkgwebcap.Bounds          `json:"bounds,omitempty"`
	Viewport     pkgwebcap.Viewport         `json:"viewport"`
	Browser      pkgwebcap.BrowserInfo      `json:"browser"`
	Warnings     []pkgwebcap.CaptureWarning `json:"warnings,omitempty"`
	Guards       []pkgwebcap.GuardOutcome   `json:"guards,omitempty"`
	Tiling       *pkgwebcap.CaptureTiling   `json:"tiling,omitempty"`
}

type batchToolResult struct {
	Count   int                 `json:"count"`
	Results []captureToolResult `json:"results"`
}

type diffToolResult struct {
	Mode         pkgwebcap.DiffMode    `json:"mode"`
	BasePath     string                `json:"base_path"`
	ComparePath  string                `json:"compare_path"`
	OutputPath   string                `json:"output_path"`
	MetadataPath string                `json:"metadata_path,omitempty"`
	Threshold    float64               `json:"threshold"`
	Summary      pkgwebcap.DiffSummary `json:"summary"`
	Entry        *pkgwebcap.DiffEntry  `json:"entry,omitempty"`
	Entries      []pkgwebcap.DiffEntry `json:"entries,omitempty"`
	CreatedAt    time.Time             `json:"created_at"`
}

type semanticDiffToolResult struct {
	CurrentPath     string                         `json:"current_path"`
	ReferencePath   string                         `json:"reference_path"`
	Provider        string                         `json:"provider"`
	Model           string                         `json:"model,omitempty"`
	Summary         string                         `json:"summary"`
	Verdict         pkgwebcap.SemanticDiffVerdict  `json:"verdict"`
	Severity        pkgwebcap.SemanticDiffSeverity `json:"severity"`
	Differences     []pkgwebcap.SemanticDifference `json:"differences,omitempty"`
	MetadataPath    string                         `json:"metadata_path,omitempty"`
	RawResponsePath string                         `json:"raw_response_path,omitempty"`
	Warnings        []pkgwebcap.CaptureWarning     `json:"warnings,omitempty"`
}

func (a capturePageArguments) captureRequest() pkgwebcap.CaptureRequest {
	return pkgwebcap.CaptureRequest{
		URL:               strings.TrimSpace(a.URL),
		OutputPath:        strings.TrimSpace(a.OutputPath),
		MetadataPath:      strings.TrimSpace(a.MetadataPath),
		FullPage:          a.FullPage,
		Wait:              strings.TrimSpace(a.Wait),
		WaitFor:           strings.TrimSpace(a.WaitFor),
		WaitForFunction:   strings.TrimSpace(a.WaitForFunction),
		JavaScript:        strings.TrimSpace(a.JavaScript),
		Viewport:          a.Viewport,
		ViewportPreset:    strings.TrimSpace(a.ViewportPreset),
		DevicePreset:      strings.TrimSpace(a.DevicePreset),
		UserAgent:         strings.TrimSpace(a.UserAgent),
		Readiness:         a.Readiness,
		ReadinessIdle:     strings.TrimSpace(a.ReadinessIdle),
		DisableAnimations: a.DisableAnimations,
		ReducedMotion:     a.ReducedMotion,
		WaitForFonts:      a.WaitForFonts,
		Timeout:           strings.TrimSpace(a.Timeout),
		Auth:              a.Auth,
		Guards:            a.Guards,
		OversizePolicy:    a.OversizePolicy,
		Tile:              a.Tile,
	}
}

func (a captureSectionArguments) captureRequest() pkgwebcap.CaptureRequest {
	return pkgwebcap.CaptureRequest{
		URL:               strings.TrimSpace(a.URL),
		OutputPath:        strings.TrimSpace(a.OutputPath),
		MetadataPath:      strings.TrimSpace(a.MetadataPath),
		Selector:          strings.TrimSpace(a.Selector),
		Selectors:         append([]string(nil), a.Selectors...),
		SelectorAll:       strings.TrimSpace(a.SelectorAll),
		SelectorsAll:      append([]string(nil), a.SelectorsAll...),
		Padding:           a.Padding,
		Wait:              strings.TrimSpace(a.Wait),
		WaitFor:           strings.TrimSpace(a.WaitFor),
		WaitForFunction:   strings.TrimSpace(a.WaitForFunction),
		JavaScript:        strings.TrimSpace(a.JavaScript),
		Viewport:          a.Viewport,
		ViewportPreset:    strings.TrimSpace(a.ViewportPreset),
		DevicePreset:      strings.TrimSpace(a.DevicePreset),
		UserAgent:         strings.TrimSpace(a.UserAgent),
		Readiness:         a.Readiness,
		ReadinessIdle:     strings.TrimSpace(a.ReadinessIdle),
		DisableAnimations: a.DisableAnimations,
		ReducedMotion:     a.ReducedMotion,
		WaitForFonts:      a.WaitForFonts,
		Timeout:           strings.TrimSpace(a.Timeout),
		Auth:              a.Auth,
		Guards:            a.Guards,
		OversizePolicy:    a.OversizePolicy,
		Tile:              a.Tile,
	}
}

func (a compareImagesArguments) diffRequest() pkgwebcap.DiffRequest {
	return pkgwebcap.DiffRequest{
		BasePath:     strings.TrimSpace(a.BasePath),
		ComparePath:  strings.TrimSpace(a.ComparePath),
		OutputPath:   strings.TrimSpace(a.OutputPath),
		MetadataPath: strings.TrimSpace(a.MetadataPath),
		Threshold:    a.Threshold,
	}
}

func (a semanticDiffArguments) semanticDiffRequest() pkgwebcap.SemanticDiffRequest {
	return pkgwebcap.SemanticDiffRequest{
		CurrentPath:     strings.TrimSpace(a.CurrentPath),
		ReferencePath:   strings.TrimSpace(a.ReferencePath),
		Provider:        strings.TrimSpace(a.Provider),
		Model:           strings.TrimSpace(a.Model),
		Mode:            a.Mode,
		Prompt:          strings.TrimSpace(a.Prompt),
		PromptPath:      strings.TrimSpace(a.PromptPath),
		Focus:           append([]string(nil), a.Focus...),
		MetadataPath:    strings.TrimSpace(a.MetadataPath),
		Timeout:         strings.TrimSpace(a.Timeout),
		MaxOutputTokens: a.MaxOutputTokens,
		PixelContext: pkgwebcap.SemanticPixelContext{
			PixelDiffImagePath: strings.TrimSpace(a.PixelDiffImagePath),
			ChangedPixels:      a.ChangedPixels,
			TotalPixels:        a.TotalPixels,
			ChangedPercent:     a.ChangedPercent,
			Threshold:          a.Threshold,
		},
	}
}

func hasSectionSelectors(selector string, selectors []string, selectorAll string, selectorsAll []string) bool {
	return strings.TrimSpace(selector) != "" || len(selectors) > 0 || strings.TrimSpace(selectorAll) != "" || len(selectorsAll) > 0
}

func summarizeCaptureResult(result pkgwebcap.CaptureResult) captureToolResult {
	return captureToolResult{
		OutputPath:   result.OutputPath,
		MetadataPath: result.MetadataPath,
		ByteSize:     result.ByteSize,
		CapturedAt:   result.CapturedAt,
		Engine:       result.Engine,
		URL:          result.Artifact.URL,
		Mode:         result.Artifact.Mode,
		Selectors:    append([]string(nil), result.Artifact.Selectors...),
		MatchCount:   result.Artifact.MatchCount,
		Bounds:       result.Artifact.Bounds,
		Viewport:     result.Artifact.Viewport,
		Browser:      result.Browser,
		Warnings:     append([]pkgwebcap.CaptureWarning(nil), result.Warnings...),
		Guards:       append([]pkgwebcap.GuardOutcome(nil), result.Guards...),
		Tiling:       result.Tiling,
	}
}

func summarizeBatchResult(result pkgwebcap.BatchResult) batchToolResult {
	items := make([]captureToolResult, 0, len(result.Results))
	for _, item := range result.Results {
		items = append(items, summarizeCaptureResult(item))
	}
	return batchToolResult{
		Count:   len(items),
		Results: items,
	}
}

func summarizeDiffResult(result pkgwebcap.DiffResult) diffToolResult {
	summary := diffToolResult{
		Mode:         result.Mode,
		BasePath:     result.BasePath,
		ComparePath:  result.ComparePath,
		OutputPath:   result.OutputPath,
		MetadataPath: result.MetadataPath,
		Threshold:    result.Threshold,
		Summary:      result.Summary,
		CreatedAt:    result.CreatedAt,
	}
	if result.Entry != nil {
		entry := *result.Entry
		summary.Entry = &entry
	}
	if len(result.Entries) > 0 {
		summary.Entries = append([]pkgwebcap.DiffEntry(nil), result.Entries...)
	}
	return summary
}

func summarizeSemanticDiffResult(result pkgwebcap.SemanticDiffResult) semanticDiffToolResult {
	return semanticDiffToolResult{
		CurrentPath:     result.CurrentPath,
		ReferencePath:   result.ReferencePath,
		Provider:        result.Provider,
		Model:           result.Model,
		Summary:         result.Summary,
		Verdict:         result.Verdict,
		Severity:        result.Severity,
		Differences:     append([]pkgwebcap.SemanticDifference(nil), result.Differences...),
		MetadataPath:    result.MetadataPath,
		RawResponsePath: result.RawResponsePath,
		Warnings:        append([]pkgwebcap.CaptureWarning(nil), result.Warnings...),
	}
}

func successToolResult(structured any, text string) callToolResult {
	return callToolResult{
		Content: []textContent{
			{Type: "text", Text: strings.TrimSpace(text)},
		},
		StructuredContent: structured,
	}
}

func errorToolResult(toolName string, err error) (callToolResult, error) {
	message := strings.TrimSpace(fmt.Sprintf("%v", err))
	if message == "" {
		message = fmt.Sprintf("%s failed", strings.TrimSpace(toolName))
	}
	structured := map[string]any{
		"tool":    strings.TrimSpace(toolName),
		"message": message,
	}
	var partialErr *pkgwebcap.PartialCaptureError
	if errors.As(err, &partialErr) {
		structured["code"] = string(pkgwebcap.CodePartialCapture)
		structured["failed_tile_index"] = partialErr.FailedTileIndex
		structured["completed_count"] = partialErr.CompletedCount
		structured["total_count"] = partialErr.TotalCount
		if partialErr.Result != nil {
			structured["result"] = summarizeCaptureResult(*partialErr.Result)
		}
	}
	var captureErr *pkgwebcap.Error
	if partialErr == nil && errors.As(err, &captureErr) {
		structured["code"] = string(captureErr.Code)
		structured["operation"] = captureErr.Operation
		if len(captureErr.Metadata) > 0 {
			structured["metadata"] = captureErr.Metadata
		}
	}
	return callToolResult{
		Content: []textContent{
			{Type: "text", Text: message},
		},
		StructuredContent: structured,
		IsError:           true,
	}, nil
}

func (s *Server) tools() []tool {
	captureOutputSchema := captureToolOutputSchema()
	return []tool{
		capturePageTool(captureOutputSchema),
		captureSectionTool(captureOutputSchema),
		captureManifestTool(),
		compareImagesTool(),
		semanticDiffTool(),
	}
}

func captureToolOutputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"output_path":   map[string]any{"type": "string"},
			"metadata_path": map[string]any{"type": "string"},
			"mode":          map[string]any{"type": "string"},
			"url":           map[string]any{"type": "string"},
			"engine":        map[string]any{"type": "string"},
			"guards":        map[string]any{"type": "array"},
			"tiling":        map[string]any{"type": "object"},
		},
		"required": []string{"output_path", "mode", "url", "engine"},
	}
}

func diffToolOutputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode":         map[string]any{"type": "string"},
			"output_path":  map[string]any{"type": "string"},
			"base_path":    map[string]any{"type": "string"},
			"compare_path": map[string]any{"type": "string"},
		},
		"required": []string{"mode", "output_path", "base_path", "compare_path"},
	}
}

func semanticDiffToolOutputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"current_path":   map[string]any{"type": "string"},
			"reference_path": map[string]any{"type": "string"},
			"provider":       map[string]any{"type": "string"},
			"summary":        map[string]any{"type": "string"},
			"verdict":        map[string]any{"type": "string"},
			"severity":       map[string]any{"type": "string"},
			"metadata_path":  map[string]any{"type": "string"},
		},
		"required": []string{"current_path", "reference_path", "provider", "summary", "verdict", "severity"},
	}
}

func capturePageTool(outputSchema map[string]any) tool {
	return tool{
		Name:        "capture_page",
		Title:       "Capture Page",
		Description: "Capture a viewport or full-page screenshot and persist the image plus metadata sidecar to disk.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":                map[string]any{"type": "string"},
				"output_path":        map[string]any{"type": "string"},
				"metadata_path":      map[string]any{"type": "string"},
				"full_page":          map[string]any{"type": "boolean"},
				"wait":               map[string]any{"type": "string"},
				"wait_for":           map[string]any{"type": "string"},
				"wait_for_function":  map[string]any{"type": "string"},
				"javascript":         map[string]any{"type": "string"},
				"viewport":           viewportSchema(),
				"viewport_preset":    map[string]any{"type": "string"},
				"device_preset":      map[string]any{"type": "string"},
				"user_agent":         map[string]any{"type": "string"},
				"readiness":          map[string]any{"type": "string"},
				"readiness_idle":     map[string]any{"type": "string"},
				"disable_animations": map[string]any{"type": "boolean"},
				"reduced_motion":     map[string]any{"type": "boolean"},
				"wait_for_fonts":     map[string]any{"type": "boolean"},
				"timeout":            map[string]any{"type": "string"},
				"auth":               authSchema(),
				"guards":             guardsSchema(),
				"oversize_policy":    map[string]any{"type": "string", "enum": []string{"fail", "tile"}},
				"tile":               tileSchema(),
			},
			"required": []string{"url"},
		},
		OutputSchema: outputSchema,
		Annotations:  toolAnnotations("Capture Page", true),
	}
}

func captureSectionTool(outputSchema map[string]any) tool {
	return tool{
		Name:        "capture_section",
		Title:       "Capture Section",
		Description: "Capture a screenshot clipped to a selector target or shot-scraper style selector union.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":                map[string]any{"type": "string"},
				"output_path":        map[string]any{"type": "string"},
				"metadata_path":      map[string]any{"type": "string"},
				"selector":           map[string]any{"type": "string"},
				"selectors":          stringArraySchema(),
				"selector_all":       map[string]any{"type": "string"},
				"selectors_all":      stringArraySchema(),
				"padding":            map[string]any{"type": "integer"},
				"wait":               map[string]any{"type": "string"},
				"wait_for":           map[string]any{"type": "string"},
				"wait_for_function":  map[string]any{"type": "string"},
				"javascript":         map[string]any{"type": "string"},
				"viewport":           viewportSchema(),
				"viewport_preset":    map[string]any{"type": "string"},
				"device_preset":      map[string]any{"type": "string"},
				"user_agent":         map[string]any{"type": "string"},
				"readiness":          map[string]any{"type": "string"},
				"readiness_idle":     map[string]any{"type": "string"},
				"disable_animations": map[string]any{"type": "boolean"},
				"reduced_motion":     map[string]any{"type": "boolean"},
				"wait_for_fonts":     map[string]any{"type": "boolean"},
				"timeout":            map[string]any{"type": "string"},
				"auth":               authSchema(),
				"guards":             guardsSchema(),
				"oversize_policy":    map[string]any{"type": "string", "enum": []string{"fail", "tile"}},
				"tile":               tileSchema(),
			},
			"required": []string{"url"},
		},
		OutputSchema: outputSchema,
		Annotations:  toolAnnotations("Capture Section", true),
	}
}

func captureManifestTool() tool {
	return tool{
		Name:        "capture_manifest",
		Title:       "Capture Manifest",
		Description: "Run a YAML or JSON manifest and persist all capture artifacts to disk.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"manifest_path": map[string]any{"type": "string"},
				"output_dir":    map[string]any{"type": "string"},
			},
			"required": []string{"manifest_path"},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"count": map[string]any{"type": "integer"},
			},
			"required": []string{"count"},
		},
		Annotations: toolAnnotations("Capture Manifest", true),
	}
}

func compareImagesTool() tool {
	return tool{
		Name:        "compare_images",
		Title:       "Compare Images",
		Description: "Compare two screenshots or two directories of screenshots and write diff artifacts to disk.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"base_path":     map[string]any{"type": "string"},
				"compare_path":  map[string]any{"type": "string"},
				"output_path":   map[string]any{"type": "string"},
				"metadata_path": map[string]any{"type": "string"},
				"threshold":     map[string]any{"type": "number"},
			},
			"required": []string{"base_path", "compare_path"},
		},
		OutputSchema: diffToolOutputSchema(),
		Annotations:  toolAnnotations("Compare Images", false),
	}
}

func semanticDiffTool() tool {
	return tool{
		Name:        "semantic_diff",
		Title:       "Semantic Diff",
		Description: "Compare two screenshots with a configured vision LLM provider and return compact semantic findings.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"current_path":          map[string]any{"type": "string"},
				"reference_path":        map[string]any{"type": "string"},
				"provider":              map[string]any{"type": "string"},
				"model":                 map[string]any{"type": "string"},
				"mode":                  map[string]any{"type": "string"},
				"prompt":                map[string]any{"type": "string"},
				"prompt_path":           map[string]any{"type": "string"},
				"focus":                 stringArraySchema(),
				"metadata_path":         map[string]any{"type": "string"},
				"timeout":               map[string]any{"type": "string"},
				"max_output_tokens":     map[string]any{"type": "integer"},
				"pixel_diff_image_path": map[string]any{"type": "string"},
				"changed_pixels":        map[string]any{"type": "integer"},
				"total_pixels":          map[string]any{"type": "integer"},
				"changed_percent":       map[string]any{"type": "number"},
				"threshold":             map[string]any{"type": "number"},
			},
			"required": []string{"current_path", "reference_path", "provider"},
		},
		OutputSchema: semanticDiffToolOutputSchema(),
		Annotations:  toolAnnotations("Semantic Diff", true),
	}
}

func toolAnnotations(title string, openWorld bool) map[string]any {
	return map[string]any{
		"title":           title,
		"readOnlyHint":    false,
		"destructiveHint": false,
		"openWorldHint":   openWorld,
	}
}

func viewportSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"width":        map[string]any{"type": "integer"},
			"height":       map[string]any{"type": "integer"},
			"scale_factor": map[string]any{"type": "number"},
			"mobile":       map[string]any{"type": "boolean"},
		},
	}
}

func tileSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"max_width":           map[string]any{"type": "integer"},
			"max_height":          map[string]any{"type": "integer"},
			"max_pixels":          map[string]any{"type": "integer"},
			"max_target_pixels":   map[string]any{"type": "integer"},
			"overlap":             map[string]any{"type": "integer"},
			"stitch":              map[string]any{"type": "boolean"},
			"max_stitched_pixels": map[string]any{"type": "integer"},
			"cleanup_tiles":       map[string]any{"type": "boolean"},
		},
	}
}

func authSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cookie_jar":    map[string]any{"type": "string"},
			"storage_state": map[string]any{"type": "string"},
			"headers": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":  map[string]any{"type": "string"},
						"value": map[string]any{"type": "string"},
					},
					"required": []string{"name", "value"},
				},
			},
			"cookies": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":     map[string]any{"type": "string"},
						"value":    map[string]any{"type": "string"},
						"domain":   map[string]any{"type": "string"},
						"path":     map[string]any{"type": "string"},
						"secure":   map[string]any{"type": "boolean"},
						"httpOnly": map[string]any{"type": "boolean"},
						"sameSite": map[string]any{"type": "string"},
						"expires":  map[string]any{"type": "integer"},
					},
					"required": []string{"name", "value"},
				},
			},
		},
	}
}

func guardsSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"expect_url":       map[string]any{"type": "string"},
			"fail_on_url":      stringArraySchema(),
			"fail_on_selector": stringArraySchema(),
		},
	}
}

func stringArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "string",
		},
	}
}
