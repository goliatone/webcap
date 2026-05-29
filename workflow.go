package webcap

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultWorkflowHandoffQueryParam = "workflow_handoff"

func DefaultWorkflowOptions() WorkflowOptions {
	return WorkflowOptions{
		HandoffQueryParam: defaultWorkflowHandoffQueryParam,
		BuildHandoff:      DefaultWorkflowHandoff,
	}
}

func DefaultWorkflowHandoff(scenario WorkflowScenario) (string, error) {
	handoff := WorkflowHandoff{
		SelectedScenario: strings.TrimSpace(scenario.Environment.SelectedScenario),
		PresentationMode: strings.TrimSpace(scenario.Environment.PresentationMode),
		RuntimeOverrides: normalizeStringMap(scenario.Environment.RuntimeOverrides),
	}
	if handoff.SelectedScenario == "" && handoff.PresentationMode == "" && len(handoff.RuntimeOverrides) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(handoff)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func LoadWorkflowScenarioWithOptions(path string, opts WorkflowOptions) (WorkflowScenario, error) {
	opts = opts.normalized()
	return loadWorkflowScenario(path, opts)
}

func LoadWorkflowScenario(path string) (WorkflowScenario, error) {
	return loadWorkflowScenario(path, WorkflowOptions{})
}

func loadWorkflowScenario(path string, opts WorkflowOptions) (WorkflowScenario, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return WorkflowScenario{}, newCaptureError(CodeValidation, "load_workflow_scenario", "scenario path is required", nil)
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return WorkflowScenario{}, wrapCaptureError("load_workflow_scenario", err)
	}
	payload, err := os.ReadFile(absolutePath) // #nosec G304 -- workflow scenarios are explicit user-supplied files.
	if err != nil {
		return WorkflowScenario{}, wrapCaptureError("load_workflow_scenario", err)
	}

	var scenario WorkflowScenario
	if err := yaml.Unmarshal(payload, &scenario); err != nil {
		return WorkflowScenario{}, newCaptureError(CodeValidation, "load_workflow_scenario", "invalid workflow scenario yaml", err)
	}
	scenario.SourcePath = absolutePath
	scenario.SourceDir = filepath.Dir(absolutePath)
	if err := normalizeWorkflowScenario(&scenario, opts); err != nil {
		return WorkflowScenario{}, err
	}
	return scenario, nil
}

func (opts WorkflowOptions) normalized() WorkflowOptions {
	opts.DefaultSelectedScenario = strings.TrimSpace(opts.DefaultSelectedScenario)
	opts.DefaultPresentationMode = strings.TrimSpace(opts.DefaultPresentationMode)
	opts.HandoffQueryParam = strings.TrimSpace(opts.HandoffQueryParam)
	if opts.HandoffQueryParam == "" && opts.BuildHandoff != nil {
		opts.HandoffQueryParam = defaultWorkflowHandoffQueryParam
	}
	return opts
}

func normalizeWorkflowScenario(scenario *WorkflowScenario, opts WorkflowOptions) error {
	if scenario == nil {
		return newCaptureError(CodeValidation, "normalize_workflow_scenario", "workflow scenario is required", nil)
	}
	opts = opts.normalized()
	if err := normalizeWorkflowScenarioIdentity(scenario); err != nil {
		return err
	}
	if err := normalizeWorkflowEnvironment(scenario, opts); err != nil {
		return err
	}
	normalizeWorkflowArtifacts(scenario)
	if err := normalizeWorkflowDefaults(scenario); err != nil {
		return err
	}
	if err := normalizeWorkflowStories(scenario); err != nil {
		return err
	}
	return normalizeWorkflowScreens(scenario)
}

func normalizeWorkflowScenarioIdentity(scenario *WorkflowScenario) error {
	scenario.ID = strings.TrimSpace(scenario.ID)
	scenario.Label = strings.TrimSpace(scenario.Label)
	scenario.Description = strings.TrimSpace(scenario.Description)
	scenario.StorySource = strings.TrimSpace(scenario.StorySource)
	scenario.Context = cloneMap(scenario.Context)
	scenario.Hooks = normalizeWorkflowHooks(scenario.Hooks)
	if scenario.ID == "" {
		return newCaptureError(CodeValidation, "normalize_workflow_scenario", "workflow scenario id is required", nil)
	}
	if scenario.Label == "" {
		scenario.Label = scenario.ID
	}
	if scenario.StorySource != "" {
		scenario.StorySource = resolveWorkflowPath(scenario.SourceDir, scenario.StorySource)
	}
	return nil
}

