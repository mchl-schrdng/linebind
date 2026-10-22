package report

import (
	"bytes"
	"encoding/base64"
	"errors"
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/model"
)

func sampleReport() model.Report {
	return model.Report{
		SchemaVersion: 1, Version: "0.1.0", Repository: "/projects/fieldnotes", Base: "a123456789abcdef", Candidate: "working tree",
		ContractPath: ".linebind.yml", ContractHash: "contract-hash", InputDigest: "input-digest", GeneratedAt: "2026-10-14T12:00:00Z", Status: "crossed", ExitCode: 1,
		Changes: []model.ContractChange{{Path: ".linebind.yml", Change: "modified", Detail: "The candidate removed a rule; the base contract remains in use."}},
		Checks: []model.Check{
			{ID: "client-boundary", Kind: "imports", Reason: "Browser code must not reach the database.", Required: true, Status: "crossed", Evidence: "native", Scope: []string{"src/client/**"}, Files: 12,
				Findings: []model.Finding{{Path: "src/client/account.ts", Line: 4, Target: "src/server/db.ts", Message: "A new dependency crosses the client boundary.", Remediation: "Use the account API.", Evolution: "new"}, {Path: "src/client/legacy.ts", Message: "An existing exception is still present.", Evolution: "existing"}}},
			{ID: "api-compatibility", Kind: "command", Required: false, Status: "unverified", Evidence: "structured", Scope: []string{"api/openapi.yml"}, Diagnostics: []string{"The adapter did not return a valid result."}},
			{ID: "dependency-policy", Kind: "dependencies", Required: true, Status: "passed", Evidence: "native", Scope: []string{"package.json"}, Files: 1,
				Findings: []model.Finding{{Path: "package.json", Message: "The prohibited package was removed.", Evolution: "resolved"}}},
		},
	}
}

func TestHTMLRetainsEvidenceAndIncompleteResults(t *testing.T) {
	result := sampleReport()
	var out bytes.Buffer
	if err := Render(&out, result); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"A boundary was crossed.", "3</strong> checks declared", "1</strong> not verified", "The contract changed",
		"client-boundary", "api-compatibility", "dependency-policy", "Browser code must not reach the database.",
		"src/client/account.ts:4", "src/server/db.ts", "Use the account API.", "Existing", "Resolved", "Advisory",
		"The adapter did not return a valid result.", "contract-hash", "input-digest", "a123456789abcdef",
		"Declared inputs and scope", "api/openapi.yml", "Each result is scoped to its declared inputs.",
		"Captured 14 Oct 2026 · 12:00 UTC", "This report is a snapshot; rerun after changes.",
		"Needs attention <span>2</span>",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("HTML lost report content %q", want)
		}
	}
	// With scripts disabled, every check remains readable; progressive enhancement
	// selects a panel after load rather than hiding evidence in the HTML response.
	if strings.Contains(out.String(), `class="clause-panel" hidden`) {
		t.Error("HTML hides check evidence before progressive enhancement")
	}
}

