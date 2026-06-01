package webcap

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

//go:embed internal/authscripts/go_admin_login.sh
var defaultGoAdminLoginScript string

var (
	authLookPath       = exec.LookPath
	authCommandContext = exec.CommandContext
)

func LoginAuthState(ctx context.Context, req AuthLoginRequest, now time.Time) (AuthLoginResult, error) {
	req = normalizeAuthLoginRequest(req)
	if now.IsZero() {
		now = time.Now()
	}
	result := AuthLoginResult{
		Command:         "auth login",
		BaseURL:         req.BaseURL,
		LoginPath:       req.LoginPath,
		TargetURL:       req.TargetURL,
		CookieJar:       req.CookieJar,
		ExpectedCookies: append([]string(nil), req.ExpectCookies...),
	}
	if err := validateAuthLoginRequest(req); err != nil {
		return result, err
	}
	password, ok := os.LookupEnv(req.PasswordEnv)
	if !ok {
		return result, newCaptureError(CodeValidation, "auth_login", "password environment variable is not set", nil).
			WithMetadata("password_env", req.PasswordEnv)
	}
	bashPath, err := authLookPath("bash")
	if err != nil {
		return result, newCaptureError(CodeValidation, "auth_login", "bash executable was not found", err)
	}

	scriptPath := req.ScriptPath
	scriptMode := "custom"
	cleanup := func() {}
	if scriptPath == "" {
		scriptMode = "embedded_go_admin"
		scriptPath, cleanup, err = materializeDefaultLoginScript()
		if err != nil {
			return result, err
		}
		defer cleanup()
	} else if err := validateCustomScript(scriptPath); err != nil {
		return result, err
	}
	result.Script.Mode = scriptMode
	if scriptMode == "custom" {
		result.Script.Path = scriptPath
	}

	runCtx := ctx
	cancel := func() {}
	if req.Timeout != "" {
		timeout, parseErr := time.ParseDuration(req.Timeout)
		if parseErr != nil {
			return result, newCaptureError(CodeValidation, "auth_login", "login timeout is invalid", parseErr).
				WithMetadata("timeout", req.Timeout)
		}
		runCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := authCommandContext(runCtx, bashPath, scriptPath)
	cmd.Env = authLoginEnvironment(req, password)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	secrets := append([]string{password}, collectCookieSecrets(req.CookieJar, now)...)
	result.Script.Stdout = RedactAuthSecrets(stdout.String(), secrets...)
	result.Script.Stderr = RedactAuthSecrets(stderr.String(), secrets...)
	if runCtx.Err() != nil {
		result.Script.TimedOut = errors.Is(runCtx.Err(), context.DeadlineExceeded)
		result.Script.Cancelled = errors.Is(runCtx.Err(), context.Canceled) && !result.Script.TimedOut
		return result, newCaptureError(CodeTimeout, "auth_login", "login script timed out or was cancelled", runCtx.Err()).
			WithMetadata("script", result.Script)
	}
	if err != nil {
		result.Script.ExitCode = authExitCode(err)
		return result, newCaptureError(CodeValidation, "auth_login", "login script failed", err).
			WithMetadata("script", result.Script)
	}

	inspection, inspectErr := InspectAuthState(AuthInspectRequest{
		CookieJar:     req.CookieJar,
		TargetURL:     req.TargetURL,
		ExpectCookies: req.ExpectCookies,
	}, now)
	result.Inspection = inspection
	if inspectErr != nil {
		return result, newCaptureError(CodeValidation, "auth_login", "login completed but cookie validation failed", inspectErr).
			WithMetadata("inspection", inspection)
	}
	return result, nil
}

func normalizeAuthLoginRequest(req AuthLoginRequest) AuthLoginRequest {
	req.ScriptPath = strings.TrimSpace(req.ScriptPath)
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	req.LoginPath = strings.TrimSpace(req.LoginPath)
	if req.LoginPath == "" {
		req.LoginPath = DefaultAuthLoginPath
	}
	if !strings.HasPrefix(req.LoginPath, "/") {
		req.LoginPath = "/" + req.LoginPath
	}
	req.TargetURL = strings.TrimSpace(req.TargetURL)
	req.CookieJar = strings.TrimSpace(req.CookieJar)
	req.Identifier = strings.TrimSpace(req.Identifier)
	req.PasswordEnv = strings.TrimSpace(req.PasswordEnv)
	req.ExpectCookies = normalizeStrings(req.ExpectCookies)
	req.Timeout = strings.TrimSpace(req.Timeout)
	return req
}

func validateAuthLoginRequest(req AuthLoginRequest) error {
	switch {
	case req.BaseURL == "":
		return newCaptureError(CodeValidation, "auth_login", "auth login requires base URL", nil).
			WithMetadata("field", "base_url")
	case req.CookieJar == "":
		return newCaptureError(CodeValidation, "auth_login", "auth login requires cookie jar", nil).
			WithMetadata("field", "cookie_jar")
	case req.Identifier == "":
		return newCaptureError(CodeValidation, "auth_login", "auth login requires identifier", nil).
			WithMetadata("field", "identifier")
	case req.PasswordEnv == "":
		return newCaptureError(CodeValidation, "auth_login", "auth login requires password environment variable name", nil).
			WithMetadata("field", "password_env")
	case len(req.ExpectCookies) == 0:
		return newCaptureError(CodeValidation, "auth_login", "auth login requires at least one expected cookie", nil).
			WithMetadata("field", "expect_cookies")
	}
	return nil
}

func authLoginEnvironment(req AuthLoginRequest, password string) []string {
	env := append([]string(nil), os.Environ()...)
	env = append(env,
		"WEBCAP_BASE_URL="+req.BaseURL,
		"WEBCAP_LOGIN_PATH="+req.LoginPath,
		"WEBCAP_COOKIE_JAR="+req.CookieJar,
		"WEBCAP_IDENTIFIER="+req.Identifier,
		"WEBCAP_PASSWORD="+password,
		"WEBCAP_EXPECT_COOKIE="+strings.Join(req.ExpectCookies, ","),
	)
	if req.TargetURL != "" {
		env = append(env, "WEBCAP_TARGET_URL="+req.TargetURL)
	}
	return env
}

func materializeDefaultLoginScript() (string, func(), error) {
	dir, err := os.MkdirTemp("", "webcap-auth-script-*")
	if err != nil {
		return "", func() {}, newCaptureError(CodeWrite, "auth_login", "could not create temporary login script directory", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(dir)
	}
	path := filepath.Join(dir, "go_admin_login.sh")
	if err := os.WriteFile(path, []byte(defaultGoAdminLoginScript), 0o700); err != nil {
		cleanup()
		return "", func() {}, newCaptureError(CodeWrite, "auth_login", "could not write temporary login script", err)
	}
	return path, cleanup, nil
}

func validateCustomScript(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return newCaptureError(CodeValidation, "auth_login", "custom login script is not readable", err).
			WithMetadata("script", path)
	}
	if info.IsDir() {
		return newCaptureError(CodeValidation, "auth_login", "custom login script must be a file", nil).
			WithMetadata("script", path)
	}
	file, err := os.Open(path) // #nosec G304 -- explicit user-supplied script path.
	if err != nil {
		return newCaptureError(CodeValidation, "auth_login", "custom login script is not readable", err).
			WithMetadata("script", path)
	}
	_ = file.Close()
	return nil
}

func collectCookieSecrets(path string, now time.Time) []string {
	cookies, err := loadDiagnosticCookieJar(path, now)
	if err != nil {
		return nil
	}
	values := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Cookie.Value != "" {
			values = append(values, cookie.Cookie.Value)
		}
	}
	return values
}