func normalizeWorkflowEnvironment(scenario *WorkflowScenario, opts WorkflowOptions) error {
	if scenario.Environment.BaseURL = strings.TrimSpace(scenario.Environment.BaseURL); scenario.Environment.BaseURL == "" {
		return newCaptureError(CodeValidation, "normalize_workflow_scenario", "workflow environment base_url is required", nil)
	}
	if scenario.Environment.ReportFormat = normalizeWorkflowReportFormat(scenario.Environment.ReportFormat); scenario.Environment.ReportFormat == "" {
		scenario.Environment.ReportFormat = WorkflowReportFormatHTML
	}
	if scenario.Environment.ReportFormat != WorkflowReportFormatHTML {
		return newCaptureError(CodeValidation, "normalize_workflow_scenario", fmt.Sprintf("unsupported workflow report format %q", scenario.Environment.ReportFormat), nil)
	}
	scenario.Environment.Engine = strings.TrimSpace(scenario.Environment.Engine)
	scenario.Environment.BrowserPath = strings.TrimSpace(scenario.Environment.BrowserPath)
	scenario.Environment.PlaywrightBrowser = strings.TrimSpace(scenario.Environment.PlaywrightBrowser)
	scenario.Environment.PlaywrightNodeBinary = strings.TrimSpace(scenario.Environment.PlaywrightNodeBinary)
	scenario.Environment.PlaywrightRuntimeDir = strings.TrimSpace(scenario.Environment.PlaywrightRuntimeDir)
	if scenario.Environment.PlaywrightRuntimeDir != "" {
		scenario.Environment.PlaywrightRuntimeDir = resolveWorkflowPath(scenario.SourceDir, scenario.Environment.PlaywrightRuntimeDir)
	}
	scenario.Environment.FixtureMode = strings.TrimSpace(scenario.Environment.FixtureMode)
	scenario.Environment.SelectedScenario = strings.TrimSpace(scenario.Environment.SelectedScenario)
	if scenario.Environment.SelectedScenario == "" {
		scenario.Environment.SelectedScenario = opts.DefaultSelectedScenario
	}
	scenario.Environment.PresentationMode = strings.TrimSpace(scenario.Environment.PresentationMode)
	if scenario.Environment.PresentationMode == "" {
		scenario.Environment.PresentationMode = opts.DefaultPresentationMode
	}
	scenario.Environment.RuntimeOverrides = normalizeStringMap(scenario.Environment.RuntimeOverrides)
	scenario.Environment.Query = normalizeStringMap(scenario.Environment.Query)
	return nil
}

func normalizeWorkflowArtifacts(scenario *WorkflowScenario) {
	scenario.Artifacts.Root = resolveWorkflowPath(scenario.SourceDir, firstNonEmpty(strings.TrimSpace(scenario.Artifacts.Root), "artifacts"))
	scenario.Artifacts.CurrentDir = resolveWorkflowPath(scenario.Artifacts.Root, firstNonEmpty(strings.TrimSpace(scenario.Artifacts.CurrentDir), "current"))
	scenario.Artifacts.DiffDir = resolveWorkflowPath(scenario.Artifacts.Root, firstNonEmpty(strings.TrimSpace(scenario.Artifacts.DiffDir), "diff"))
	scenario.Artifacts.ReportDir = resolveWorkflowPath(scenario.Artifacts.Root, firstNonEmpty(strings.TrimSpace(scenario.Artifacts.ReportDir), "report"))
	if strings.TrimSpace(scenario.Artifacts.ReferenceDir) != "" {
		scenario.Artifacts.ReferenceDir = resolveWorkflowPath(scenario.SourceDir, scenario.Artifacts.ReferenceDir)
	}
}

