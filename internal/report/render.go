// Package report renders the same result for people, agents, and an offline browser.
package report

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/mchl-schrdng/linebind/internal/model"
)

//go:embed report.html
var reportHTML string

//go:embed fonts/syne-latin-variable.woff2
var displayFont []byte

//go:embed fonts/OFL.txt
var fontLicense string

// The URL contains only our bundled font, never repository-controlled data.
var displayFontURL = template.URL("data:font/woff2;base64," + base64.StdEncoding.EncodeToString(displayFont))

var page = template.Must(template.New("report").Funcs(template.FuncMap{
	"font":        func() template.URL { return displayFontURL },
	"fontLicense": func() string { return fontLicense },
	"status":      statusLabel,
	"state":       state,
	"evolution":   evolutionLabel,
	"evidence":    evidenceLabel,
	"outcome":     checkOutcome,
	"kind":        func(s string) string { return strings.ReplaceAll(s, "_", " ") },
	"short":       shortRef,
	"time":        displayTime,
	"policy":      policySource,
	"ordinal":     func(i int) string { return fmt.Sprintf("%02d", i+1) },
}).Parse(reportHTML))

type pageData struct {
	model.Report
	Name             string
	Title            string
	Crossed          int
	Unverified       int
	Attention        int
	Transcript       string
	CheckTranscripts []string
}

// Render writes a self-contained, read-only HTML report. Repository data is
// always escaped by html/template and is never interpolated into JavaScript.
func Render(w io.Writer, result model.Report) error {
	data := pageData{Report: result, Name: filepath.Base(result.Repository)}
	if result.Repository == "" {
		data.Name = "Project report"
	}
	for _, check := range result.Checks {
		switch state(check.Status) {
		case "crossed":
			data.Crossed++
		case "unverified":
			data.Unverified++
		}
	}
	data.Attention = data.Crossed + data.Unverified
	switch state(result.Status) {
	case "passed":
		data.Title = "Required checks passed."
	case "crossed":
		data.Title = "A boundary was crossed."
	default:
		data.Title = "Verification is incomplete."
	}
	if result.Purpose != "preview" && state(result.Status) == "crossed" && len(result.Changes) > 0 && data.Crossed == 0 {
		data.Title = "Contract review required."
		if data.Unverified > 0 {
			data.Title = "Contract changed; verification is incomplete."
		}
	}
	if result.Purpose == "preview" {
		switch state(result.Status) {
		case "passed":
			data.Title = "Proposed checks passed."
		case "crossed":
			data.Title = "A proposed boundary was crossed."
		default:
			data.Title = "Proposed checks are incomplete."
		}
	}
	if len(result.Checks) == 0 {
		data.Title = "No checks were declared."
		if len(result.Diagnostics) > 0 {
			data.Title = "No checks could be completed."
		}
	}
	var transcript bytes.Buffer
	summary := summaryText(result)
	if err := writeText(&transcript, result, true, summary); err != nil {
		return err
	}
	data.Transcript = transcript.String()
	for _, check := range result.Checks {
		excerpt := result
		excerpt.Checks = []model.Check{check}
		excerpt.Diagnostics = nil
		excerpt.Changes = nil
		excerpt.RuleChangeNote = ""
		excerpt.RuleChanges = nil
		for _, change := range result.RuleChanges {
			if change.ID == check.ID {
				excerpt.RuleChanges = append(excerpt.RuleChanges, change)
			}
		}
		var text bytes.Buffer
		text.WriteString("Single-check excerpt. The header status is the overall report outcome. Summary and next action also describe the full report; details below cover only this check.\n")
		if err := writeText(&text, excerpt, true, summary); err != nil {
			return err
		}
		data.CheckTranscripts = append(data.CheckTranscripts, text.String())
	}
	var out bytes.Buffer
	if err := page.Execute(&out, data); err != nil {
		return err
	}
	_, err := io.WriteString(w, removeControls(out.String()))
	return err
}

