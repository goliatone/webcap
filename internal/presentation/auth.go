package presentation

import (
	"fmt"
	"io"
	"strings"

	pkgwebcap "github.com/goliatone/webcap"
)

func writeAuthInspect(w io.Writer, result pkgwebcap.AuthInspectResult) error {
	if err := writeLines(w,
		"Auth inspect complete",
		fmt.Sprintf("Source: %s %s", result.Source.Type, result.Source.Path),
		fmt.Sprintf("Cookies: %d", len(result.Cookies)),
	); err != nil {
		return err
	}
	if result.TargetURL != "" {
		if _, err := fmt.Fprintf(w, "Target: %s\n", result.TargetURL); err != nil {
			return err
		}
	}
	if result.HasOriginStorage {
		if _, err := fmt.Fprintln(w, "Origin storage: present"); err != nil {
			return err
		}
	}
	if err := writeAuthExpectedCookies(w, result.ExpectedCookies); err != nil {
		return err
	}
	if err := writeAuthCookieSummary(w, result.Cookies); err != nil {
		return err
	}
	return writeAuthWarnings(w, result.Warnings)
}

func writeAuthLogin(w io.Writer, result pkgwebcap.AuthLoginResult) error {
	if err := writeLines(w,
		"Auth login complete",
		fmt.Sprintf("Base URL: %s", result.BaseURL),
		fmt.Sprintf("Login path: %s", result.LoginPath),
		fmt.Sprintf("Cookie jar: %s", result.CookieJar),
		fmt.Sprintf("Script: %s", result.Script.Mode),
	); err != nil {
		return err
	}
	if result.TargetURL != "" {
		if _, err := fmt.Fprintf(w, "Target: %s\n", result.TargetURL); err != nil {
			return err
		}
	}
	return writeAuthExpectedCookies(w, result.Inspection.ExpectedCookies)
}

func writeAuthExpectedCookies(w io.Writer, statuses []pkgwebcap.AuthExpectedCookieStatus) error {
	if len(statuses) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "Expected cookies:"); err != nil {
		return err
	}
	for _, status := range statuses {
		if _, err := fmt.Fprintf(w, "  - %s: %s", status.Name, status.Status); err != nil {
			return err
		}
		if len(status.Reasons) > 0 {
			if _, err := fmt.Fprintf(w, " (%s)", strings.Join(status.Reasons, ", ")); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}

func writeAuthCookieSummary(w io.Writer, cookies []pkgwebcap.AuthCookieDiagnostic) error {
	if len(cookies) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "Cookie diagnostics:"); err != nil {
		return err
	}
	for _, cookie := range cookies {
		status := "active"
		if cookie.Expired {
			status = "expired"
		} else if cookie.Applicable != nil && !*cookie.Applicable {
			status = "not_applicable"
		}
		if _, err := fmt.Fprintf(w, "  - %s domain=%s path=%s status=%s", cookie.Name, cookie.Domain, cookie.Path, status); err != nil {
			return err
		}
		if len(cookie.NotApplicable) > 0 {
			if _, err := fmt.Fprintf(w, " reasons=%s", strings.Join(cookie.NotApplicable, ",")); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}

func writeAuthWarnings(w io.Writer, warnings []pkgwebcap.AuthDiagnosticWarning) error {
	if len(warnings) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "Warnings:"); err != nil {
		return err
	}
	for _, warning := range warnings {
		if _, err := fmt.Fprintf(w, "  - %s: %s", warning.Code, warning.Message); err != nil {
			return err
		}
		if warning.Cookie != "" {
			if _, err := fmt.Fprintf(w, " (%s)", warning.Cookie); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}
