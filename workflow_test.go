package webcap

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type recordingEngine struct {
	requests []CaptureRequest
	result   EngineResult
	err      error
}

func (e *recordingEngine) Name() string { return "recording" }

func (e *recordingEngine) Capture(ctx context.Context, req CaptureRequest) (EngineResult, error) {
	e.requests = append(e.requests, req)
	return e.result, e.err
}

func TestParseWorkflowStories(t *testing.T) {
	tempDir := t.TempDir()
	storyPath := filepath.Join(tempDir, "stories.md")
	writeWorkflowStoryFixture(t, storyPath)

	stories, err := parseWorkflowStories(storyPath)
	if err != nil {
		t.Fatalf("parseWorkflowStories returned error: %v", err)
	}
	if len(stories) != 1 {
		t.Fatalf("unexpected story count: %d", len(stories))
	}
	if stories["D1"].Priority != "P1" {
		t.Fatalf("unexpected D1 priority: %s", stories["D1"].Priority)
	}
	if !strings.Contains(stories["D1"].Title, "review a captured page") {
		t.Fatalf("unexpected D1 title: %s", stories["D1"].Title)
	}
	if len(stories["D1"].AcceptanceCriteria) == 0 {
		t.Fatal("expected D1 acceptance criteria")
	}
}

func TestLoadWorkflowScenarioWithOptions(t *testing.T) {
	tempDir := t.TempDir()
	scenarioPath := writeWorkflowScenarioFixture(t, tempDir)

	scenario, err := LoadWorkflowScenarioWithOptions(scenarioPath, WorkflowOptions{
		DefaultSelectedScenario: "fixture-scenario",
		DefaultPresentationMode: "review",
		HandoffQueryParam:       "fixture_handoff",
		BuildHandoff:            DefaultWorkflowHandoff,
	})
	if err != nil {
		t.Fatalf("LoadWorkflowScenario returned error: %v", err)
	}
	if scenario.ID != "fixture-flow" {
		t.Fatalf("unexpected scenario id: %s", scenario.ID)
	}
	if scenario.Environment.BaseURL != "http://localhost:8383" {
		t.Fatalf("unexpected base url: %s", scenario.Environment.BaseURL)
	}
	if scenario.Environment.SelectedScenario != "fixture-scenario" {
		t.Fatalf("unexpected selected scenario: %s", scenario.Environment.SelectedScenario)
	}
	if scenario.Environment.PresentationMode != "review" {
		t.Fatalf("unexpected presentation mode: %s", scenario.Environment.PresentationMode)
	}
	if len(scenario.Screens) != 1 {
		t.Fatalf("unexpected screen count: %d", len(scenario.Screens))
	}
	if scenario.Defaults.Viewport.Width != 1024 {
		t.Fatalf("unexpected default viewport: %+v", scenario.Defaults.Viewport)
	}
	if scenario.Artifacts.Root != filepath.Join(tempDir, "artifacts") {
		t.Fatalf("unexpected artifact root: %s", scenario.Artifacts.Root)
	}
	if _, ok := scenario.Stories["D1"]; !ok {
		t.Fatal("expected D1 story to be loaded from story source")
	}
	if !filepath.IsAbs(scenario.Screens[0].ReferenceImage) {
		t.Fatalf("expected absolute reference path, got %s", scenario.Screens[0].ReferenceImage)
	}
}