func TestCheckTranscriptKeepsScopedEvidenceWithoutRepeatingGlobalDetails(t *testing.T) {
	result := sampleReport()
	result.Diagnostics = []string{"Global diagnostic retained in the full report."}
	result.RuleChangeNote = "Global rule-change note retained in the full report."
	result.RuleChanges = []model.RuleChange{
		{ID: "client-boundary", Change: "modified", Before: "old client rule", After: "new client rule"},
		{ID: "api-compatibility", Change: "modified", Before: "old API rule", After: "new API rule"},
	}
	result.Checks[0].Specification = "The evaluated client rule."
	result.Checks[0].Diagnostics = []string{"Check diagnostic retained in the excerpt."}
	result.Checks[0].Findings[0].Fingerprint = "finding-reference"
	var rendered, full bytes.Buffer
	if err := Render(&rendered, result); err != nil {
		t.Fatal(err)
	}
	if err := Text(&full, result, true); err != nil {
		t.Fatal(err)
	}
	transcript := func(id string) string {
		t.Helper()
		pattern := `(?s)<pre[^>]* id="` + regexp.QuoteMeta(id) + `"[^>]*>(.*?)</pre>`
		match := regexp.MustCompile(pattern).FindStringSubmatch(rendered.String())
		if len(match) != 2 {
			t.Fatalf("missing transcript %s", id)
		}
		return html.UnescapeString(match[1])
	}
	if transcript("diagnostic-text") != full.String() {
		t.Fatal("full report transcript lost evidence")
	}
	// Selecting a passing check must retain the other check's failure and the
	// policy review, without copying those checks' detailed evidence.
	passing := transcript("diagnostic-check-2")
	for _, want := range []string{"required checks: 1 passed, 1 crossed, 0 unverified", "advisory checks: 0 passed, 0 crossed, 1 unverified", "policy files changed: 1", "Repair unintended violations", "Review policy changes separately"} {
		if !strings.Contains(passing, want) {
			t.Errorf("passing-check excerpt hid overall context %q", want)
		}
	}
	if strings.Contains(passing, "[Crossed] client-boundary") {
		t.Fatal("passing-check excerpt copied another check's detailed evidence")
	}
	excerpt := transcript("diagnostic-check-0")
	for _, want := range []string{
		"The header status is the overall report outcome.",
		"Contract hash: " + result.ContractHash,
		"Input digest: " + result.InputDigest,
		result.Checks[0].Specification,
		result.Checks[0].Diagnostics[0],
		result.Checks[0].Findings[0].Message,
		"Reference: finding-reference",
		"Rule change [modified]: client-boundary",
		"old client rule", "new client rule",
	} {
		if !strings.Contains(excerpt, want) {
			t.Errorf("check excerpt lost %q", want)
		}
	}
	for _, unwanted := range []string{
		result.Diagnostics[0], result.RuleChangeNote,
		"Contract change [", result.Changes[0].Detail,
		"Rule change [modified]: api-compatibility",
		"old API rule", "new API rule",
	} {
		if strings.Contains(excerpt, unwanted) {
			t.Errorf("check excerpt repeats unrelated detail %q", unwanted)
		}
	}
}

func TestHTMLNeverInterpretsRepositoryMarkup(t *testing.T) {
	payload := `</script><script>alert("untrusted")</script><img src=x onerror=alert(1)>`
	result := sampleReport()
	result.Repository = payload
	result.Base = payload
	result.Candidate = payload
	result.ContractPath = payload
	result.ContractHash = payload
	result.InputDigest = payload
	result.Diagnostics = []string{payload}
	result.Changes = []model.ContractChange{{Path: payload, Change: payload, Detail: payload}}
	result.RuleChanges = []model.RuleChange{{ID: payload, Change: "modified", Before: payload, After: payload, PolicyFiles: []string{payload}}}
	result.Checks[0].Specification = payload
	result.Checks[0].ID = payload
	result.Checks[0].Reason = payload
	result.Checks[0].Kind = payload
	result.Checks[0].Scope = []string{payload}
	result.Checks[0].Findings = []model.Finding{{Path: payload, Target: payload, Message: payload, Remediation: payload, Evolution: payload}}
	var out bytes.Buffer
	if err := Render(&out, result); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), payload) || strings.Contains(out.String(), "<img src=x") {
		t.Fatal("untrusted repository data became executable markup")
	}
	if strings.Count(out.String(), "<script>") != 1 || strings.Count(out.String(), "</script>") != 1 {
		t.Fatal("untrusted data changed the script boundary")
	}
	if !strings.Contains(out.String(), "&lt;script&gt;alert") {
		t.Error("hostile input was not preserved as escaped evidence")
	}
}

