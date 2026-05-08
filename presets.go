package webcap

type DeviceProfile struct {
	Name      string   `json:"name"`
	Viewport  Viewport `json:"viewport"`
	UserAgent string   `json:"user_agent,omitempty"`
}

var viewportPresets = map[string]Viewport{
	"desktop-hd":   {Width: 1366, Height: 768, ScaleFactor: 1},
	"desktop-xl":   {Width: 1440, Height: 1200, ScaleFactor: 1},
	"desktop-wide": {Width: 1728, Height: 1117, ScaleFactor: 1},
	"tablet":       {Width: 1024, Height: 1366, ScaleFactor: 2},
	"mobile":       {Width: 390, Height: 844, ScaleFactor: 3, Mobile: true},
}

var devicePresets = map[string]DeviceProfile{
	"iphone-15": {
		Name:      "iphone-15",
		Viewport:  Viewport{Width: 393, Height: 852, ScaleFactor: 3, Mobile: true},
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
	},
	"ipad-air": {
		Name:      "ipad-air",
		Viewport:  Viewport{Width: 820, Height: 1180, ScaleFactor: 2, Mobile: true},
		UserAgent: "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
	},
	"pixel-8": {
		Name:      "pixel-8",
		Viewport:  Viewport{Width: 412, Height: 915, ScaleFactor: 2.625, Mobile: true},
		UserAgent: "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36",
	},
}

func LookupViewportPreset(name string) (Viewport, bool) {
	value, ok := viewportPresets[normalizePresetName(name)]
	return value, ok
}

func LookupDevicePreset(name string) (DeviceProfile, bool) {
	value, ok := devicePresets[normalizePresetName(name)]
	return value, ok
}
