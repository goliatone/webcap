package webcap

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultViewportWidth   = 1440
	defaultViewportHeight  = 1200
	defaultScaleFactor     = 1.0
	defaultTimeout         = 30 * time.Second
	defaultImageFormat     = "png"
	defaultReadinessIdle   = 500 * time.Millisecond
	defaultReadinessMode   = ReadinessComplete
	defaultOutputDirectory = "webcap-output"

	DefaultOversizePolicy      = OversizePolicyFail
	DefaultTileMaxWidth        = 8192
	DefaultTileMaxHeight       = 8192
	DefaultTileMaxPixels       = 67108864
	DefaultTileMaxTargetPixels = 536870912
	DefaultTileMaxStitchPixels = 268435456
	DefaultTileOverlap         = 0
)

type Viewport struct {
	Width       int     `json:"width" yaml:"width"`
	Height      int     `json:"height" yaml:"height"`
	ScaleFactor float64 `json:"scale_factor,omitempty" yaml:"scale_factor,omitempty"`
	Mobile      bool    `json:"mobile,omitempty" yaml:"mobile,omitempty"`
}

type ReadinessMode string

const (
	ReadinessNone        ReadinessMode = "none"
	ReadinessInteractive ReadinessMode = "interactive"
	ReadinessComplete    ReadinessMode = "complete"
	ReadinessNetworkIdle ReadinessMode = "network_idle"
)