func TestUnknownStatusNeverBecomesPassed(t *testing.T) {
	result := sampleReport()
	result.Status = "future-status"
	result.Checks = []model.Check{{ID: "future", Status: "future-status"}}
	var out bytes.Buffer
	if err := Render(&out, result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Verification is incomplete.") || !strings.Contains(out.String(), "1</strong> not verified") {
		t.Fatal("unknown status was not presented as unverified")
	}
}

func TestHTMLRemovesControlAndBidirectionalCharacters(t *testing.T) {
	result := sampleReport()
	result.Checks[0].Findings[0].Path = "src/\u202eexe\u202c.txt\x1b\r\u009d"
	result.Checks[0].Reason = "Keep\nreadable\ttext\x07."
	var out bytes.Buffer
	if err := Render(&out, result); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out.String(), "\u202e\u202c\x1b\r\u009d\x07") {
		t.Fatal("HTML contains control or bidirectional-formatting characters")
	}
	if !strings.Contains(out.String(), "src/exe.txt:4") || !strings.Contains(out.String(), "Keep\nreadable\ttext.") {
		t.Fatal("control filtering changed ordinary evidence or its spacing")
	}
}

func TestCheckNavigationUsesUniqueGeneratedIDs(t *testing.T) {
	result := sampleReport()
	for i := range result.Checks {
		result.Checks[i].ID = `duplicate id " #<unsafe>`
	}
	var out bytes.Buffer
	if err := Render(&out, result); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, match := range regexp.MustCompile(`\bid="([^"]+)"`).FindAllStringSubmatch(out.String(), -1) {
		if seen[match[1]] {
			t.Errorf("duplicate HTML ID %q", match[1])
		}
		seen[match[1]] = true
	}
	for _, match := range regexp.MustCompile(`href="#([^"]+)"`).FindAllStringSubmatch(out.String(), -1) {
		if !seen[match[1]] {
			t.Errorf("navigation target %q does not exist", match[1])
		}
	}
}