func normalizeWorkflowDefaults(scenario *WorkflowScenario) error {
	var err error
	scenario.Defaults.Comparison, err = normalizeWorkflowComparison(scenario.Defaults.Comparison)
	if err != nil {
		return err
	}
	scenario.Defaults.SemanticDiff, err = normalizeWorkflowSemanticDiff(scenario.Defaults.SemanticDiff)
	if err != nil {
		return err
	}
	if scenario.Defaults.Readiness == "" {
		scenario.Defaults.Readiness = defaultReadinessMode
	}
	if scenario.Defaults.Viewport.Width == 0 && scenario.Defaults.Viewport.Height == 0 && scenario.Defaults.ViewportPreset == "" && scenario.Defaults.DevicePreset == "" {
		scenario.Defaults.Viewport.Width = 1024
		scenario.Defaults.Viewport.Height = defaultViewportHeight
	}
	if scenario.Defaults.FullPage == nil {
		fullPage := true
		scenario.Defaults.FullPage = &fullPage
	}
	return nil
}

func normalizeWorkflowStories(scenario *WorkflowScenario) error {
	stories := map[string]WorkflowStory{}
	if scenario.StorySource != "" {
		parsedStories, err := parseWorkflowStories(scenario.StorySource)
		if err != nil {
			return err
		}
		maps.Copy(stories, parsedStories)
	}
	for id, story := range scenario.Stories {
		normalizedStory := normalizeWorkflowStory(id, story)
		if normalizedStory.ID == "" {
			return newCaptureError(CodeValidation, "normalize_workflow_scenario", "workflow story id is required", nil)
		}
		merged := stories[normalizedStory.ID]
		stories[normalizedStory.ID] = mergeWorkflowStory(merged, normalizedStory)
	}
	scenario.Stories = stories
	return nil
}

func normalizeWorkflowScreens(scenario *WorkflowScenario) error {
	if len(scenario.Screens) == 0 {
		return newCaptureError(CodeValidation, "normalize_workflow_scenario", "workflow scenario must define at least one screen", nil)
	}
	screenIDs := map[string]struct{}{}
	evidenceIDs := map[string]struct{}{}
	for index := range scenario.Screens {
		screen := &scenario.Screens[index]
		if err := normalizeWorkflowScreen(screen, *scenario); err != nil {
			return newCaptureError(CodeValidation, "normalize_workflow_scenario", fmt.Sprintf("workflow screen %d is invalid", index+1), err)
		}
		if _, exists := screenIDs[screen.ID]; exists {
			return newCaptureError(CodeValidation, "normalize_workflow_scenario", fmt.Sprintf("duplicate workflow screen id %q", screen.ID), nil)
		}
		screenIDs[screen.ID] = struct{}{}
		for _, storyID := range append(append([]string(nil), screen.PrimaryStories...), screen.SupportingStories...) {
			if _, ok := scenario.Stories[storyID]; !ok {
				return newCaptureError(CodeValidation, "normalize_workflow_scenario", fmt.Sprintf("screen %q references unknown story %q", screen.ID, storyID), nil)
			}
		}
		for _, item := range screen.Evidence {
			key := item.ID
			if _, exists := evidenceIDs[key]; exists {
				return newCaptureError(CodeValidation, "normalize_workflow_scenario", fmt.Sprintf("duplicate workflow evidence id %q", key), nil)
			}
			evidenceIDs[key] = struct{}{}
			for _, storyID := range item.Stories {
				if _, ok := scenario.Stories[storyID]; !ok {
					return newCaptureError(CodeValidation, "normalize_workflow_scenario", fmt.Sprintf("evidence %q references unknown story %q", item.ID, storyID), nil)
				}
			}
		}
	}
	return nil
}

