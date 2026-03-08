package sectionsnavigationlib

import (
	"fmt"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
)

var ctx = uitest.SharedContext()

type SectionCase struct {
	ID           string
	NavAria      string
	SectionAria  string
	Screenshot   string
	AssertJSExpr string
	AssertFail   string
}

func RunSectionFromMenu(sc *testv1.StepContext, c SectionCase, startAtDefault bool) (testv1.StepRunResult, error) {
	ctx.BeginStep(sc)
	waitSection := 10 * time.Second
	waitClick := 5 * time.Second
	waitAssert := 5 * time.Second
	if err := ctx.EnsureBuiltAndServed(); err != nil {
		return testv1.StepRunResult{}, err
	}

	defaultURL := ctx.AppURL("/#ui-home-docs")
	browserOpts, _, err := uitest.BrowserOptionsFor(defaultURL)
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	navigateURL := strings.TrimSpace(browserOpts.URL)
	if navigateURL == "" {
		navigateURL = defaultURL
	}
	testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
		cfg.BrowserNewTargetURL = navigateURL
	})
	if !startAtDefault {
		browserOpts.SkipNavigateOnReuse = true
	}
	if _, err := sc.EnsureBrowser(browserOpts); err != nil {
		return testv1.StepRunResult{}, err
	}
	if startAtDefault {
		if err := sc.Goto(navigateURL); err != nil {
			return testv1.StepRunResult{}, err
		}
		if err := sc.WaitForAriaLabelAttrEquals("Docs Section", "data-active", "true", waitSection); err != nil {
			return testv1.StepRunResult{}, err
		}
	}

	if err := sc.WaitForAriaLabel("Toggle Global Menu", waitSection); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait("Toggle Global Menu", waitClick); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabelAfterWait(c.NavAria, waitClick); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals(c.SectionAria, "data-active", "true", waitSection); err != nil {
		return testv1.StepRunResult{}, err
	}
	if c.AssertJSExpr != "" {
		if err := uitest.AssertJS(sc, waitAssert, c.AssertJSExpr, c.AssertFail); err != nil {
			return testv1.StepRunResult{}, err
		}
	}
	if uitest.UsesServiceManagedBrowser(sc) {
		sc.Warnf("skipping overlay overlap detection for service-managed chrome src_v3 session")
	} else {
		overlaps, err := sc.DetectOverlayOverlaps(5 * time.Second)
		if err != nil {
			return testv1.StepRunResult{}, err
		}
		sc.Logf("OVERLAP: section=%s check=start", c.ID)
		if len(overlaps) == 0 {
			sc.Logf("OVERLAP: section=%s none", c.ID)
		} else {
			var unexpected []string
			for _, ov := range overlaps {
				line := fmt.Sprintf(
					"OVERLAP: section=%s %s:%s/%s(%s) <-> %s:%s/%s(%s) area=%.1fpx a=%.2f%% b=%.2f%% allowedByMenu=%t",
					c.ID,
					blank(ov.AKind),
					blank(ov.AOverlay), blank(ov.ARole), blank(ov.ASection),
					blank(ov.BKind),
					blank(ov.BOverlay), blank(ov.BRole), blank(ov.BSection),
					ov.Intersection, ov.PercentOfA, ov.PercentOfB, ov.AllowedByMenu,
				)
				sc.Logf(line)
				if !ov.AllowedByMenu {
					unexpected = append(unexpected, line)
				}
			}
			if len(unexpected) > 0 {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected overlay overlap(s): %s", strings.Join(unexpected, " | "))
			}
		}
	}
	if err := uitest.CaptureScreenshot(sc, c.Screenshot); err != nil {
		return testv1.StepRunResult{}, err
	}
	return testv1.StepRunResult{Report: fmt.Sprintf("section %s navigation verified", c.ID)}, nil
}

func blank(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}