type CaptureRequest struct {
	URL               string             `json:"url" yaml:"url"`
	OutputPath        string             `json:"output,omitempty" yaml:"output,omitempty"`
	MetadataPath      string             `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	FullPage          bool               `json:"full_page,omitempty" yaml:"full_page,omitempty"`
	Selector          string             `json:"selector,omitempty" yaml:"selector,omitempty"`
	Selectors         []string           `json:"selectors,omitempty" yaml:"selectors,omitempty"`
	SelectorAll       string             `json:"selector_all,omitempty" yaml:"selector_all,omitempty"`
	SelectorsAll      []string           `json:"selectors_all,omitempty" yaml:"selectors_all,omitempty"`
	Padding           int                `json:"padding,omitempty" yaml:"padding,omitempty"`
	Wait              string             `json:"wait,omitempty" yaml:"wait,omitempty"`
	WaitFor           string             `json:"wait_for,omitempty" yaml:"wait_for,omitempty"`
	JavaScript        string             `json:"javascript,omitempty" yaml:"javascript,omitempty"`
	Viewport          Viewport           `json:"viewport" yaml:"viewport,omitempty"`
	ViewportPreset    string             `json:"viewport_preset,omitempty" yaml:"viewport_preset,omitempty"`
	DevicePreset      string             `json:"device_preset,omitempty" yaml:"device_preset,omitempty"`
	UserAgent         string             `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
	Readiness         ReadinessMode      `json:"readiness,omitempty" yaml:"readiness,omitempty"`
	ReadinessIdle     string             `json:"readiness_idle,omitempty" yaml:"readiness_idle,omitempty"`
	DisableAnimations bool               `json:"disable_animations,omitempty" yaml:"disable_animations,omitempty"`
	ReducedMotion     bool               `json:"reduced_motion,omitempty" yaml:"reduced_motion,omitempty"`
	WaitForFonts      bool               `json:"wait_for_fonts,omitempty" yaml:"wait_for_fonts,omitempty"`
	BeforeNavigateJS  string             `json:"before_navigate_js,omitempty" yaml:"before_navigate_js,omitempty"`
	AfterNavigateJS   string             `json:"after_navigate_js,omitempty" yaml:"after_navigate_js,omitempty"`
	BeforeCaptureJS   string             `json:"before_capture_js,omitempty" yaml:"before_capture_js,omitempty"`
	Timeout           string             `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	OversizePolicy    OversizePolicy     `json:"oversize_policy,omitempty" yaml:"oversize_policy,omitempty"`
	Tile              CaptureTileOptions `json:"tile,omitempty,omitzero" yaml:"tile,omitempty"`
}

type OversizePolicy string

const (
	OversizePolicyFail OversizePolicy = "fail"
	OversizePolicyTile OversizePolicy = "tile"
)

type CaptureTileOptions struct {
	MaxWidth          int   `json:"max_width,omitempty" yaml:"max_width,omitempty"`
	MaxHeight         int   `json:"max_height,omitempty" yaml:"max_height,omitempty"`
	MaxPixels         int64 `json:"max_pixels,omitempty" yaml:"max_pixels,omitempty"`
	MaxTargetPixels   int64 `json:"max_target_pixels,omitempty" yaml:"max_target_pixels,omitempty"`
	Overlap           int   `json:"overlap,omitempty" yaml:"overlap,omitempty"`
	Stitch            bool  `json:"stitch,omitempty" yaml:"stitch,omitempty"`
	MaxStitchedPixels int64 `json:"max_stitched_pixels,omitempty" yaml:"max_stitched_pixels,omitempty"`
	CleanupTiles      bool  `json:"cleanup_tiles,omitempty" yaml:"cleanup_tiles,omitempty"`
	set               map[string]bool
}

func (o CaptureTileOptions) IsZero() bool {
	return o.MaxWidth == 0 &&
		o.MaxHeight == 0 &&
		o.MaxPixels == 0 &&
		o.MaxTargetPixels == 0 &&
		o.Overlap == 0 &&
		!o.Stitch &&
		o.MaxStitchedPixels == 0 &&
		!o.CleanupTiles
}

func (o CaptureTileOptions) fieldSet(name string) bool {
	return o.set != nil && o.set[name]
}

func (o *CaptureTileOptions) UnmarshalYAML(value *yaml.Node) error {
	type rawTileOptions struct {
		MaxWidth          *int   `yaml:"max_width"`
		MaxHeight         *int   `yaml:"max_height"`
		MaxPixels         *int64 `yaml:"max_pixels"`
		MaxTargetPixels   *int64 `yaml:"max_target_pixels"`
		Overlap           *int   `yaml:"overlap"`
		Stitch            *bool  `yaml:"stitch"`
		MaxStitchedPixels *int64 `yaml:"max_stitched_pixels"`
		CleanupTiles      *bool  `yaml:"cleanup_tiles"`
	}
	var raw rawTileOptions
	if err := value.Decode(&raw); err != nil {
		return err
	}
	applyRawTileOptions(o, raw.MaxWidth, raw.MaxHeight, raw.MaxPixels, raw.MaxTargetPixels, raw.Overlap, raw.Stitch, raw.MaxStitchedPixels, raw.CleanupTiles)
	return nil
}

func (o *CaptureTileOptions) UnmarshalJSON(payload []byte) error {
	type rawTileOptions struct {
		MaxWidth          *int   `json:"max_width"`
		MaxHeight         *int   `json:"max_height"`
		MaxPixels         *int64 `json:"max_pixels"`
		MaxTargetPixels   *int64 `json:"max_target_pixels"`
		Overlap           *int   `json:"overlap"`
		Stitch            *bool  `json:"stitch"`
		MaxStitchedPixels *int64 `json:"max_stitched_pixels"`
		CleanupTiles      *bool  `json:"cleanup_tiles"`
	}
	var raw rawTileOptions
	if err := json.Unmarshal(payload, &raw); err != nil {
		return err
	}
	applyRawTileOptions(o, raw.MaxWidth, raw.MaxHeight, raw.MaxPixels, raw.MaxTargetPixels, raw.Overlap, raw.Stitch, raw.MaxStitchedPixels, raw.CleanupTiles)
	return nil
}

func applyRawTileOptions(o *CaptureTileOptions, maxWidth, maxHeight *int, maxPixels, maxTargetPixels *int64, overlap *int, stitch *bool, maxStitchedPixels *int64, cleanupTiles *bool) {
	if o == nil {
		return
	}
	o.set = map[string]bool{}
	if maxWidth != nil {
		o.MaxWidth = *maxWidth
		o.set["max_width"] = true
	}
	if maxHeight != nil {
		o.MaxHeight = *maxHeight
		o.set["max_height"] = true
	}
	if maxPixels != nil {
		o.MaxPixels = *maxPixels
		o.set["max_pixels"] = true
	}
	if maxTargetPixels != nil {
		o.MaxTargetPixels = *maxTargetPixels
		o.set["max_target_pixels"] = true
	}
	if overlap != nil {
		o.Overlap = *overlap
		o.set["overlap"] = true
	}
	if stitch != nil {
		o.Stitch = *stitch
		o.set["stitch"] = true
	}
	if maxStitchedPixels != nil {
		o.MaxStitchedPixels = *maxStitchedPixels
		o.set["max_stitched_pixels"] = true
	}
	if cleanupTiles != nil {
		o.CleanupTiles = *cleanupTiles
		o.set["cleanup_tiles"] = true
	}
}

type CaptureMode string

const (
	CaptureModeViewport     CaptureMode = "viewport"
	CaptureModeFullPage     CaptureMode = "full_page"
	CaptureModeSelector     CaptureMode = "selector"
	CaptureModeSelectors    CaptureMode = "selectors"
	CaptureModeSelectorAll  CaptureMode = "selector_all"
	CaptureModeSelectorsAll CaptureMode = "selectors_all"
)

type Bounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type CaptureTileStatus string

const (
	CaptureTilePending   CaptureTileStatus = "pending"
	CaptureTileCompleted CaptureTileStatus = "completed"
	CaptureTileFailed    CaptureTileStatus = "failed"
)

type CaptureTilingStatus string

const (
	CaptureTilingComplete CaptureTilingStatus = "complete"
	CaptureTilingPartial  CaptureTilingStatus = "partial"
	CaptureTilingFailed   CaptureTilingStatus = "failed"
)

type CaptureTileLimits struct {
	MaxWidth          int     `json:"max_width"`
	MaxHeight         int     `json:"max_height"`
	MaxPixels         int64   `json:"max_pixels"`
	MaxTargetPixels   int64   `json:"max_target_pixels"`
	MaxStitchedPixels int64   `json:"max_stitched_pixels"`
	Overlap           int     `json:"overlap"`
	ScaleFactor       float64 `json:"scale_factor"`
}

type CaptureTile struct {
	Index             int               `json:"index"`
	Row               int               `json:"row"`
	Column            int               `json:"column"`
	SourceBounds      Bounds            `json:"source_bounds"`
	DestinationBounds *Bounds           `json:"destination_bounds,omitempty"`
	OutputPath        string            `json:"output_path,omitempty"`
	ByteSize          int               `json:"byte_size,omitempty"`
	Status            CaptureTileStatus `json:"status"`
	Error             string            `json:"error,omitempty"`
	Bytes             []byte            `json:"-"`
}

type CaptureTiling struct {
	Status         CaptureTilingStatus `json:"status"`
	TargetBounds   Bounds              `json:"target_bounds"`
	Limits         CaptureTileLimits   `json:"limits"`
	TileCount      int                 `json:"tile_count"`
	CompletedCount int                 `json:"completed_count"`
	FailedCount    int                 `json:"failed_count"`
	MetadataPath   string              `json:"metadata_path,omitempty"`
	StitchedPath   string              `json:"stitched_path,omitempty"`
	Tiles          []CaptureTile       `json:"tiles,omitempty"`
	Warnings       []CaptureWarning    `json:"warnings,omitempty"`
}

type CaptureArtifact struct {
	Bytes       []byte      `json:"-"`
	ImageFormat string      `json:"image_format"`
	Mode        CaptureMode `json:"mode"`
	Bounds      *Bounds     `json:"bounds,omitempty"`
	MatchCount  int         `json:"match_count,omitempty"`
	URL         string      `json:"url"`
	Viewport    Viewport    `json:"viewport"`
	Selectors   []string    `json:"selectors,omitempty"`
}

type BrowserInfo struct {
	Engine          string `json:"engine"`
	BrowserPath     string `json:"browser_path,omitempty"`
	Product         string `json:"product,omitempty"`
	ProtocolVersion string `json:"protocol_version,omitempty"`
	Revision        string `json:"revision,omitempty"`
	UserAgent       string `json:"user_agent,omitempty"`
	JSVersion       string `json:"js_version,omitempty"`
	Headless        bool   `json:"headless"`
}

type CaptureTiming struct {
	NavigationStartedAt   time.Time `json:"navigation_started_at"`
	NavigationCompletedAt time.Time `json:"navigation_completed_at"`
	ReadyAt               time.Time `json:"ready_at"`
	CapturedAt            time.Time `json:"captured_at"`
	TotalDuration         string    `json:"total_duration"`
}

type CaptureWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CaptureNormalization struct {
	OutputGenerated   bool          `json:"output_generated"`
	OutputBaseName    string        `json:"output_base_name,omitempty"`
	OutputDirectory   string        `json:"output_directory,omitempty"`
	ViewportPreset    string        `json:"viewport_preset,omitempty"`
	DevicePreset      string        `json:"device_preset,omitempty"`
	Readiness         ReadinessMode `json:"readiness"`
	ReadinessIdle     string        `json:"readiness_idle,omitempty"`
	DisableAnimations bool          `json:"disable_animations"`
	ReducedMotion     bool          `json:"reduced_motion"`
	WaitForFonts      bool          `json:"wait_for_fonts"`
}

type CaptureResult struct {
	OutputPath     string               `json:"output_path"`
	MetadataPath   string               `json:"metadata_path,omitempty"`
	ByteSize       int                  `json:"byte_size"`
	Artifact       CaptureArtifact      `json:"artifact"`
	CapturedAt     time.Time            `json:"captured_at"`
	Engine         string               `json:"engine"`
	Browser        BrowserInfo          `json:"browser"`
	Timing         CaptureTiming        `json:"timing"`
	Warnings       []CaptureWarning     `json:"warnings,omitempty"`
	Tiling         *CaptureTiling       `json:"tiling,omitempty"`
	Normalization  CaptureNormalization `json:"normalization"`
	ResolvedConfig CaptureRequest       `json:"resolved_config"`
}

type BatchResult struct {
	Results []CaptureResult `json:"results"`
}

type Manifest struct {
	OutputDir         string             `json:"output_dir,omitempty" yaml:"output_dir,omitempty"`
	Viewport          *Viewport          `json:"viewport,omitempty" yaml:"viewport,omitempty"`
	ViewportPreset    string             `json:"viewport_preset,omitempty" yaml:"viewport_preset,omitempty"`
	DevicePreset      string             `json:"device_preset,omitempty" yaml:"device_preset,omitempty"`
	Wait              string             `json:"wait,omitempty" yaml:"wait,omitempty"`
	WaitFor           string             `json:"wait_for,omitempty" yaml:"wait_for,omitempty"`
	Readiness         ReadinessMode      `json:"readiness,omitempty" yaml:"readiness,omitempty"`
	ReadinessIdle     string             `json:"readiness_idle,omitempty" yaml:"readiness_idle,omitempty"`
	DisableAnimations bool               `json:"disable_animations,omitempty" yaml:"disable_animations,omitempty"`
	ReducedMotion     bool               `json:"reduced_motion,omitempty" yaml:"reduced_motion,omitempty"`
	WaitForFonts      bool               `json:"wait_for_fonts,omitempty" yaml:"wait_for_fonts,omitempty"`
	Timeout           string             `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	OversizePolicy    OversizePolicy     `json:"oversize_policy,omitempty" yaml:"oversize_policy,omitempty"`
	Tile              CaptureTileOptions `json:"tile,omitempty,omitzero" yaml:"tile,omitempty"`
	Shots             []ManifestShot     `json:"shots" yaml:"shots"`
}