func normalizeWorkflowScreen(screen *WorkflowScreen, scenario WorkflowScenario) error {
	if screen == nil {
		return newCaptureError(CodeValidation, "normalize_workflow_screen", "workflow screen is required", nil)
	}
	screen.ID = sanitizeName(screen.ID)
	screen.Label = strings.TrimSpace(screen.Label)
	screen.Route = strings.TrimSpace(screen.Route)
	screen.OutputName = sanitizeName(screen.OutputName)
	screen.ReferenceImage = strings.TrimSpace(screen.ReferenceImage)
	screen.PrimaryStories = normalizeStrings(screen.PrimaryStories)
	screen.SupportingStories = normalizeStrings(screen.SupportingStories)
	screen.ExpectedElements = normalizeStrings(screen.ExpectedElements)
	screen.Notes = normalizeStrings(screen.Notes)
	screen.Annotations = normalizeStrings(screen.Annotations)
	screen.Query = normalizeStringMap(screen.Query)
	screen.Hooks = normalizeWorkflowHooks(screen.Hooks)
	screen.Comparison = mergeWorkflowComparison(scenario.Defaults.Comparison, screen.Comparison)
	screen.SemanticDiff = mergeWorkflowSemanticDiff(scenario.Defaults.SemanticDiff, screen.SemanticDiff)
	var err error
	screen.Comparison, err = normalizeWorkflowComparison(screen.Comparison)
	if err != nil {
		return err
	}
	screen.SemanticDiff, err = normalizeWorkflowSemanticDiff(screen.SemanticDiff)
	if err != nil {
		return err
	}
	if screen.SemanticDiff.PromptPath != "" {
		screen.SemanticDiff.PromptPath = resolveWorkflowPath(scenario.SourceDir, screen.SemanticDiff.PromptPath)
	}
	if screen.SemanticDiff.RawResponsePath != "" {
		screen.SemanticDiff.RawResponsePath = resolveWorkflowPath(scenario.SourceDir, screen.SemanticDiff.RawResponsePath)
	}
	if screen.ID == "" {
		return newCaptureError(CodeValidation, "normalize_workflow_screen", "workflow screen id is required", nil)
	}
	if screen.Label == "" {
		screen.Label = strings.ReplaceAll(screen.ID, "-", " ")
	}
	if screen.Route == "" {
		return newCaptureError(CodeValidation, "normalize_workflow_screen", "workflow screen route is required", nil)
	}
	if !strings.HasPrefix(screen.Route, "/") {
		return newCaptureError(CodeValidation, "normalize_workflow_screen", "workflow screen route must start with /", nil)
	}
	if screen.OutputName == "" {
		screen.OutputName = screen.ID
	}
	if screen.ReferenceImage == "" {
		return newCaptureError(CodeValidation, "normalize_workflow_screen", "workflow screen reference_image is required", nil)
	}
	if strings.TrimSpace(scenario.Artifacts.ReferenceDir) != "" && !filepath.IsAbs(screen.ReferenceImage) {
		screen.ReferenceImage = filepath.Join(scenario.Artifacts.ReferenceDir, screen.ReferenceImage)
	} else {
		screen.ReferenceImage = resolveWorkflowPath(scenario.SourceDir, screen.ReferenceImage)
	}
	if _, err := os.Stat(screen.ReferenceImage); err != nil {
		return newCaptureError(CodeValidation, "normalize_workflow_screen", fmt.Sprintf("workflow screen reference_image %q not found", screen.ReferenceImage), err)
	}
	for index := range screen.Evidence {
		screen.Evidence[index].ID = strings.TrimSpace(screen.Evidence[index].ID)
		screen.Evidence[index].Text = strings.TrimSpace(screen.Evidence[index].Text)
		screen.Evidence[index].Stories = normalizeStrings(screen.Evidence[index].Stories)
		screen.Evidence[index].ExpectedSelectors = normalizeStrings(screen.Evidence[index].ExpectedSelectors)
		screen.Evidence[index].Notes = normalizeStrings(screen.Evidence[index].Notes)
		if screen.Evidence[index].ID == "" || screen.Evidence[index].Text == "" {
			return newCaptureError(CodeValidation, "normalize_workflow_screen", fmt.Sprintf("workflow screen %q has invalid evidence item", screen.ID), nil)
		}
	}
	return nil
}

func normalizeWorkflowStory(id string, story WorkflowStory) WorkflowStory {
	story.ID = strings.TrimSpace(firstNonEmpty(story.ID, id))
	story.Priority = strings.TrimSpace(story.Priority)
	story.Title = strings.TrimSpace(story.Title)
	story.AcceptanceCriteria = normalizeStrings(story.AcceptanceCriteria)
	story.Notes = normalizeStrings(story.Notes)
	return story
}

func mergeWorkflowStory(base, override WorkflowStory) WorkflowStory {
	if override.ID != "" {
		base.ID = override.ID
	}
	if override.Priority != "" {
		base.Priority = override.Priority
	}
	if override.Title != "" {
		base.Title = override.Title
	}
	if len(override.AcceptanceCriteria) > 0 {
		base.AcceptanceCriteria = append([]string(nil), override.AcceptanceCriteria...)
	}
	if len(override.Notes) > 0 {
		base.Notes = append([]string(nil), override.Notes...)
	}
	return base
}