func TestCaptureScenarioBuildsRequestsWithConfiguredWorkflowOptions(t *testing.T) {
	tempDir := t.TempDir()
	scenarioPath := writeWorkflowScenarioFixture(t, tempDir)
	opts := WorkflowOptions{
		DefaultSelectedScenario: "fixture-scenario",
		DefaultPresentationMode: "review",
		HandoffQueryParam:       "fixture_handoff",
		BuildHandoff:            DefaultWorkflowHandoff,
	}
	scenario, err := LoadWorkflowScenarioWithOptions(scenarioPath, opts)
	if err != nil {
		t.Fatalf("LoadWorkflowScenario returned error: %v", err)
	}

	engine := &recordingEngine{
		result: EngineResult{
			Artifact: CaptureArtifact{
				Bytes:       []byte("png"),
				ImageFormat: "png",
				Mode:        CaptureModeFullPage,
				URL:         "http://localhost:8383/triage/patient",
				Viewport:    Viewport{Width: 1024, Height: 1200, ScaleFactor: 1},
			},
			Browser: BrowserInfo{Engine: "recording", Headless: true},
			Timing: CaptureTiming{
				NavigationStartedAt:   time.Unix(100, 0).UTC(),
				NavigationCompletedAt: time.Unix(101, 0).UTC(),
				ReadyAt:               time.Unix(102, 0).UTC(),
				CapturedAt:            time.Unix(103, 0).UTC(),
				TotalDuration:         "3s",
			},
		},
	}
	service := NewServiceWithOptions(engine, Options{Workflow: opts})

	result, err := service.CaptureScenario(context.Background(), scenario)
	if err != nil {
		t.Fatalf("CaptureScenario returned error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("unexpected capture count: %d", len(result.Results))
	}
	if len(engine.requests) != 1 {
		t.Fatalf("unexpected request count: %d", len(engine.requests))
	}

	first := engine.requests[0]
	if first.OutputPath != filepath.Join(scenario.Artifacts.CurrentDir, "patient.png") {
		t.Fatalf("unexpected output path: %s", first.OutputPath)
	}
	if !first.FullPage {
		t.Fatal("expected workflow requests to default to full-page capture")
	}
	parsedURL, err := url.Parse(first.URL)
	if err != nil {
		t.Fatalf("parse request url: %v", err)
	}
	query := parsedURL.Query()
	if query.Get("scenario") != "fixture-scenario" {
		t.Fatalf("unexpected scenario query: %s", query.Get("scenario"))
	}
	if query.Get("presentation_mode") != "review" {
		t.Fatalf("unexpected presentation mode: %s", query.Get("presentation_mode"))
	}
	if strings.TrimSpace(query.Get("fixture_handoff")) == "" {
		t.Fatal("expected configured handoff query")
	}
	if _, err := os.Stat(filepath.Join(scenario.Artifacts.CurrentDir, "patient.png.json")); err != nil {
		t.Fatalf("expected metadata sidecar to be written: %v", err)
	}
}

func TestCaptureScenarioCompilesHookScriptsInOrder(t *testing.T) {
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("writeTestPNG: %v", err)
	}

	scenario := WorkflowScenario{
		ID:          "hook-order",
		Label:       "Hook Order",
		SourceDir:   tempDir,
		SourcePath:  filepath.Join(tempDir, "hook-order.yaml"),
		Stories:     map[string]WorkflowStory{"D1": {ID: "D1", Title: "Hook story"}},
		Environment: WorkflowEnvironment{BaseURL: "http://localhost:8383"},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Hooks: WorkflowHooks{
			AuthSetup: WorkflowHookSet{
				BeforeNavigate: "window.__order = (window.__order || []); window.__order.push('scenario-auth');",
			},
			Navigation: WorkflowHookSet{
				BeforeCapture: "window.__order = (window.__order || []); window.__order.push('scenario-nav-capture');",
			},
		},
		Screens: []WorkflowScreen{
			{
				ID:             "hooked",
				Route:          "/triage/patient",
				ReferenceImage: referencePath,
				PrimaryStories: []string{"D1"},
				Hooks: WorkflowHooks{
					StateSetup: WorkflowHookSet{
						BeforeNavigate: "window.__order.push('screen-state');",
					},
					Navigation: WorkflowHookSet{
						Mode:          WorkflowHookModeReplace,
						BeforeCapture: "window.__order.push('screen-nav-capture');",
					},
				},
			},
		},
	}

	engine := &recordingEngine{
		result: EngineResult{
			Artifact: CaptureArtifact{
				Bytes:       []byte("png"),
				ImageFormat: "png",
				Mode:        CaptureModeFullPage,
				URL:         "http://localhost:8383/triage/patient",
				Viewport:    Viewport{Width: 1024, Height: 1200, ScaleFactor: 1},
			},
			Browser: BrowserInfo{Engine: "recording", Headless: true},
			Timing: CaptureTiming{
				NavigationStartedAt:   time.Unix(100, 0).UTC(),
				NavigationCompletedAt: time.Unix(101, 0).UTC(),
				ReadyAt:               time.Unix(102, 0).UTC(),
				CapturedAt:            time.Unix(103, 0).UTC(),
				TotalDuration:         "3s",
			},
		},
	}
	service := NewService(engine)

	if _, err := service.CaptureScenario(context.Background(), scenario); err != nil {
		t.Fatalf("CaptureScenario returned error: %v", err)
	}
	if len(engine.requests) != 1 {
		t.Fatalf("unexpected request count: %d", len(engine.requests))
	}

	beforeNavigate := engine.requests[0].BeforeNavigateJS
	if !strings.Contains(beforeNavigate, "scenario-auth") || !strings.Contains(beforeNavigate, "screen-state") {
		t.Fatalf("expected compiled before navigate hook script, got %s", beforeNavigate)
	}
	if strings.Index(beforeNavigate, "scenario-auth") > strings.Index(beforeNavigate, "screen-state") {
		t.Fatalf("expected scenario hook before screen hook, got %s", beforeNavigate)
	}

	beforeCapture := engine.requests[0].BeforeCaptureJS
	if strings.Contains(beforeCapture, "scenario-nav-capture") {
		t.Fatalf("expected screen replace mode to omit scenario navigation hooks, got %s", beforeCapture)
	}
	if !strings.Contains(beforeCapture, "screen-nav-capture") {
		t.Fatalf("expected compiled screen before capture hook script, got %s", beforeCapture)
	}
}