type ManifestShot struct {
	ID                string             `json:"id,omitempty" yaml:"id,omitempty"`
	URL               string             `json:"url" yaml:"url"`
	Output            string             `json:"output,omitempty" yaml:"output,omitempty"`
	Metadata          string             `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	FullPage          bool               `json:"full_page,omitempty" yaml:"full_page,omitempty"`
	Selector          string             `json:"selector,omitempty" yaml:"selector,omitempty"`
	Selectors         []string           `json:"selectors,omitempty" yaml:"selectors,omitempty"`
	SelectorAll       string             `json:"selector_all,omitempty" yaml:"selector_all,omitempty"`
	SelectorsAll      []string           `json:"selectors_all,omitempty" yaml:"selectors_all,omitempty"`
	Padding           int                `json:"padding,omitempty" yaml:"padding,omitempty"`
	Wait              string             `json:"wait,omitempty" yaml:"wait,omitempty"`
	WaitFor           string             `json:"wait_for,omitempty" yaml:"wait_for,omitempty"`
	JavaScript        string             `json:"javascript,omitempty" yaml:"javascript,omitempty"`
	Viewport          *Viewport          `json:"viewport,omitempty" yaml:"viewport,omitempty"`
	ViewportPreset    string             `json:"viewport_preset,omitempty" yaml:"viewport_preset,omitempty"`
	DevicePreset      string             `json:"device_preset,omitempty" yaml:"device_preset,omitempty"`
	Readiness         ReadinessMode      `json:"readiness,omitempty" yaml:"readiness,omitempty"`
	ReadinessIdle     string             `json:"readiness_idle,omitempty" yaml:"readiness_idle,omitempty"`
	DisableAnimations bool               `json:"disable_animations,omitempty" yaml:"disable_animations,omitempty"`
	ReducedMotion     bool               `json:"reduced_motion,omitempty" yaml:"reduced_motion,omitempty"`
	WaitForFonts      bool               `json:"wait_for_fonts,omitempty" yaml:"wait_for_fonts,omitempty"`
	Timeout           string             `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	OversizePolicy    OversizePolicy     `json:"oversize_policy,omitempty" yaml:"oversize_policy,omitempty"`
	Tile              CaptureTileOptions `json:"tile,omitempty,omitzero" yaml:"tile,omitempty"`
}

