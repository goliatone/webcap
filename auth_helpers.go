package webcap

import "time"

const DefaultAuthLoginPath = "/admin/login"

var DefaultDebugCookieNames = []string{"admin_debug_session"}

type AuthInspectRequest struct {
	CookieJar        string   `json:"cookie_jar,omitempty"`
	StorageState     string   `json:"storage_state,omitempty"`
	TargetURL        string   `json:"target_url,omitempty"`
	ExpectCookies    []string `json:"expect_cookies,omitempty"`
	WarnDebugCookies []string `json:"warn_debug_cookies,omitempty"`
}

type AuthLoginRequest struct {
	ScriptPath    string   `json:"script,omitempty"`
	BaseURL       string   `json:"base_url"`
	LoginPath     string   `json:"login_path"`
	TargetURL     string   `json:"target_url,omitempty"`
	CookieJar     string   `json:"cookie_jar"`
	Identifier    string   `json:"identifier"`
	PasswordEnv   string   `json:"password_env"`
	ExpectCookies []string `json:"expect_cookies"`
	Timeout       string   `json:"timeout,omitempty"`
}

type AuthStateSource struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

type AuthCookieDiagnostic struct {
	Name            string     `json:"name"`
	Domain          string     `json:"domain,omitempty"`
	Path            string     `json:"path,omitempty"`
	Secure          bool       `json:"secure,omitempty"`
	HTTPOnly        bool       `json:"http_only,omitempty"`
	SameSite        string     `json:"same_site,omitempty"`
	Expires         int64      `json:"expires,omitempty"`
	Expired         bool       `json:"expired"`
	Skipped         bool       `json:"skipped,omitempty"`
	HostOnly        bool       `json:"host_only,omitempty"`
	Applicable      *bool      `json:"applicable,omitempty"`
	NotApplicable   []string   `json:"not_applicable,omitempty"`
	SourceLine      int        `json:"source_line,omitempty"`
	StorageState    bool       `json:"storage_state,omitempty"`
	CheckedAt       *time.Time `json:"checked_at,omitempty"`
	CaptureImportOK bool       `json:"capture_import_ok"`
}

type AuthExpectedCookieStatus struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Applicable  *bool    `json:"applicable,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
	MatchedName string   `json:"matched_name,omitempty"`
}

type AuthDiagnosticWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Cookie  string `json:"cookie,omitempty"`
}

type AuthInspectResult struct {
	Command          string                     `json:"command"`
	Source           AuthStateSource            `json:"source"`
	TargetURL        string                     `json:"target_url,omitempty"`
	Cookies          []AuthCookieDiagnostic     `json:"cookies"`
	ExpectedCookies  []AuthExpectedCookieStatus `json:"expected_cookies,omitempty"`
	Warnings         []AuthDiagnosticWarning    `json:"warnings,omitempty"`
	HasOriginStorage bool                       `json:"has_origin_storage,omitempty"`
	InspectedAt      time.Time                  `json:"inspected_at"`
}

type AuthScriptResult struct {
	Mode      string `json:"mode"`
	Path      string `json:"path,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
	TimedOut  bool   `json:"timed_out,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
}

type AuthLoginResult struct {
	Command         string            `json:"command"`
	BaseURL         string            `json:"base_url"`
	LoginPath       string            `json:"login_path"`
	TargetURL       string            `json:"target_url,omitempty"`
	CookieJar       string            `json:"cookie_jar"`
	ExpectedCookies []string          `json:"expected_cookies"`
	Script          AuthScriptResult  `json:"script"`
	Inspection      AuthInspectResult `json:"inspection"`
}