// Text writes a compact, plain-text report. Agent mode adds the exact input and
// contract identifiers so a copied diagnostic remains tied to its analysis.
func Text(w io.Writer, result model.Report, agent bool) error {
	return writeText(w, result, agent, summaryText(result))
}

// Excerpts keep the full report's summary so a selected passing check cannot
// hide another check's violation, incomplete evidence, or policy review.
func writeText(w io.Writer, result model.Report, agent bool, summary string) error {
	var out bytes.Buffer
	if result.Purpose == "preview" {
		fmt.Fprintf(&out, "Linebind %s — PREVIEW / NOT APPROVED — %s\n", result.Version, statusLabel(result.Status))
		fmt.Fprintln(&out, "Uses the candidate contract and policy files. The exit code describes proposed checks only; it does not approve the policy change or establish compliance with the base contract.")
	} else {
		fmt.Fprintf(&out, "Linebind %s — %s\n", result.Version, statusLabel(result.Status))
	}
	fmt.Fprintf(&out, "Repository: %s\nComparison: %s → %s\n", result.Repository, result.Base, result.Candidate)
	out.WriteString(summary)
	if agent {
		fmt.Fprintf(&out, "Contract: %s\nContract hash: %s\nInput digest: %s\n", result.ContractPath, result.ContractHash, result.InputDigest)
		fmt.Fprintf(&out, "Captured: %s\nPolicy source: %s\n", result.GeneratedAt, policySource(result.Purpose))
	}
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintf(&out, "Diagnostic: %s\n", diagnostic)
	}
	for _, change := range result.Changes {
		fmt.Fprintf(&out, "Contract change [%s]: %s — %s\n", change.Change, change.Path, change.Detail)
	}
	if result.RuleChangeNote != "" {
		fmt.Fprintln(&out, result.RuleChangeNote)
	}
	for _, change := range result.RuleChanges {
		fmt.Fprintf(&out, "Rule change [%s]: %s\n", change.Change, change.ID)
		if agent {
			if change.Before != "" {
				fmt.Fprintf(&out, "Before (base):\n%s", change.Before)
			}
			if change.After != "" {
				fmt.Fprintf(&out, "After (candidate):\n%s", change.After)
			}
		}
		for _, path := range change.PolicyFiles {
			fmt.Fprintf(&out, "Changed policy input: %s\n", path)
		}
	}
	for _, check := range result.Checks {
		mode := "advisory"
		if check.Required {
			mode = "required"
		}
		fmt.Fprintf(&out, "\n[%s] %s (%s; %s)\n", statusLabel(check.Status), check.ID, mode, evidenceLabel(check.Evidence))
		if check.Reason != "" {
			fmt.Fprintf(&out, "Reason: %s\n", check.Reason)
		}
		if len(check.Scope) > 0 {
			fmt.Fprintf(&out, "Declared inputs: %s\n", strings.Join(check.Scope, ", "))
		}
		if agent && check.Specification != "" {
			fmt.Fprintf(&out, "Evaluated rule (defaults included):\n%s", check.Specification)
		}
		for _, diagnostic := range check.Diagnostics {
			fmt.Fprintf(&out, "Diagnostic: %s\n", diagnostic)
		}
		for _, finding := range check.Findings {
			fmt.Fprintf(&out, "- [%s] %s", evolutionLabel(finding.Evolution), finding.Path)
			if finding.Line > 0 {
				fmt.Fprintf(&out, ":%d", finding.Line)
			}
			if finding.Target != "" {
				fmt.Fprintf(&out, " → %s", finding.Target)
			}
			fmt.Fprintf(&out, "\n  %s\n", finding.Message)
			if finding.Remediation != "" {
				fmt.Fprintf(&out, "  Next: %s\n", finding.Remediation)
			}
			if agent && finding.Fingerprint != "" {
				fmt.Fprintf(&out, "  Reference: %s\n", finding.Fingerprint)
			}
		}
	}
	fmt.Fprintln(&out, "\nResults apply only to the declared checks and recorded inputs.")
	_, err := io.WriteString(w, removeControls(out.String()))
	return err
}