type EngineResult struct {
	Artifact CaptureArtifact  `json:"artifact"`
	Browser  BrowserInfo      `json:"browser"`
	Timing   CaptureTiming    `json:"timing"`
	Warnings []CaptureWarning `json:"warnings,omitempty"`
	Tiling   *CaptureTiling   `json:"tiling,omitempty"`
}

func NormalizeCaptureRequest(req CaptureRequest) (CaptureRequest, error) {
	req = trimCaptureRequest(req)
	if err := validatePresetSelection(req); err != nil {
		return CaptureRequest{}, err
	}
	if err := applyPresets(&req); err != nil {
		return CaptureRequest{}, err
	}
	if err := normalizeViewport(&req.Viewport); err != nil {
		return CaptureRequest{}, err
	}
	if req.Readiness == "" {
		req.Readiness = defaultReadinessMode
	}
	normalizeTileOptions(&req)
	if err := validateCaptureRequest(req); err != nil {
		return CaptureRequest{}, err
	}
	normalizeCaptureOutputPaths(&req)

	return req, nil
}

func trimCaptureRequest(req CaptureRequest) CaptureRequest {
	req.URL = strings.TrimSpace(req.URL)
	req.OutputPath = strings.TrimSpace(req.OutputPath)
	req.MetadataPath = strings.TrimSpace(req.MetadataPath)
	req.Selector = strings.TrimSpace(req.Selector)
	req.SelectorAll = strings.TrimSpace(req.SelectorAll)
	req.Wait = strings.TrimSpace(req.Wait)
	req.WaitFor = strings.TrimSpace(req.WaitFor)
	req.JavaScript = strings.TrimSpace(req.JavaScript)
	req.BeforeNavigateJS = strings.TrimSpace(req.BeforeNavigateJS)
	req.AfterNavigateJS = strings.TrimSpace(req.AfterNavigateJS)
	req.BeforeCaptureJS = strings.TrimSpace(req.BeforeCaptureJS)
	req.Timeout = strings.TrimSpace(req.Timeout)
	req.OversizePolicy = OversizePolicy(strings.TrimSpace(strings.ToLower(string(req.OversizePolicy))))
	req.ViewportPreset = normalizePresetName(req.ViewportPreset)
	req.DevicePreset = normalizePresetName(req.DevicePreset)
	req.UserAgent = strings.TrimSpace(req.UserAgent)
	req.ReadinessIdle = strings.TrimSpace(req.ReadinessIdle)
	req.Selectors = normalizeStrings(req.Selectors)
	req.SelectorsAll = normalizeStrings(req.SelectorsAll)
	return req
}