func TestGenerateWorkflowReportWritesArtifacts(t *testing.T) {
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	currentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 240, G: 240, B: 240, A: 255},
	}); err != nil {
		t.Fatalf("writeTestPNG reference: %v", err)
	}
	if err := writeTestPNG(currentPath, []color.NRGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 230, G: 200, B: 200, A: 255},
	}); err != nil {
		t.Fatalf("writeTestPNG current: %v", err)
	}
	captureMetadata := CaptureResult{
		OutputPath: currentPath,
		Warnings:   []CaptureWarning{{Code: "capture_warning", Message: "demo warning"}},
	}
	encoded, err := json.Marshal(captureMetadata)
	if err != nil {
		t.Fatalf("marshal capture metadata: %v", err)
	}
	if err := os.WriteFile(currentPath+".json", append(encoded, '\n'), 0o644); err != nil {
		t.Fatalf("write capture metadata: %v", err)
	}

	scenario := WorkflowScenario{
		ID:          "report-test",
		Label:       "Report Test",
		Description: "workflow report fixture",
		SourceDir:   tempDir,
		SourcePath:  filepath.Join(tempDir, "report-test.yaml"),
		Stories: map[string]WorkflowStory{
			"D1": {ID: "D1", Priority: "P1", Title: "Patient review"},
		},
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Screens: []WorkflowScreen{
			{
				ID:               "patient",
				Label:            "Patient",
				Route:            "/triage/patient",
				OutputName:       "patient",
				ReferenceImage:   referencePath,
				PrimaryStories:   []string{"D1"},
				ExpectedElements: []string{"#patient-review-proceed-btn"},
				Annotations:      []string{"Focus on patient summary hierarchy."},
				Evidence: []WorkflowEvidenceItem{
					{ID: "patient-direct-entry", Text: "Review step renders", Stories: []string{"D1"}},
				},
			},
		},
	}

	service := NewService(nil)
	service.now = func() time.Time {
		return time.Date(2026, time.March, 30, 20, 46, 0, 0, time.UTC)
	}
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("unexpected entry count: %d", len(result.Entries))
	}
	if result.Entries[0].CurrentCapture == nil {
		t.Fatal("expected report entry to include current capture metadata")
	}
	if result.Entries[0].DiffImagePath == "" {
		t.Fatal("expected diff image path")
	}
	if result.Entries[0].DiffEntry == nil {
		t.Fatal("expected diff entry metadata")
	}
	if len(result.Entries[0].Warnings) == 0 {
		t.Fatal("expected capture warnings to be carried into report entry")
	}
	if result.Entries[0].Status.Level != workflowStatusWarning {
		t.Fatalf("expected warning entry status, got %+v", result.Entries[0].Status)
	}
	if result.Stories[0].Status.Level != workflowStatusWarning {
		t.Fatalf("expected warning story status, got %+v", result.Stories[0].Status)
	}
	if result.Status.Level != workflowStatusWarning {
		t.Fatalf("expected warning report status, got %+v", result.Status)
	}
	if _, err := os.Stat(result.ReportPath); err != nil {
		t.Fatalf("expected html report: %v", err)
	}
	if _, err := os.Stat(result.MetadataPath); err != nil {
		t.Fatalf("expected report metadata: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "styles.css")); err != nil {
		t.Fatalf("expected report stylesheet: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "assets", "current", "patient.png")); err != nil {
		t.Fatalf("expected staged current image asset: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "assets", "reference", filepath.Base(referencePath))); err != nil {
		t.Fatalf("expected staged reference image asset: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "assets", "diff", "patient.png")); err != nil {
		t.Fatalf("expected staged diff image asset: %v", err)
	}
	if _, err := os.Stat(result.Entries[0].DiffImagePath); err != nil {
		t.Fatalf("expected diff image: %v", err)
	}

	reportHTML, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report html: %v", err)
	}
	reportMarkup := string(reportHTML)
	reportStylesheet, err := os.ReadFile(filepath.Join(scenario.Artifacts.ReportDir, "styles.css"))
	if err != nil {
		t.Fatalf("read report stylesheet: %v", err)
	}
	reportCSS := string(reportStylesheet)
	for _, expected := range []string{
		"Print / Save PDF",
		"Generated Mar 30, 2026 at 8:46 PM",
		"Capture Issues",
		"Focus on patient summary hierarchy.",
		"data-screen-id=\"patient\"",
		"function setCompareMode(index, mode)",
		"el.dataset.screenId === screenID",
		"class=\"story-list-item-main\"",
		"data-story-compare-section",
		"style=\"--clip-right: 50%\"",
		"slider.style.setProperty('--clip-right', '50%')",
		"percent.toFixed(1) + '%'",
	} {
		if !strings.Contains(reportMarkup, expected) {
			t.Fatalf("expected report html to contain %q", expected)
		}
	}
	if strings.Contains(reportMarkup, "%%") {
		t.Fatal("expected rendered report html to avoid invalid %% percent literals")
	}
	for _, expected := range []string{
		".story-list-item-main { flex: 1 1 auto; min-width: 0; overflow: hidden; }",
		".story-list-item .pill { margin-left: auto; }",
		"clip-path: inset(0 var(--clip-right, 50%) 0 0);",
		"left: calc(100% - var(--clip-right, 50%));",
		"touch-action: none;",
	} {
		if !strings.Contains(reportCSS, expected) {
			t.Fatalf("expected report stylesheet to contain %q", expected)
		}
	}
	if strings.Contains(reportCSS, "%%") {
		t.Fatal("expected generated stylesheet to avoid invalid %% percent literals")
	}
	if strings.Contains(reportMarkup, "fonts.googleapis.com") {
		t.Fatal("expected report html to avoid remote font dependencies")
	}
	if !strings.Contains(reportMarkup, "document.querySelectorAll('.compare-mode-btn').forEach(btn => {") {
		t.Fatal("expected compare-mode event binding block in report html")
	}
	if !strings.Contains(reportMarkup, "document.addEventListener('keydown', (e) => {") {
		t.Fatal("expected keyboard navigation block in report html")
	}
}

func TestGenerateWorkflowReportUsesComparisonAssets(t *testing.T) {
	tempDir := t.TempDir()
	referencePath := filepath.Join(tempDir, "reference.png")
	currentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	if err := writeTestPNG(referencePath, []color.NRGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 230, G: 230, B: 255, A: 255},
		{R: 210, G: 210, B: 255, A: 255},
		{R: 190, G: 190, B: 255, A: 255},
	}); err != nil {
		t.Fatalf("writeTestPNG reference: %v", err)
	}
	if err := writeTestPNG(currentPath, []color.NRGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 245, B: 245, A: 255},
		{R: 255, G: 235, B: 235, A: 255},
		{R: 255, G: 225, B: 225, A: 255},
	}); err != nil {
		t.Fatalf("writeTestPNG current: %v", err)
	}

	scenario := WorkflowScenario{
		ID:         "report-compare-test",
		Label:      "Report Compare Test",
		SourceDir:  tempDir,
		SourcePath: filepath.Join(tempDir, "report-compare-test.yaml"),
		Stories: map[string]WorkflowStory{
			"D1": {ID: "D1", Title: "Patient review"},
		},
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Defaults: WorkflowDefaults{
			Comparison: WorkflowComparison{
				Mode: WorkflowComparisonModeContentOnly,
				CurrentCrop: &WorkflowCompareRect{
					X: 0, Y: 0, Width: 4, Height: 1,
				},
				ReferenceCrop: &WorkflowCompareRect{
					X: 1, Y: 0, Width: 2, Height: 1,
				},
				ResizeTo: WorkflowComparisonResizeCurrent,
			},
		},
		Screens: []WorkflowScreen{
			{
				ID:             "patient",
				Label:          "Patient",
				Route:          "/triage/patient",
				OutputName:     "patient",
				ReferenceImage: referencePath,
				PrimaryStories: []string{"D1"},
			},
		},
	}

	service := NewService(nil)
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	entry := result.Entries[0]
	if entry.Status.Level != workflowStatusInfo {
		t.Fatalf("expected info entry status for visual diff, got %+v", entry.Status)
	}
	if result.Stories[0].Status.Level != workflowStatusInfo {
		t.Fatalf("expected info story status for visual diff, got %+v", result.Stories[0].Status)
	}
	if result.Status.Level != workflowStatusInfo {
		t.Fatalf("expected info report status for visual diff, got %+v", result.Status)
	}
	if entry.ComparedCurrentImagePath == "" || entry.ComparedReferenceImagePath == "" {
		t.Fatalf("expected compared image paths, got current=%q reference=%q", entry.ComparedCurrentImagePath, entry.ComparedReferenceImagePath)
	}
	if filepath.Base(entry.ComparedCurrentImagePath) != "patient-current.png" {
		t.Fatalf("unexpected compared current asset: %s", entry.ComparedCurrentImagePath)
	}
	if filepath.Base(entry.ComparedReferenceImagePath) != "patient-reference.png" {
		t.Fatalf("unexpected compared reference asset: %s", entry.ComparedReferenceImagePath)
	}

	currentCompared, err := loadImage(entry.ComparedCurrentImagePath)
	if err != nil {
		t.Fatalf("load compared current: %v", err)
	}
	referenceCompared, err := loadImage(entry.ComparedReferenceImagePath)
	if err != nil {
		t.Fatalf("load compared reference: %v", err)
	}
	if currentCompared.Bounds().Size() != image.Pt(4, 1) {
		t.Fatalf("unexpected compared current size: %v", currentCompared.Bounds().Size())
	}
	if referenceCompared.Bounds().Size() != image.Pt(4, 1) {
		t.Fatalf("unexpected compared reference size: %v", referenceCompared.Bounds().Size())
	}
	if _, err := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "assets", "current", "patient-current.png")); err != nil {
		t.Fatalf("expected staged compared current image asset: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scenario.Artifacts.ReportDir, "assets", "reference", "patient-reference.png")); err != nil {
		t.Fatalf("expected staged compared reference image asset: %v", err)
	}
}