func normalizeWorkflowHooks(hooks WorkflowHooks) WorkflowHooks {
	hooks.AuthSetup = normalizeWorkflowHookSet(hooks.AuthSetup)
	hooks.StateSetup = normalizeWorkflowHookSet(hooks.StateSetup)
	hooks.Navigation = normalizeWorkflowHookSet(hooks.Navigation)
	return hooks
}

func normalizeWorkflowHookSet(set WorkflowHookSet) WorkflowHookSet {
	set.BeforeNavigate = strings.TrimSpace(set.BeforeNavigate)
	set.AfterNavigate = strings.TrimSpace(set.AfterNavigate)
	set.BeforeCapture = strings.TrimSpace(set.BeforeCapture)
	set.Mode = strings.TrimSpace(strings.ToLower(set.Mode))
	switch set.Mode {
	case "", WorkflowHookModeAppend:
		set.Mode = WorkflowHookModeAppend
	case WorkflowHookModeReplace:
	default:
		set.Mode = WorkflowHookModeAppend
	}
	set.Context = cloneMap(set.Context)
	return set
}

func (s *Service) CaptureScenario(ctx context.Context, scenario WorkflowScenario) (WorkflowCaptureResult, error) {
	if s == nil || s.engine == nil {
		return WorkflowCaptureResult{}, newCaptureError(CodeCapture, "capture_scenario", "webcap engine is not configured", nil)
	}
	if err := normalizeWorkflowScenario(&scenario, s.workflow); err != nil {
		return WorkflowCaptureResult{}, err
	}

	results := make([]WorkflowScreenCaptureResult, 0, len(scenario.Screens))
	for _, screen := range scenario.Screens {
		req, targetURL, err := synthesizeWorkflowCaptureRequest(scenario, screen, s.workflow)
		if err != nil {
			return WorkflowCaptureResult{}, err
		}
		result, err := s.Capture(ctx, req)
		if err != nil {
			return WorkflowCaptureResult{}, wrapCaptureError("capture_scenario", err)
		}
		results = append(results, WorkflowScreenCaptureResult{
			ScreenID:         screen.ID,
			Label:            screen.Label,
			Route:            screen.Route,
			TargetURL:        targetURL,
			OutputPath:       result.OutputPath,
			MetadataPath:     result.MetadataPath,
			ReferenceImage:   screen.ReferenceImage,
			Capture:          result,
			ExpectedElements: append([]string(nil), screen.ExpectedElements...),
		})
	}

	return WorkflowCaptureResult{
		ScenarioID:   scenario.ID,
		ScenarioPath: scenario.SourcePath,
		CurrentDir:   scenario.Artifacts.CurrentDir,
		CapturedAt:   s.now(),
		Results:      results,
	}, nil
}

func synthesizeWorkflowCaptureRequest(scenario WorkflowScenario, screen WorkflowScreen, opts WorkflowOptions) (CaptureRequest, string, error) {
	targetURL, err := workflowScreenURL(scenario, screen, opts)
	if err != nil {
		return CaptureRequest{}, "", err
	}
	request := screen.Capture
	request.URL = targetURL
	request.OutputPath = filepath.Join(scenario.Artifacts.CurrentDir, screen.OutputName+"."+defaultImageFormat)
	request.MetadataPath = request.OutputPath + ".json"
	applyWorkflowCaptureDefaults(&request, scenario.Defaults)
	request.Auth = resolveCaptureAuthPaths(scenario.SourceDir, request.Auth)
	request.BeforeNavigateJS = buildWorkflowHookScript(scenario, screen, "before_navigate", request)
	request.AfterNavigateJS = buildWorkflowHookScript(scenario, screen, "after_navigate", request)
	request.BeforeCaptureJS = buildWorkflowHookScript(scenario, screen, "before_capture", request)
	normalized, err := NormalizeCaptureRequest(request)
	if err != nil {
		return CaptureRequest{}, "", err
	}
	return normalized, targetURL, nil
}