func validateCaptureRequest(req CaptureRequest) error {
	if req.URL == "" {
		return newCaptureError(CodeValidation, "normalize_request", "capture url is required", nil)
	}
	if req.Padding < 0 {
		return newCaptureError(CodeValidation, "normalize_request", "padding must be >= 0", nil)
	}
	if !isValidReadinessMode(req.Readiness) {
		return newCaptureError(CodeValidation, "normalize_request", fmt.Sprintf("unsupported readiness mode %q", req.Readiness), nil)
	}
	if err := validateCaptureDurations(req); err != nil {
		return err
	}
	if err := validateTileOptions(req); err != nil {
		return err
	}
	return validateTargetMode(req)
}

func normalizeTileOptions(req *CaptureRequest) {
}

func validateTileOptions(req CaptureRequest) error {
	switch effectiveOversizePolicy(req) {
	case OversizePolicyFail, OversizePolicyTile:
	default:
		return newCaptureError(CodeValidation, "normalize_request", fmt.Sprintf("unsupported oversize policy %q", req.OversizePolicy), nil)
	}
	tile := effectiveTileOptions(req.Tile)
	if tile.MaxWidth <= 0 {
		return newCaptureError(CodeValidation, "normalize_request", "tile max_width must be > 0", nil)
	}
	if tile.MaxHeight <= 0 {
		return newCaptureError(CodeValidation, "normalize_request", "tile max_height must be > 0", nil)
	}
	if tile.MaxPixels <= 0 {
		return newCaptureError(CodeValidation, "normalize_request", "tile max_pixels must be > 0", nil)
	}
	if tile.MaxTargetPixels <= 0 {
		return newCaptureError(CodeValidation, "normalize_request", "tile max_target_pixels must be > 0", nil)
	}
	if tile.MaxStitchedPixels <= 0 {
		return newCaptureError(CodeValidation, "normalize_request", "tile max_stitched_pixels must be > 0", nil)
	}
	if tile.Overlap < 0 {
		return newCaptureError(CodeValidation, "normalize_request", "tile overlap must be >= 0", nil)
	}
	if tile.Overlap >= tile.MaxWidth || tile.Overlap >= tile.MaxHeight {
		return newCaptureError(CodeValidation, "normalize_request", "tile overlap must be smaller than max_width and max_height", nil)
	}
	return nil
}

