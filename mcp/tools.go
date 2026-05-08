package mcp

import (
	"fmt"
	"strings"
	"time"

	pkgwebcap "github.com/goliatone/webcap"
)

type capturePageArguments struct {
	URL               string                  `json:"url"`
	OutputPath        string                  `json:"output_path,omitempty"`
	MetadataPath      string                  `json:"metadata_path,omitempty"`
	FullPage          bool                    `json:"full_page,omitempty"`
	Wait              string                  `json:"wait,omitempty"`
	WaitFor           string                  `json:"wait_for,omitempty"`
	JavaScript        string                  `json:"javascript,omitempty"`
	Viewport          pkgwebcap.Viewport      `json:"viewport"`
	ViewportPreset    string                  `json:"viewport_preset,omitempty"`
	DevicePreset      string                  `json:"device_preset,omitempty"`
	UserAgent         string                  `json:"user_agent,omitempty"`
	Readiness         pkgwebcap.ReadinessMode `json:"readiness,omitempty"`
	ReadinessIdle     string                  `json:"readiness_idle,omitempty"`
	DisableAnimations bool                    `json:"disable_animations,omitempty"`
	ReducedMotion     bool                    `json:"reduced_motion,omitempty"`
	WaitForFonts      bool                    `json:"wait_for_fonts,omitempty"`
	Timeout           string                  `json:"timeout,omitempty"`
	Selector          string                  `json:"selector,omitempty"`
	Selectors         []string                `json:"selectors,omitempty"`
	SelectorAll       string                  `json:"selector_all,omitempty"`
	SelectorsAll      []string                `json:"selectors_all,omitempty"`
}

type captureSectionArguments struct {
	URL               string                  `json:"url"`
	OutputPath        string                  `json:"output_path,omitempty"`
	MetadataPath      string                  `json:"metadata_path,omitempty"`
	Selector          string                  `json:"selector,omitempty"`
	Selectors         []string                `json:"selectors,omitempty"`
	SelectorAll       string                  `json:"selector_all,omitempty"`
	SelectorsAll      []string                `json:"selectors_all,omitempty"`
	Padding           int                     `json:"padding,omitempty"`
	Wait              string                  `json:"wait,omitempty"`
	WaitFor           string                  `json:"wait_for,omitempty"`
	JavaScript        string                  `json:"javascript,omitempty"`
	Viewport          pkgwebcap.Viewport      `json:"viewport"`
	ViewportPreset    string                  `json:"viewport_preset,omitempty"`
	DevicePreset      string                  `json:"device_preset,omitempty"`
	UserAgent         string                  `json:"user_agent,omitempty"`
	Readiness         pkgwebcap.ReadinessMode `json:"readiness,omitempty"`
	ReadinessIdle     string                  `json:"readiness_idle,omitempty"`
	DisableAnimations bool                    `json:"disable_animations,omitempty"`
	ReducedMotion     bool                    `json:"reduced_motion,omitempty"`
	WaitForFonts      bool                    `json:"wait_for_fonts,omitempty"`
	Timeout           string                  `json:"timeout,omitempty"`
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

func (a capturePageArguments) captureRequest() pkgwebcap.CaptureRequest {
	return pkgwebcap.CaptureRequest{
		URL:               strings.TrimSpace(a.URL),
		OutputPath:        strings.TrimSpace(a.OutputPath),
		MetadataPath:      strings.TrimSpace(a.MetadataPath),
		FullPage:          a.FullPage,
		Wait:              strings.TrimSpace(a.Wait),
		WaitFor:           strings.TrimSpace(a.WaitFor),
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
	return callToolResult{
		Content: []textContent{
			{Type: "text", Text: message},
		},
		StructuredContent: map[string]any{
			"tool":    strings.TrimSpace(toolName),
			"message": message,
		},
		IsError: true,
	}, nil
}

func (s *Server) tools() []tool {
	captureOutputSchema := captureToolOutputSchema()
	return []tool{
		capturePageTool(captureOutputSchema),
		captureSectionTool(captureOutputSchema),
		captureManifestTool(),
		compareImagesTool(),
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

func stringArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "string",
		},
	}
}
