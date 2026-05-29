package webcap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chromedp/chromedp"
)

func verifyURLGuards(guards CaptureGuards, finalURL string) error {
	_, err := evaluateURLGuardOutcomes(guards, finalURL)
	return err
}

func evaluateURLGuardOutcomes(guards CaptureGuards, finalURL string) ([]GuardOutcome, error) {
	finalURL = strings.TrimSpace(finalURL)
	var outcomes []GuardOutcome
	if guards.ExpectURL != "" {
		matched := strings.Contains(finalURL, guards.ExpectURL)
		outcomes = append(outcomes, GuardOutcome{
			Kind:     "expect_url",
			Value:    guards.ExpectURL,
			FinalURL: finalURL,
			Matched:  matched,
			Status:   guardOutcomeStatus(matched),
		})
		if !matched {
			return outcomes, newCaptureError(CodeCapture, "verify_url_guard", "final URL did not contain expected substring", nil).
				WithMetadata("guard", "expect_url").
				WithMetadata("expect_url", guards.ExpectURL).
				WithMetadata("final_url", finalURL)
		}
	}
	for _, forbidden := range guards.FailOnURL {
		matched := strings.Contains(finalURL, forbidden)
		passed := !matched
		outcomes = append(outcomes, GuardOutcome{
			Kind:     "fail_on_url",
			Value:    forbidden,
			FinalURL: finalURL,
			Matched:  matched,
			Status:   guardOutcomeStatus(passed),
		})
		if matched {
			return outcomes, newCaptureError(CodeCapture, "verify_url_guard", "final URL matched forbidden substring", nil).
				WithMetadata("guard", "fail_on_url").
				WithMetadata("fail_on_url", forbidden).
				WithMetadata("final_url", finalURL)
		}
	}
	return outcomes, nil
}

func currentChromiumURL(ctx context.Context) (string, error) {
	var finalURL string
	if err := chromedp.Run(ctx, chromedp.Location(&finalURL)); err != nil {
		return "", wrapCaptureError("capture_final_url", err)
	}
	return strings.TrimSpace(finalURL), nil
}

func verifyChromiumSelectorGuards(ctx context.Context, guards CaptureGuards, finalURL string) error {
	_, err := evaluateChromiumSelectorGuardOutcomes(ctx, guards, finalURL)
	return err
}

func evaluateChromiumSelectorGuardOutcomes(ctx context.Context, guards CaptureGuards, finalURL string) ([]GuardOutcome, error) {
	if len(guards.FailOnSelector) == 0 {
		return nil, nil
	}
	script, err := selectorGuardScript(guards.FailOnSelector)
	if err != nil {
		return nil, err
	}
	var matched string
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &matched)); err != nil {
		return nil, wrapCaptureError("verify_selector_guard", err)
	}
	outcomes := selectorGuardOutcomes(guards.FailOnSelector, finalURL, matched)
	if matched == "" {
		return outcomes, nil
	}
	return outcomes, newCaptureError(CodeCapture, "verify_selector_guard", "page matched forbidden selector", nil).
		WithMetadata("guard", "fail_on_selector").
		WithMetadata("selector", matched).
		WithMetadata("final_url", strings.TrimSpace(finalURL))
}

func selectorGuardOutcomes(selectors []string, finalURL, matched string) []GuardOutcome {
	selectors = normalizeStrings(selectors)
	if len(selectors) == 0 {
		return nil
	}
	finalURL = strings.TrimSpace(finalURL)
	matched = strings.TrimSpace(matched)
	outcomes := make([]GuardOutcome, 0, len(selectors))
	for _, selector := range selectors {
		isMatch := selector == matched
		outcomes = append(outcomes, GuardOutcome{
			Kind:     "fail_on_selector",
			Value:    selector,
			FinalURL: finalURL,
			Matched:  isMatch,
			Status:   guardOutcomeStatus(!isMatch),
		})
	}
	return outcomes
}

func guardOutcomeStatus(passed bool) string {
	if passed {
		return "passed"
	}
	return "failed"
}

func selectorGuardScript(selectors []string) (string, error) {
	encoded, err := json.Marshal(normalizeStrings(selectors))
	if err != nil {
		return "", wrapCaptureError("verify_selector_guard", err)
	}
	return fmt.Sprintf(`(() => {
  const selectors = %s;
  for (const selector of selectors) {
    const nodes = Array.from(document.querySelectorAll(selector));
    for (const node of nodes) {
      const style = window.getComputedStyle(node);
      const rect = node.getBoundingClientRect();
      if (style && style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0) {
        return selector;
      }
    }
  }
  return "";
})()`, string(encoded)), nil
}