func effectiveOversizePolicy(req CaptureRequest) OversizePolicy {
	if req.OversizePolicy == "" {
		return DefaultOversizePolicy
	}
	return req.OversizePolicy
}

func effectiveTileOptions(tile CaptureTileOptions) CaptureTileOptions {
	if tile.MaxWidth == 0 {
		tile.MaxWidth = DefaultTileMaxWidth
	}
	if tile.MaxHeight == 0 {
		tile.MaxHeight = DefaultTileMaxHeight
	}
	if tile.MaxPixels == 0 {
		tile.MaxPixels = DefaultTileMaxPixels
	}
	if tile.MaxTargetPixels == 0 {
		tile.MaxTargetPixels = DefaultTileMaxTargetPixels
	}
	if tile.MaxStitchedPixels == 0 {
		tile.MaxStitchedPixels = DefaultTileMaxStitchPixels
	}
	return tile
}

func validateCaptureDurations(req CaptureRequest) error {
	if _, err := ParseDurationOrDefault(req.Timeout, defaultTimeout); err != nil {
		return newCaptureError(CodeValidation, "normalize_request", "invalid timeout duration", err)
	}
	if _, err := ParseDurationOrDefault(req.Wait, 0); err != nil {
		return newCaptureError(CodeValidation, "normalize_request", "invalid wait duration", err)
	}
	if _, err := ParseDurationOrDefault(req.ReadinessIdle, defaultReadinessIdle); err != nil {
		return newCaptureError(CodeValidation, "normalize_request", "invalid readiness idle duration", err)
	}
	return nil
}

