package webcap

import (
	"context"
	"net/http"
	"time"

	"github.com/goliatone/webcap/pkg/llms"
)

const (
	WorkflowReportFormatHTML = "html"
	WorkflowHookModeAppend   = "append"
	WorkflowHookModeReplace  = "replace"
)

type Options struct {
	Workflow     WorkflowOptions
	SemanticDiff SemanticDiffOptions
	Now          func() time.Time
}

type SemanticDiffOptions struct {
	Providers                    map[string]SemanticDiffProvider
	CredentialResolver           SemanticCredentialResolver
	RedactImage                  SemanticImageRedactor
	HTTPClient                   *http.Client
	DefaultProvider              string
	DefaultModels                map[string]string
	DefaultTimeout               time.Duration
	MaxImageBytes                int64
	MaxProviderImageBytes        int64
	MaxImageLongEdge             int
	MaxImagePixels               int64
	MaxEncodedImageBytes         int64
	MaxCombinedEncodedImageBytes int64
	MaxRequestBodyBytes          int64
	ResizeImages                 bool
	MaxOutputTokens              int
	PersistRawResponses          bool
	LLMs                         llms.Options
	OpenAIBaseURL                string
	AnthropicBaseURL             string
}

type WorkflowOptions struct {
	DefaultSelectedScenario string
	DefaultPresentationMode string
	HandoffQueryParam       string
	BuildHandoff            func(WorkflowScenario) (string, error)
}

type WorkflowHandoff struct {
	SelectedScenario string            `json:"selected_scenario,omitempty"`
	PresentationMode string            `json:"presentation_mode,omitempty"`
	RuntimeOverrides map[string]string `json:"runtime_overrides,omitempty"`
}

type WorkflowScenario struct {
	ID          string                   `json:"id" yaml:"id"`
	Label       string                   `json:"label,omitempty" yaml:"label,omitempty"`
	Description string                   `json:"description,omitempty" yaml:"description,omitempty"`
	StorySource string                   `json:"story_source,omitempty" yaml:"story_source,omitempty"`
	Environment WorkflowEnvironment      `json:"environment" yaml:"environment,omitempty"`
	Artifacts   WorkflowArtifactLayout   `json:"artifacts" yaml:"artifacts,omitempty"`
	Defaults    WorkflowDefaults         `json:"defaults" yaml:"defaults,omitempty"`
	Hooks       WorkflowHooks            `json:"hooks" yaml:"hooks,omitempty"`
	Stories     map[string]WorkflowStory `json:"stories,omitempty" yaml:"stories,omitempty"`
	Screens     []WorkflowScreen         `json:"screens" yaml:"screens"`
	Context     map[string]any           `json:"context,omitempty" yaml:"context,omitempty"`
	SourcePath  string                   `json:"-" yaml:"-"`
	SourceDir   string                   `json:"-" yaml:"-"`
}

type WorkflowEnvironment struct {
	BaseURL              string            `json:"base_url" yaml:"base_url"`
	Engine               string            `json:"engine,omitempty" yaml:"engine,omitempty"`
	BrowserPath          string            `json:"browser_path,omitempty" yaml:"browser_path,omitempty"`
	Headless             *bool             `json:"headless,omitempty" yaml:"headless,omitempty"`
	PlaywrightBrowser    string            `json:"playwright_browser,omitempty" yaml:"playwright_browser,omitempty"`
	PlaywrightNodeBinary string            `json:"playwright_node_binary,omitempty" yaml:"playwright_node_binary,omitempty"`
	PlaywrightRuntimeDir string            `json:"playwright_runtime_dir,omitempty" yaml:"playwright_runtime_dir,omitempty"`
	FixtureMode          string            `json:"fixture_mode,omitempty" yaml:"fixture_mode,omitempty"`
	ReportFormat         string            `json:"report_format,omitempty" yaml:"report_format,omitempty"`
	SelectedScenario     string            `json:"selected_scenario,omitempty" yaml:"selected_scenario,omitempty"`
	PresentationMode     string            `json:"presentation_mode,omitempty" yaml:"presentation_mode,omitempty"`
	RuntimeOverrides     map[string]string `json:"runtime_overrides,omitempty" yaml:"runtime_overrides,omitempty"`
	Query                map[string]string `json:"query,omitempty" yaml:"query,omitempty"`
}