const sensitiveOutputKeys = `authorization|cookie|set-cookie|x-csrf-token|_token|csrf|token|secret|password|access_token|refresh_token|id_token`

var (
	sensitiveJSONPattern   = regexp.MustCompile(`(?i)(["']?(?:` + sensitiveOutputKeys + `)["']?[[:space:]]*:[[:space:]]*)(["'][^"'\r\n]*["']|[^,}\]\r\n[:space:]]+)`)
	sensitiveKeyValPattern = regexp.MustCompile(`(?im)^([[:space:]]*(?:` + sensitiveOutputKeys + `)[[:space:]]*[:=][[:space:]]*)([^\r\n]*)`)
	sensitiveSpacePattern  = regexp.MustCompile(`(?im)^([[:space:]]*(?:authorization|cookie|set-cookie|x-csrf-token|token|secret|password|access_token|refresh_token|id_token)[[:space:]]+)([^\r\n]+)`)
)

func RedactAuthSecrets(text string, secrets ...string) string {
	redacted := text
	for _, secret := range secrets {
		if secret = strings.TrimSpace(secret); secret != "" {
			redacted = strings.ReplaceAll(redacted, secret, "[REDACTED]")
		}
	}
	redacted = sensitiveJSONPattern.ReplaceAllString(redacted, `${1}"[REDACTED]"`)
	redacted = sensitiveKeyValPattern.ReplaceAllString(redacted, "${1}[REDACTED]")
	redacted = sensitiveSpacePattern.ReplaceAllString(redacted, "${1}[REDACTED]")
	return redacted
}

func authExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func defaultGoAdminScriptForTest() string {
	return fmt.Sprint(defaultGoAdminLoginScript)
}