func normalizeCaptureOutputPaths(req *CaptureRequest) {
	if req == nil {
		return
	}
	if req.Readiness == ReadinessNetworkIdle && req.ReadinessIdle == "" {
		req.ReadinessIdle = defaultReadinessIdle.String()
	}
	if req.OutputPath != "" && filepath.Ext(req.OutputPath) == "" {
		req.OutputPath += "." + defaultImageFormat
	}
	if req.OutputPath != "" && req.MetadataPath == "" {
		req.MetadataPath = req.OutputPath + ".json"
	}
}

func (req CaptureRequest) Mode() CaptureMode {
	switch {
	case req.FullPage:
		return CaptureModeFullPage
	case req.Selector != "":
		return CaptureModeSelector
	case len(req.Selectors) > 0:
		return CaptureModeSelectors
	case req.SelectorAll != "":
		return CaptureModeSelectorAll
	case len(req.SelectorsAll) > 0:
		return CaptureModeSelectorsAll
	default:
		return CaptureModeViewport
	}
}

func (req CaptureRequest) TargetSelectors() ([]string, bool) {
	switch req.Mode() {
	case CaptureModeSelector:
		return []string{req.Selector}, false
	case CaptureModeSelectors:
		return append([]string(nil), req.Selectors...), false
	case CaptureModeSelectorAll:
		return []string{req.SelectorAll}, true
	case CaptureModeSelectorsAll:
		return append([]string(nil), req.SelectorsAll...), true
	default:
		return nil, false
	}
}

func (req CaptureRequest) WaitDuration() time.Duration {
	value, _ := ParseDurationOrDefault(req.Wait, 0)
	return value
}

func (req CaptureRequest) TimeoutDuration() time.Duration {
	value, _ := ParseDurationOrDefault(req.Timeout, defaultTimeout)
	return value
}

func (req CaptureRequest) ReadinessIdleDuration() time.Duration {
	value, _ := ParseDurationOrDefault(req.ReadinessIdle, defaultReadinessIdle)
	return value
}

func (req CaptureRequest) Normalization(outputGenerated bool, outputBaseName, outputDirectory string) CaptureNormalization {
	return CaptureNormalization{
		OutputGenerated:   outputGenerated,
		OutputBaseName:    strings.TrimSpace(outputBaseName),
		OutputDirectory:   strings.TrimSpace(outputDirectory),
		ViewportPreset:    req.ViewportPreset,
		DevicePreset:      req.DevicePreset,
		Readiness:         req.Readiness,
		ReadinessIdle:     req.ReadinessIdle,
		DisableAnimations: req.DisableAnimations,
		ReducedMotion:     req.ReducedMotion,
		WaitForFonts:      req.WaitForFonts,
	}
}

func ParseDurationOrDefault(raw string, fallback time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	return time.ParseDuration(raw)
}

