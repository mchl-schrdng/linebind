package contract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mchl-schrdng/linebind/internal/model"
)

const maxFindings = 1000

// Evaluate runs native checks only. Command checks remain unverified until the
// caller executes them in an explicitly selected execution environment.
func Evaluate(c *Contract, files map[string][]byte) []model.Check {
	if c == nil {
		return []model.Check{{ID: "contract", Status: "unverified", Required: true, Evidence: "native", Diagnostics: []string{"no contract loaded"}}}
	}
	checks := make([]model.Check, 0, len(c.Rules))
	for _, r := range c.Rules {
		checks = append(checks, evaluateRule(r, files, false))
	}
	return checks
}

// EvaluateBaseline permits a known empty native scope before the first file is
// added. The caller must still mark omitted snapshot inputs as unverified.
func EvaluateBaseline(r Rule, files map[string][]byte) model.Check {
	return evaluateRule(r, files, r.Kind == "forbid-text" || r.Kind == "dependencies" || r.Kind == "document")
}

func newCheck(r Rule) model.Check {
	return model.Check{ID: r.ID, Kind: r.Kind, Reason: r.Reason, Required: r.Required, Status: "passed", Evidence: "native", Scope: slices.Clone(r.Scope), Specification: r.Specification, Findings: []model.Finding{}, Diagnostics: []string{}}
}

func evaluateRule(r Rule, files map[string][]byte, allowEmpty bool) model.Check {
	check := newCheck(r)
	if r.Kind == "co-change" {
		check.Status = "unverified"
		check.Diagnostics = append(check.Diagnostics, "co-change requires both base and candidate snapshots")
		return check
	}
	scope := r.Scope
	if r.Kind == "require-files" && len(scope) == 0 {
		scope = r.Files.Require
		check.Scope = slices.Clone(scope)
	}
	selected, err := Select(scope, files)
	if err != nil {
		check.Diagnostics = append(check.Diagnostics, err.Error())
	} else {
		check.Files = len(selected)
		if len(selected) == 0 && r.Kind != "require-files" {
			if !allowEmpty {
				check.Diagnostics = append(check.Diagnostics, "declared scope matched no files")
			}
		} else {
			switch r.Kind {
			case "forbid-text":
				evaluateText(r, selected, files, &check)
			case "require-files":
				evaluateFiles(r, selected, &check)
			case "dependencies":
				evaluateDependencies(r, selected, files, &check)
			case "document":
				evaluateDocuments(r, selected, files, &check)
			case "command":
				check.Evidence = "command"
				if r.Command.Format != "exit" {
					check.Evidence = "structured"
				}
				check.Diagnostics = append(check.Diagnostics, "command requires an explicitly selected execution environment")
			default:
				check.Diagnostics = append(check.Diagnostics, "unsupported rule kind")
			}
		}
	}
	if len(check.Diagnostics) > 0 {
		check.Status = "unverified"
	} else if len(check.Findings) > 0 {
		check.Status = "crossed"
	}
	return check
}

func evaluateText(r Rule, selected []string, files map[string][]byte, check *model.Check) {
	rx := regexp.MustCompile(r.Text.Pattern)
	for _, name := range selected {
		data := files[name]
		if !utf8.Valid(data) {
			check.Diagnostics = append(check.Diagnostics, name+": expected UTF-8 text")
			continue
		}
		occurrences := map[string]int{}
		line, start, scanned, end := 1, 0, 0, -1
		for _, at := range rx.FindAllIndex(data, maxFindings+1) {
			if len(check.Findings) == maxFindings {
				check.Diagnostics = append(check.Diagnostics, "finding limit reached; remaining text matches were not reported")
				return
			}
			prefix := data[scanned:at[0]]
			line += bytes.Count(prefix, []byte{'\n'})
			if last := bytes.LastIndexByte(prefix, '\n'); last >= 0 {
				start = scanned + last + 1
			}
			scanned = at[0]
			if end < at[1] {
				end = bytes.IndexByte(data[at[1]:], '\n')
				if end < 0 {
					end = len(data)
				} else {
					end += at[1]
				}
			}
			context := strings.TrimSpace(string(data[start:end]))
			occurrences[context]++
			f := finding(r, name, "text-match", "Forbidden lexical pattern matched; comments and strings are included.", context, strconv.Itoa(occurrences[context]))
			f.Line = line
			check.Findings = append(check.Findings, f)
		}
	}
}

func evaluateFiles(r Rule, selected []string, check *model.Check) {
	for _, expected := range r.Files.Require {
		found := false
		for _, name := range selected {
			matched, _ := Match(expected, name)
			if matched {
				found = true
				break
			}
		}
		if !found {
			if len(check.Findings) == maxFindings {
				check.Diagnostics = append(check.Diagnostics, "finding limit reached; remaining required-file findings were not reported")
				return
			}
			check.Findings = append(check.Findings, finding(r, expected, expected, "Required file pattern has no matching file."))
		}
	}
}

func finding(r Rule, name, target, message string, identity ...string) model.Finding {
	parts := append([]string{SemanticDigest(r), name, target}, identity...)
	data, _ := json.Marshal(parts)
	hash := sha256.Sum256(data)
	return model.Finding{Fingerprint: hex.EncodeToString(hash[:]), Path: name, Target: target, Message: message, Remediation: r.Remediation, Evolution: "not_comparable"}
}

// SemanticDigest excludes explanation/presentation and comparison mode. A
// changed predicate cannot reuse findings authorized under the old predicate.
func SemanticDigest(r Rule) string {
	r.Reason, r.Remediation, r.Compare = "", "", ""
	r.Required = true
	r.Scope = slices.Clone(r.Scope)
	slices.Sort(r.Scope)
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf("invalid:%s", r.ID)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