func TestGenerateWorkflowReportRendersMultiScreenStoryComparison(t *testing.T) {
	tempDir := t.TempDir()
	patientReferencePath := filepath.Join(tempDir, "patient-reference.png")
	recommendationReferencePath := filepath.Join(tempDir, "recommendation-reference.png")
	patientCurrentPath := filepath.Join(tempDir, "artifacts", "current", "patient.png")
	recommendationCurrentPath := filepath.Join(tempDir, "artifacts", "current", "recommendation.png")

	for _, fixture := range []struct {
		path   string
		pixels []color.NRGBA
	}{
		{
			path: patientReferencePath,
			pixels: []color.NRGBA{
				{R: 255, G: 255, B: 255, A: 255},
				{R: 245, G: 245, B: 255, A: 255},
			},
		},
		{
			path: recommendationReferencePath,
			pixels: []color.NRGBA{
				{R: 255, G: 255, B: 255, A: 255},
				{R: 255, G: 245, B: 245, A: 255},
			},
		},
		{
			path: patientCurrentPath,
			pixels: []color.NRGBA{
				{R: 255, G: 255, B: 255, A: 255},
				{R: 235, G: 235, B: 255, A: 255},
			},
		},
		{
			path: recommendationCurrentPath,
			pixels: []color.NRGBA{
				{R: 255, G: 255, B: 255, A: 255},
				{R: 255, G: 235, B: 235, A: 255},
			},
		},
	} {
		if err := writeTestPNG(fixture.path, fixture.pixels); err != nil {
			t.Fatalf("writeTestPNG %s: %v", filepath.Base(fixture.path), err)
		}
	}

	scenario := WorkflowScenario{
		ID:         "report-multi-story-test",
		Label:      "Report Multi Story Test",
		SourceDir:  tempDir,
		SourcePath: filepath.Join(tempDir, "report-multi-story-test.yaml"),
		Stories: map[string]WorkflowStory{
			"D1": {ID: "D1", Title: "Provider reviews multiple linked screens"},
		},
		Environment: WorkflowEnvironment{
			BaseURL:      "http://localhost:8383",
			ReportFormat: WorkflowReportFormatHTML,
		},
		Artifacts: WorkflowArtifactLayout{
			Root:       filepath.Join(tempDir, "artifacts"),
			CurrentDir: filepath.Join(tempDir, "artifacts", "current"),
			DiffDir:    filepath.Join(tempDir, "artifacts", "diff"),
			ReportDir:  filepath.Join(tempDir, "artifacts", "report"),
		},
		Screens: []WorkflowScreen{
			{
				ID:             "patient",
				Label:          "Patient",
				Route:          "/triage/patient",
				OutputName:     "patient",
				ReferenceImage: patientReferencePath,
				PrimaryStories: []string{"D1"},
			},
			{
				ID:             "recommendation",
				Label:          "Recommendation",
				Route:          "/triage/recommendation",
				OutputName:     "recommendation",
				ReferenceImage: recommendationReferencePath,
				PrimaryStories: []string{"D1"},
			},
		},
	}

	service := NewService(nil)
	result, err := service.GenerateWorkflowReport(context.Background(), WorkflowReportRequest{Scenario: scenario})
	if err != nil {
		t.Fatalf("GenerateWorkflowReport returned error: %v", err)
	}
	if len(result.Stories) != 1 {
		t.Fatalf("unexpected story count: %d", len(result.Stories))
	}
	if got := result.Stories[0].ScreenIDs; len(got) != 2 || got[0] != "patient" || got[1] != "recommendation" {
		t.Fatalf("unexpected story screen ids: %#v", got)
	}

	reportHTML, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report html: %v", err)
	}
	reportMarkup := string(reportHTML)
	for _, expected := range []string{
		`data-story="D1"`,
		`data-screen-id="patient" data-thumb-index="0"`,
		`data-screen-id="recommendation" data-thumb-index="1"`,
		`const storedIndex = storyCompareIndex[storyID];`,
		`const initialIndex = Number.isInteger(storedIndex) && storedIndex >= 0 && storedIndex < thumbs.length`,
		`setStoryCompareScreen(storyID, initialIndex);`,
	} {
		if !strings.Contains(reportMarkup, expected) {
			t.Fatalf("expected report html to contain %q", expected)
		}
	}
	if strings.Count(reportMarkup, "data-thumb-index=") != 2 {
		t.Fatalf("expected exactly two story comparison thumbnails, got %d", strings.Count(reportMarkup, "data-thumb-index="))
	}
}

