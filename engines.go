package webcap

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type EngineName string

const (
	EngineChromium   EngineName = "chromium"
	EnginePlaywright EngineName = "playwright"
)

type EngineConfig struct {
	EngineName           EngineName
	BrowserPath          string
	Headless             bool
	PlaywrightBrowser    string
	PlaywrightNodeBinary string
	PlaywrightRuntimeDir string
	PlaywrightScriptPath string
}

func NormalizeEngineName(value string) EngineName {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", string(EngineChromium):
		return EngineChromium
	case string(EnginePlaywright):
		return EnginePlaywright
	default:
		return EngineName(strings.TrimSpace(strings.ToLower(value)))
	}
}

func NewEngine(config EngineConfig) (Engine, error) {
	switch NormalizeEngineName(string(config.EngineName)) {
	case EngineChromium:
		return NewChromiumEngine(ChromiumOptions{
			BrowserPath: config.BrowserPath,
			Headless:    config.Headless,
		}), nil
	case EnginePlaywright:
		return NewPlaywrightEngine(PlaywrightOptions{
			NodeBinary:  config.PlaywrightNodeBinary,
			BrowserName: config.PlaywrightBrowser,
			BrowserPath: config.BrowserPath,
			Headless:    config.Headless,
			RuntimeDir:  config.PlaywrightRuntimeDir,
			ScriptPath:  config.PlaywrightScriptPath,
		})
	default:
		return nil, newCaptureError(CodeValidation, "new_engine", fmt.Sprintf("unsupported engine %q", config.EngineName), nil)
	}
}

func DefaultPlaywrightRuntimeDir() string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "playwright_runtime"
	}
	return filepath.Join(filepath.Dir(currentFile), "playwright_runtime")
}