func applyWorkflowCaptureDefaults(request *CaptureRequest, defaults WorkflowDefaults) {
	if request == nil {
		return
	}
	if request.Viewport == (Viewport{}) && defaults.Viewport != (Viewport{}) {
		request.Viewport = defaults.Viewport
	}
	if request.ViewportPreset == "" {
		request.ViewportPreset = defaults.ViewportPreset
	}
	if request.DevicePreset == "" {
		request.DevicePreset = defaults.DevicePreset
	}
	if request.Readiness == "" {
		request.Readiness = defaults.Readiness
	}
	if request.ReadinessIdle == "" {
		request.ReadinessIdle = defaults.ReadinessIdle
	}
	if !request.DisableAnimations {
		request.DisableAnimations = defaults.DisableAnimations
	}
	if !request.ReducedMotion {
		request.ReducedMotion = defaults.ReducedMotion
	}
	if !request.WaitForFonts {
		request.WaitForFonts = defaults.WaitForFonts
	}
	if request.Timeout == "" {
		request.Timeout = defaults.Timeout
	}
	if request.Wait == "" {
		request.Wait = defaults.Wait
	}
	if request.WaitFor == "" {
		request.WaitFor = defaults.WaitFor
	}
	if request.WaitForFunction == "" {
		request.WaitForFunction = defaults.WaitForFunction
	}
	request.Auth = mergeCaptureAuth(defaults.Auth, request.Auth)
	request.Guards = mergeCaptureGuards(defaults.Guards, request.Guards)
	if request.OversizePolicy == "" {
		request.OversizePolicy = defaults.OversizePolicy
	}
	request.Tile = mergeTileOptions(defaults.Tile, request.Tile)
	if request.Mode() == CaptureModeViewport && defaults.FullPage != nil {
		request.FullPage = *defaults.FullPage
	}
}

func workflowScreenURL(scenario WorkflowScenario, screen WorkflowScreen, opts WorkflowOptions) (string, error) {
	opts = opts.normalized()
	baseURL, err := url.Parse(strings.TrimSpace(scenario.Environment.BaseURL))
	if err != nil {
		return "", newCaptureError(CodeValidation, "workflow_screen_url", "invalid workflow base_url", err)
	}
	routeURL, err := baseURL.Parse(screen.Route)
	if err != nil {
		return "", wrapCaptureError("workflow_screen_url", err)
	}

	query := routeURL.Query()
	if scenario.Environment.SelectedScenario != "" {
		query.Set("scenario", scenario.Environment.SelectedScenario)
	}
	if scenario.Environment.PresentationMode != "" {
		query.Set("presentation_mode", scenario.Environment.PresentationMode)
	}
	if opts.BuildHandoff != nil {
		handoff, err := opts.BuildHandoff(scenario)
		if err != nil {
			return "", wrapCaptureError("workflow_handoff", err)
		}
		if strings.TrimSpace(handoff) != "" {
			query.Set(opts.HandoffQueryParam, strings.TrimSpace(handoff))
		}
	}
	for key, value := range scenario.Environment.Query {
		query.Set(key, value)
	}
	for key, value := range screen.Query {
		query.Set(key, value)
	}
	routeURL.RawQuery = query.Encode()
	return routeURL.String(), nil
}

func buildWorkflowHookScript(scenario WorkflowScenario, screen WorkflowScreen, phase string, request CaptureRequest) string {
	stageScripts := make([]string, 0, 6)
	for _, hookType := range []string{"auth_setup", "state_setup", "navigation"} {
		scenarioSet := workflowHookSetByName(scenario.Hooks, hookType)
		screenSet := workflowHookSetByName(screen.Hooks, hookType)
		if screenSet.Mode != WorkflowHookModeReplace {
			if script := workflowHookPhaseScript(scenarioSet, phase); script != "" {
				stageScripts = append(stageScripts, wrapWorkflowHookScript(script, workflowHookRuntime{
					Phase:      phase,
					Kind:       hookType,
					ScreenID:   screen.ID,
					ScenarioID: scenario.ID,
					Request:    request,
					Context:    mergeWorkflowContext(scenario.Context, scenarioSet.Context),
				}))
			}
		}
		if script := workflowHookPhaseScript(screenSet, phase); script != "" {
			stageScripts = append(stageScripts, wrapWorkflowHookScript(script, workflowHookRuntime{
				Phase:      phase,
				Kind:       hookType,
				ScreenID:   screen.ID,
				ScenarioID: scenario.ID,
				Request:    request,
				Context:    mergeWorkflowContext(scenario.Context, screenSet.Context),
			}))
		}
	}
	return strings.TrimSpace(strings.Join(stageScripts, "\n"))
}