func writeWorkflowStoryFixture(t *testing.T, path string) {
	t.Helper()
	payload := `# Workflow Stories

| Code | Priority | User Story | Acceptance Criteria | Notes |
| ---- | -------- | ---------- | ------------------- | ----- |
| D1 | P1 | As a tester, I want to review a captured page. | Page renders<br>Capture is stable | Fixture story |

## Wireframe References
`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write story fixture: %v", err)
	}
}

func writeWorkflowScenarioFixture(t *testing.T, dir string) string {
	t.Helper()
	storyPath := filepath.Join(dir, "stories.md")
	referencePath := filepath.Join(dir, "reference.png")
	writeWorkflowStoryFixture(t, storyPath)
	if err := writeTestPNG(referencePath, []color.NRGBA{{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatalf("write reference fixture: %v", err)
	}
	scenarioPath := filepath.Join(dir, "scenario.yaml")
	payload := `id: fixture-flow
label: Fixture Flow
story_source: stories.md
environment:
  base_url: http://localhost:8383
artifacts:
  root: artifacts
  reference_dir: .
defaults:
  viewport:
    width: 1024
    height: 1200
  full_page: true
stories: {}
screens:
  - id: patient
    label: Patient
    route: /triage/patient
    output_name: patient
    reference_image: reference.png
    primary_stories:
      - D1
`
	if err := os.WriteFile(scenarioPath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write scenario fixture: %v", err)
	}
	return scenarioPath
}