type WorkflowArtifactLayout struct {
	Root         string `json:"root,omitempty" yaml:"root,omitempty"`
	CurrentDir   string `json:"current_dir,omitempty" yaml:"current_dir,omitempty"`
	ReferenceDir string `json:"reference_dir,omitempty" yaml:"reference_dir,omitempty"`
	DiffDir      string `json:"diff_dir,omitempty" yaml:"diff_dir,omitempty"`
	ReportDir    string `json:"report_dir,omitempty" yaml:"report_dir,omitempty"`
}

type WorkflowDefaults struct {
	Viewport          Viewport             `json:"viewport" yaml:"viewport,omitempty"`
	ViewportPreset    string               `json:"viewport_preset,omitempty" yaml:"viewport_preset,omitempty"`
	DevicePreset      string               `json:"device_preset,omitempty" yaml:"device_preset,omitempty"`
	Comparison        WorkflowComparison   `json:"comparison" yaml:"comparison,omitempty"`
	SemanticDiff      WorkflowSemanticDiff `json:"semantic_diff" yaml:"semantic_diff,omitempty"`
	Readiness         ReadinessMode        `json:"readiness,omitempty" yaml:"readiness,omitempty"`
	ReadinessIdle     string               `json:"readiness_idle,omitempty" yaml:"readiness_idle,omitempty"`
	DisableAnimations bool                 `json:"disable_animations,omitempty" yaml:"disable_animations,omitempty"`
	ReducedMotion     bool                 `json:"reduced_motion,omitempty" yaml:"reduced_motion,omitempty"`
	WaitForFonts      bool                 `json:"wait_for_fonts,omitempty" yaml:"wait_for_fonts,omitempty"`
	Timeout           string               `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Wait              string               `json:"wait,omitempty" yaml:"wait,omitempty"`
	WaitFor           string               `json:"wait_for,omitempty" yaml:"wait_for,omitempty"`
	WaitForFunction   string               `json:"wait_for_function,omitempty" yaml:"wait_for_function,omitempty"`
	FullPage          *bool                `json:"full_page,omitempty" yaml:"full_page,omitempty"`
	OversizePolicy    OversizePolicy       `json:"oversize_policy,omitempty" yaml:"oversize_policy,omitempty"`
	Tile              CaptureTileOptions   `json:"tile,omitzero" yaml:"tile,omitempty"`
}

type WorkflowHooks struct {
	AuthSetup  WorkflowHookSet `json:"auth_setup" yaml:"auth_setup,omitempty"`
	StateSetup WorkflowHookSet `json:"state_setup" yaml:"state_setup,omitempty"`
	Navigation WorkflowHookSet `json:"navigation" yaml:"navigation,omitempty"`
}

type WorkflowHookSet struct {
	BeforeNavigate string         `json:"before_navigate,omitempty" yaml:"before_navigate,omitempty"`
	AfterNavigate  string         `json:"after_navigate,omitempty" yaml:"after_navigate,omitempty"`
	BeforeCapture  string         `json:"before_capture,omitempty" yaml:"before_capture,omitempty"`
	Mode           string         `json:"mode,omitempty" yaml:"mode,omitempty"`
	Context        map[string]any `json:"context,omitempty" yaml:"context,omitempty"`
}

type WorkflowStory struct {
	ID                 string   `json:"id" yaml:"id"`
	Priority           string   `json:"priority,omitempty" yaml:"priority,omitempty"`
	Title              string   `json:"title,omitempty" yaml:"title,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty" yaml:"acceptance_criteria,omitempty"`
	Notes              []string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type WorkflowScreen struct {
	ID                string                 `json:"id" yaml:"id"`
	Label             string                 `json:"label,omitempty" yaml:"label,omitempty"`
	Route             string                 `json:"route" yaml:"route"`
	Query             map[string]string      `json:"query,omitempty" yaml:"query,omitempty"`
	OutputName        string                 `json:"output_name,omitempty" yaml:"output_name,omitempty"`
	Comparison        WorkflowComparison     `json:"comparison" yaml:"comparison,omitempty"`
	SemanticDiff      WorkflowSemanticDiff   `json:"semantic_diff" yaml:"semantic_diff,omitempty"`
	ReferenceImage    string                 `json:"reference_image" yaml:"reference_image"`
	PrimaryStories    []string               `json:"primary_stories,omitempty" yaml:"primary_stories,omitempty"`
	SupportingStories []string               `json:"supporting_stories,omitempty" yaml:"supporting_stories,omitempty"`
	ExpectedElements  []string               `json:"expected_elements,omitempty" yaml:"expected_elements,omitempty"`
	Notes             []string               `json:"notes,omitempty" yaml:"notes,omitempty"`
	Annotations       []string               `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Hooks             WorkflowHooks          `json:"hooks" yaml:"hooks,omitempty"`
	Evidence          []WorkflowEvidenceItem `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	Capture           CaptureRequest         `json:"capture" yaml:"capture,omitempty"`
}

type WorkflowComparison struct {
	Mode          string               `json:"mode,omitempty" yaml:"mode,omitempty"`
	CurrentCrop   *WorkflowCompareRect `json:"current_crop,omitempty" yaml:"current_crop,omitempty"`
	ReferenceCrop *WorkflowCompareRect `json:"reference_crop,omitempty" yaml:"reference_crop,omitempty"`
	ResizeTo      string               `json:"resize_to,omitempty" yaml:"resize_to,omitempty"`
}

type WorkflowCompareRect struct {
	X      int `json:"x" yaml:"x"`
	Y      int `json:"y" yaml:"y"`
	Width  int `json:"width" yaml:"width"`
	Height int `json:"height" yaml:"height"`
}

type WorkflowSemanticDiff struct {
	Enabled            *bool                      `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Provider           string                     `json:"provider,omitempty" yaml:"provider,omitempty"`
	Model              string                     `json:"model,omitempty" yaml:"model,omitempty"`
	Mode               SemanticDiffMode           `json:"mode,omitempty" yaml:"mode,omitempty"`
	Run                SemanticDiffRunPolicy      `json:"run,omitempty" yaml:"run,omitempty"`
	Focus              []string                   `json:"focus,omitempty" yaml:"focus,omitempty"`
	Prompt             string                     `json:"prompt,omitempty" yaml:"prompt,omitempty"`
	PromptPath         string                     `json:"prompt_path,omitempty" yaml:"prompt_path,omitempty"`
	Timeout            string                     `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	MaxOutputTokens    int                        `json:"max_output_tokens,omitempty" yaml:"max_output_tokens,omitempty"`
	PersistRawResponse bool                       `json:"persist_raw_response,omitempty" yaml:"persist_raw_response,omitempty"`
	RawResponsePath    string                     `json:"raw_response_path,omitempty" yaml:"raw_response_path,omitempty"`
	AdvisoryPolicy     SemanticDiffAdvisoryPolicy `json:"advisory_policy,omitempty" yaml:"advisory_policy,omitempty"`
	FailureSeverity    SemanticDiffSeverity       `json:"failure_severity,omitempty" yaml:"failure_severity,omitempty"`
	FailureVerdicts    []SemanticDiffVerdict      `json:"failure_verdicts,omitempty" yaml:"failure_verdicts,omitempty"`
	APIKey             string                     `json:"-" yaml:"api_key,omitempty"`
	OpenAIAPIKey       string                     `json:"-" yaml:"openai_api_key,omitempty"`
	AnthropicAPIKey    string                     `json:"-" yaml:"anthropic_api_key,omitempty"`
}

type WorkflowEvidenceItem struct {
	ID                string   `json:"id" yaml:"id"`
	Text              string   `json:"text" yaml:"text"`
	Stories           []string `json:"stories,omitempty" yaml:"stories,omitempty"`
	ExpectedSelectors []string `json:"expected_selectors,omitempty" yaml:"expected_selectors,omitempty"`
	Notes             []string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type WorkflowScreenCaptureResult struct {
	ScreenID         string        `json:"screen_id"`
	Label            string        `json:"label"`
	Route            string        `json:"route"`
	TargetURL        string        `json:"target_url"`
	OutputPath       string        `json:"output_path"`
	MetadataPath     string        `json:"metadata_path,omitempty"`
	ReferenceImage   string        `json:"reference_image"`
	Capture          CaptureResult `json:"capture"`
	ExpectedElements []string      `json:"expected_elements,omitempty"`
}

type WorkflowCaptureResult struct {
	ScenarioID   string                        `json:"scenario_id"`
	ScenarioPath string                        `json:"scenario_path"`
	CurrentDir   string                        `json:"current_dir"`
	CapturedAt   time.Time                     `json:"captured_at"`
	Results      []WorkflowScreenCaptureResult `json:"results"`
}

type WorkflowReportRequest struct {
	Scenario WorkflowScenario `json:"scenario"`
}

type WorkflowReportEntry struct {
	ScreenID                   string                 `json:"screen_id"`
	Label                      string                 `json:"label"`
	Route                      string                 `json:"route"`
	TargetURL                  string                 `json:"target_url"`
	CurrentImagePath           string                 `json:"current_image_path,omitempty"`
	CurrentMetadata            string                 `json:"current_metadata,omitempty"`
	CurrentCapture             *CaptureResult         `json:"current_capture,omitempty"`
	ReferenceImage             string                 `json:"reference_image"`
	ComparisonMode             string                 `json:"comparison_mode,omitempty"`
	ComparedCurrentImagePath   string                 `json:"compared_current_image_path,omitempty"`
	ComparedReferenceImagePath string                 `json:"compared_reference_image_path,omitempty"`
	DiffImagePath              string                 `json:"diff_image_path,omitempty"`
	DiffMetadataPath           string                 `json:"diff_metadata_path,omitempty"`
	DiffEntry                  *DiffEntry             `json:"diff_entry,omitempty"`
	DiffSummary                *DiffSummary           `json:"diff_summary,omitempty"`
	SemanticDiff               *SemanticDiffResult    `json:"semantic_diff,omitempty"`
	SemanticMetadataPath       string                 `json:"semantic_metadata_path,omitempty"`
	SemanticFailure            bool                   `json:"semantic_failure,omitempty"`
	MissingCurrent             bool                   `json:"missing_current,omitempty"`
	MissingReference           bool                   `json:"missing_reference,omitempty"`
	PrimaryStories             []WorkflowStory        `json:"primary_stories,omitempty"`
	SupportingStories          []WorkflowStory        `json:"supporting_stories,omitempty"`
	Evidence                   []WorkflowEvidenceItem `json:"evidence,omitempty"`
	ExpectedElements           []string               `json:"expected_elements,omitempty"`
	Notes                      []string               `json:"notes,omitempty"`
	Annotations                []string               `json:"annotations,omitempty"`
	Warnings                   []CaptureWarning       `json:"warnings,omitempty"`
	Status                     WorkflowReviewStatus   `json:"status"`
}

type WorkflowReviewStatus struct {
	Level   string `json:"level"`
	Label   string `json:"label"`
	Summary string `json:"summary,omitempty"`
}

type WorkflowStoryReport struct {
	StoryID      string                 `json:"story_id"`
	Story        WorkflowStory          `json:"story"`
	ScreenIDs    []string               `json:"screen_ids,omitempty"`
	Evidence     []WorkflowEvidenceItem `json:"evidence,omitempty"`
	MissingPaths []string               `json:"missing_paths,omitempty"`
	Status       WorkflowReviewStatus   `json:"status"`
}

type WorkflowReportResult struct {
	ScenarioID   string                `json:"scenario_id"`
	ScenarioPath string                `json:"scenario_path"`
	ReportFormat string                `json:"report_format"`
	ReportPath   string                `json:"report_path"`
	MetadataPath string                `json:"metadata_path"`
	CurrentDir   string                `json:"current_dir"`
	DiffDir      string                `json:"diff_dir"`
	Entries      []WorkflowReportEntry `json:"entries"`
	Stories      []WorkflowStoryReport `json:"stories"`
	CreatedAt    time.Time             `json:"created_at"`
	Status       WorkflowReviewStatus  `json:"status"`
}

type WorkflowService interface {
	CaptureScenario(ctx context.Context, scenario WorkflowScenario) (WorkflowCaptureResult, error)
	GenerateWorkflowReport(ctx context.Context, req WorkflowReportRequest) (WorkflowReportResult, error)
}