func summaryText(result model.Report) string {
	type counts struct{ passed, crossed, unverified int }
	var required, advisory counts
	for _, check := range result.Checks {
		group := &required
		if !check.Required {
			group = &advisory
		}
		switch state(check.Status) {
		case "passed":
			group.passed++
		case "crossed":
			group.crossed++
		default:
			group.unverified++
		}
	}
	parts := []string{fmt.Sprintf("required checks: %d passed, %d crossed, %d unverified", required.passed, required.crossed, required.unverified)}
	if len(result.Checks) == 0 {
		parts = []string{"no checks completed"}
	}
	if advisory.passed+advisory.crossed+advisory.unverified > 0 {
		parts = append(parts, fmt.Sprintf("advisory checks: %d passed, %d crossed, %d unverified", advisory.passed, advisory.crossed, advisory.unverified))
	}
	if len(result.Changes) > 0 {
		parts = append(parts, fmt.Sprintf("policy files changed: %d", len(result.Changes)))
	}
	var actions []string
	if required.crossed > 0 {
		actions = append(actions, "Repair unintended violations using the findings and remediation.")
	}
	if required.unverified > 0 || len(result.Checks) == 0 {
		actions = append(actions, "Resolve incomplete verification using the diagnostics.")
	}
	if advisory.crossed+advisory.unverified > 0 {
		actions = append(actions, "Review advisory findings and diagnostics; they do not block the exit code.")
	}
	if result.Purpose == "preview" {
		actions = append(actions, "Review the proposal; preview does not approve adoption.")
	} else if len(result.Changes) > 0 {
		actions = append(actions, "Review policy changes separately; use preview with the same base to evaluate the proposal, not approve it.")
	}
	if len(actions) == 0 {
		actions = append(actions, "Re-run after relevant edits, keeping the comparison base fixed.")
	}
	return "Summary: " + strings.Join(parts, "; ") + ".\nNext action: " + strings.Join(actions, " ") + "\n"
}

func policySource(purpose string) string {
	if purpose == "preview" {
		return "candidate (preview / not approved)"
	}
	return "base"
}

// Control and bidirectional-formatting characters must not alter a diagnostic's
// terminal behavior or the visible order of a path in either rendered format.
func removeControls(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Bidi_Control, r) || (unicode.IsControl(r) && r != '\n' && r != '\t') {
			return -1
		}
		return r
	}, s)
}

func state(s string) string {
	if s == "passed" || s == "crossed" {
		return s
	}
	return "unverified"
}

func statusLabel(s string) string {
	switch state(s) {
	case "passed":
		return "Passed"
	case "crossed":
		return "Crossed"
	default:
		return "Not verified"
	}
}

func checkOutcome(check model.Check) string {
	if state(check.Status) == "passed" {
		if len(check.Findings) > 0 {
			return "This check passed within its declared scope. Review the finding labels: existing or resolved findings can remain in a passing result."
		}
		return "This check passed within its declared scope."
	}
	if !check.Required {
		return "This advisory check does not block a passing result. Review its findings or diagnostics before proceeding."
	}
	if state(check.Status) == "crossed" {
		return "This required check blocks a passing result. Review the evidence and next steps below."
	}
	return "This required check could not be completed and blocks a passing result. Resolve the diagnostics, then run it again."
}

func evolutionLabel(s string) string {
	switch s {
	case "new":
		return "New"
	case "existing":
		return "Existing"
	case "resolved":
		return "Resolved"
	default:
		return "Not comparable"
	}
}

func evidenceLabel(s string) string {
	switch s {
	case "native":
		return "Native check"
	case "structured":
		return "Structured result"
	case "command":
		return "Command result"
	default:
		return "Unspecified evidence"
	}
}

func shortRef(s string) string {
	if len(s) < 13 {
		return s
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return s
		}
	}
	return s[:8]
}

func displayTime(s string) string {
	if parsed, err := time.Parse(time.RFC3339, s); err == nil {
		return parsed.UTC().Format("02 Jan 2006 · 15:04 UTC")
	}
	return s
}