func normalizeViewport(viewport *Viewport) error {
	if viewport == nil {
		return newCaptureError(CodeValidation, "normalize_request", "viewport is required", nil)
	}
	if viewport.Width <= 0 {
		viewport.Width = defaultViewportWidth
	}
	if viewport.Height <= 0 {
		viewport.Height = defaultViewportHeight
	}
	if viewport.ScaleFactor <= 0 {
		viewport.ScaleFactor = defaultScaleFactor
	}
	if viewport.Width <= 0 || viewport.Height <= 0 {
		return newCaptureError(CodeValidation, "normalize_request", "viewport width and height must be > 0", nil)
	}
	return nil
}

func validatePresetSelection(req CaptureRequest) error {
	if req.ViewportPreset != "" && req.DevicePreset != "" {
		return newCaptureError(CodeValidation, "normalize_request", "viewport_preset and device_preset are mutually exclusive", nil)
	}
	if req.ViewportPreset != "" && hasExplicitViewport(req.Viewport) && !matchesViewportPreset(req.ViewportPreset, req.Viewport) {
		return newCaptureError(CodeValidation, "normalize_request", "viewport_preset cannot be combined with explicit viewport values", nil)
	}
	if req.DevicePreset != "" && (hasExplicitViewport(req.Viewport) || req.UserAgent != "") && !matchesDevicePreset(req.DevicePreset, req.Viewport, req.UserAgent) {
		return newCaptureError(CodeValidation, "normalize_request", "device_preset cannot be combined with explicit viewport values or user_agent", nil)
	}
	return nil
}

func applyPresets(req *CaptureRequest) error {
	if req == nil {
		return nil
	}
	if req.DevicePreset != "" {
		profile, ok := LookupDevicePreset(req.DevicePreset)
		if !ok {
			return newCaptureError(CodeValidation, "normalize_request", fmt.Sprintf("unsupported device preset %q", req.DevicePreset), nil)
		}
		req.Viewport = profile.Viewport
		req.UserAgent = profile.UserAgent
		return nil
	}
	if req.ViewportPreset != "" {
		viewport, ok := LookupViewportPreset(req.ViewportPreset)
		if !ok {
			return newCaptureError(CodeValidation, "normalize_request", fmt.Sprintf("unsupported viewport preset %q", req.ViewportPreset), nil)
		}
		req.Viewport = viewport
	}
	return nil
}

func validateTargetMode(req CaptureRequest) error {
	targetCount := 0
	if req.FullPage {
		targetCount++
	}
	if req.Selector != "" {
		targetCount++
	}
	if len(req.Selectors) > 0 {
		targetCount++
	}
	if req.SelectorAll != "" {
		targetCount++
	}
	if len(req.SelectorsAll) > 0 {
		targetCount++
	}
	if targetCount > 1 {
		return newCaptureError(CodeValidation, "normalize_request", "capture target flags are mutually exclusive", nil)
	}
	return nil
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizePresetName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func isValidReadinessMode(mode ReadinessMode) bool {
	switch mode {
	case "", ReadinessNone, ReadinessInteractive, ReadinessComplete, ReadinessNetworkIdle:
		return true
	default:
		return false
	}
}

func hasExplicitViewport(viewport Viewport) bool {
	return viewport.Width > 0 || viewport.Height > 0 || viewport.ScaleFactor > 0 || viewport.Mobile
}

func matchesViewportPreset(name string, viewport Viewport) bool {
	preset, ok := LookupViewportPreset(name)
	if !ok {
		return false
	}
	return preset == viewport
}

func matchesDevicePreset(name string, viewport Viewport, userAgent string) bool {
	preset, ok := LookupDevicePreset(name)
	if !ok {
		return false
	}
	return preset.Viewport == viewport && strings.TrimSpace(preset.UserAgent) == strings.TrimSpace(userAgent)
}

func cloneWarnings(warnings []CaptureWarning) []CaptureWarning {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]CaptureWarning, len(warnings))
	copy(out, warnings)
	return out
}