type workflowHookRuntime struct {
	Phase      string         `json:"phase"`
	Kind       string         `json:"kind"`
	ScreenID   string         `json:"screen_id"`
	ScenarioID string         `json:"scenario_id"`
	Request    CaptureRequest `json:"request"`
	Context    map[string]any `json:"context,omitempty"`
}

func wrapWorkflowHookScript(script string, runtime workflowHookRuntime) string {
	script = strings.TrimSpace(script)
	if script == "" {
		return ""
	}
	payload, _ := json.Marshal(runtime)
	return strings.TrimSpace(fmt.Sprintf(`(function () {
  const hook = %s;
  const __webcap = hook;
  const __screen = { id: hook.screen_id };
  const __scenario = { id: hook.scenario_id };
  const __artifacts = { output_path: hook.request.output, metadata_path: hook.request.metadata };
  const __context = hook.context || {};
  %s
})();`, string(payload), script))
}

func workflowHookSetByName(hooks WorkflowHooks, name string) WorkflowHookSet {
	switch strings.TrimSpace(name) {
	case "auth_setup":
		return hooks.AuthSetup
	case "state_setup":
		return hooks.StateSetup
	case "navigation":
		return hooks.Navigation
	default:
		return WorkflowHookSet{}
	}
}

func workflowHookPhaseScript(set WorkflowHookSet, phase string) string {
	switch strings.TrimSpace(phase) {
	case "before_navigate":
		return strings.TrimSpace(set.BeforeNavigate)
	case "after_navigate":
		return strings.TrimSpace(set.AfterNavigate)
	case "before_capture":
		return strings.TrimSpace(set.BeforeCapture)
	default:
		return ""
	}
}

func mergeWorkflowContext(values ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, value := range values {
		for key, item := range value {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			out[key] = item
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeWorkflowReportFormat(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", WorkflowReportFormatHTML:
		return WorkflowReportFormatHTML
	default:
		return value
	}
}

func normalizeStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		cleanKey := strings.TrimSpace(key)
		cleanValue := strings.TrimSpace(values[key])
		if cleanKey == "" || cleanValue == "" {
			continue
		}
		out[cleanKey] = cleanValue
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func resolveWorkflowPath(baseDir, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	if baseDir == "" {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(baseDir, value))
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[strings.TrimSpace(key)] = value
	}
	return out
}

func parseWorkflowStories(path string) (map[string]WorkflowStory, error) {
	payload, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return nil, wrapCaptureError("parse_workflow_stories", err)
	}
	lines := strings.Split(string(payload), "\n")
	stories := map[string]WorkflowStory{}
	inTable := false
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "## Wireframe References") {
			break
		}
		if strings.HasPrefix(line, "| Code | Priority | User Story") {
			inTable = true
			continue
		}
		if !inTable || !strings.HasPrefix(line, "|") {
			continue
		}
		if strings.Contains(line, "| ---- |") {
			continue
		}
		parts := splitMarkdownTableRow(line)
		if len(parts) < 5 {
			continue
		}
		story := WorkflowStory{
			ID:                 strings.TrimSpace(parts[0]),
			Priority:           strings.TrimSpace(parts[1]),
			Title:              strings.TrimSpace(parts[2]),
			AcceptanceCriteria: splitMarkdownCell(parts[3]),
			Notes:              splitMarkdownCell(parts[4]),
		}
		if story.ID == "" {
			continue
		}
		stories[story.ID] = story
	}
	if len(stories) == 0 {
		return nil, newCaptureError(CodeValidation, "parse_workflow_stories", "no workflow stories could be parsed", nil)
	}
	return stories, nil
}

func splitMarkdownTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	rawParts := strings.Split(line, "|")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		parts = append(parts, strings.TrimSpace(part))
	}
	return parts
}

func splitMarkdownCell(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "(no notes)" {
		return nil
	}
	parts := strings.Split(value, "<br>")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, "`"))
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