func TestEmptyReportDoesNotSuggestCoverage(t *testing.T) {
	var out bytes.Buffer
	if err := Render(&out, model.Report{Status: "passed"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No checks were declared.") || strings.Contains(out.String(), "Required checks passed.") {
		t.Fatal("empty result suggests checks passed")
	}
}

func TestFatalDiagnosticDoesNotClaimNoRulesExist(t *testing.T) {
	var out bytes.Buffer
	result := model.Report{Status: "unverified", Diagnostics: []string{"The base revision is unavailable."}}
	if err := Render(&out, result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No checks could be completed.") || strings.Contains(out.String(), "No checks were declared.") || strings.Contains(out.String(), "0</strong> checks declared") {
		t.Fatal("failed verification was confused with an empty contract")
	}
	if !strings.Contains(out.String(), "The base revision is unavailable.") {
		t.Fatal("fatal diagnostic was lost")
	}
}

func TestTextKeepsSemanticsAndStripsTerminalControls(t *testing.T) {
	result := sampleReport()
	result.Diagnostics = []string{"unsafe\x1b[2J text\x00\r\x1b]8;;https://example.invalid\x07label\x1b]8;;\x07\u009d\u202e"}
	var out bytes.Buffer
	if err := Text(&out, result, true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[Crossed] client-boundary", "[Not verified] api-compatibility (advisory; Structured result)", "[Existing]", "[Resolved]", "Next: Use the account API.", "Contract hash: contract-hash", "Input digest: input-digest"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("text lost report content %q", want)
		}
	}
	if strings.ContainsAny(out.String(), "\x1b\x00\r\x07\u009d\u202e") {
		t.Error("text includes terminal control characters")
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRendererPropagatesWriteErrors(t *testing.T) {
	if err := Render(failedWriter{}, sampleReport()); err == nil {
		t.Error("HTML swallowed writer failure")
	}
	if err := Text(failedWriter{}, sampleReport(), false); err == nil {
		t.Error("text swallowed writer failure")
	}
}

func TestPreviewStaysDistinctInHTMLTextAndCopiedEvidence(t *testing.T) {
	for _, status := range []string{"passed", "crossed", "unverified"} {
		t.Run(status, func(t *testing.T) {
			result := sampleReport()
			result.Purpose, result.Status = "preview", status
			var html, text bytes.Buffer
			if err := Render(&html, result); err != nil {
				t.Fatal(err)
			}
			if err := Text(&text, result, true); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"Preview / not approved", "candidate contract and policy files", "PREVIEW / NOT APPROVED", "Contract hash: contract-hash"} {
				if !strings.Contains(html.String(), want) {
					t.Errorf("preview HTML or copied evidence lost %q", want)
				}
			}
			if !strings.Contains(text.String(), "PREVIEW / NOT APPROVED") || !strings.Contains(text.String(), "does not approve the policy change") {
				t.Fatal("text preview could be confused with approval")
			}
			if strings.Contains(html.String(), "Required checks passed.") {
				t.Fatal("preview uses the gate title")
			}
		})
	}
}

func TestCheckOutcomeExplainsRequiredAndAdvisoryResults(t *testing.T) {
	for _, test := range []struct {
		name     string
		check    model.Check
		contains string
	}{
		{"required violation", model.Check{Required: true, Status: "crossed"}, "This required check blocks a passing result"},
		{"required incomplete", model.Check{Required: true, Status: "unverified"}, "could not be completed and blocks a passing result"},
		{"unknown required status", model.Check{Required: true, Status: "future"}, "could not be completed and blocks a passing result"},
		{"advisory violation", model.Check{Status: "crossed"}, "This advisory check does not block a passing result"},
		{"advisory incomplete", model.Check{Status: "unverified"}, "This advisory check does not block a passing result"},
		{"passing with evidence", model.Check{Status: "passed", Findings: []model.Finding{{Evolution: "existing"}}}, "existing or resolved findings can remain in a passing result"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Render(&out, model.Report{Checks: []model.Check{test.check}}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), test.contains) {
				t.Fatalf("HTML does not explain the result: want %q", test.contains)
			}
		})
	}
}

func TestContractReviewTitleDoesNotInventOrHideViolations(t *testing.T) {
	for _, test := range []struct {
		name, purpose, status, checkStatus, title string
	}{
		{"policy only", "", "crossed", "passed", "Contract review required."},
		{"policy and incomplete check", "", "crossed", "unverified", "Contract changed; verification is incomplete."},
		{"policy and violation", "", "crossed", "crossed", "A boundary was crossed."},
		{"passing proposal", "preview", "passed", "passed", "Proposed checks passed."},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := model.Report{
				Purpose: test.purpose, Status: test.status,
				Changes: []model.ContractChange{{Path: ".linebind.yml", Change: "modified"}},
				Checks:  []model.Check{{ID: "boundary", Status: test.checkStatus, Required: true}},
			}
			var out bytes.Buffer
			if err := Render(&out, result); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), `<h1 id="report-title">`+test.title+`</h1>`) {
				t.Fatalf("HTML does not distinguish policy review from verification: want %q", test.title)
			}
		})
	}
}

func TestReportEmbedsFontAndLicenseOffline(t *testing.T) {
	var out bytes.Buffer
	if err := Render(&out, sampleReport()); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	match := regexp.MustCompile(`data:font/woff2;base64,([A-Za-z0-9+/=]+)`).FindStringSubmatch(html)
	if len(match) != 2 {
		t.Fatal("embedded font URL missing or escaped incorrectly")
	}
	font, err := base64.StdEncoding.DecodeString(match[1])
	if err != nil || !bytes.Equal(font, displayFont) {
		t.Fatal("embedded font differs from bundled asset")
	}
	for _, value := range []string{"font-src data:", "SIL OPEN FONT LICENSE", "font-license"} {
		if !strings.Contains(html, value) {
			t.Errorf("missing %s", value)
		}
	}
}
