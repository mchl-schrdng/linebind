// Package report renders the same result for people, agents, and an offline browser.
package report

import (
	"bytes"
	_ "embed"
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

var page = template.Must(template.New("report").Funcs(template.FuncMap{
	"status":    statusLabel,
	"state":     state,
	"evolution": evolutionLabel,
	"evidence":  evidenceLabel,
	"kind":      func(s string) string { return strings.ReplaceAll(s, "_", " ") },
	"short":     shortRef,
	"time":      displayTime,
}).Parse(reportHTML))

type pageData struct {
	model.Report
	Name       string
	Title      string
	Crossed    int
	Unverified int
	Transcript string
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
	switch state(result.Status) {
	case "passed":
		data.Title = "Required checks passed."
	case "crossed":
		data.Title = "A boundary was crossed."
	default:
		data.Title = "Verification is incomplete."
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
	if err := Text(&transcript, result, true); err != nil {
		return err
	}
	data.Transcript = transcript.String()
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
	var out bytes.Buffer
	if result.Purpose == "preview" {
		fmt.Fprintf(&out, "Linebind %s — PREVIEW / NOT APPROVED — %s\n", result.Version, statusLabel(result.Status))
		fmt.Fprintln(&out, "Uses the candidate contract and policy files. The exit code describes proposed checks only; this is not a CI gate or policy approval.")
	} else {
		fmt.Fprintf(&out, "Linebind %s — %s\n", result.Version, statusLabel(result.Status))
	}
	fmt.Fprintf(&out, "Repository: %s\nComparison: %s → %s\n", result.Repository, result.Base, result.Candidate)
	if agent {
		fmt.Fprintf(&out, "Contract: %s\nContract hash: %s\nInput digest: %s\n", result.ContractPath, result.ContractHash, result.InputDigest)
	}
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintf(&out, "Diagnostic: %s\n", diagnostic)
	}
	for _, change := range result.Changes {
		fmt.Fprintf(&out, "Contract change [%s]: %s — %s\n", change.Change, change.Path, change.Detail)
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
