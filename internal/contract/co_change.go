package contract

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"github.com/mchl-schrdng/linebind/internal/model"
)

// EvaluateChanges checks file contents at the two endpoints, not commit history
// or the quality of a companion edit. Renames are a deletion plus an addition.
func EvaluateChanges(r Rule, base, candidate map[string][]byte) model.Check {
	check := newCheck(r)
	check.Scope = append(check.Scope, r.CoChange.Require...)
	union := make(map[string][]byte, len(base)+len(candidate))
	for name, data := range base {
		union[name] = data
	}
	for name, data := range candidate {
		union[name] = data
	}
	triggers, err := Select(r.Scope, union)
	if err != nil {
		check.Status = "unverified"
		check.Diagnostics = append(check.Diagnostics, err.Error())
		return check
	}
	inputs, err := Select(check.Scope, union)
	if err != nil {
		check.Status = "unverified"
		check.Diagnostics = append(check.Diagnostics, err.Error())
		return check
	}
	check.Files = len(inputs)
	trigger := ""
	for _, name := range triggers {
		before, existed := base[name]
		after, exists := candidate[name]
		if existed != exists || !SameContent(before, after) {
			trigger = name
			break
		}
	}
	if trigger == "" {
		return check
	}
	for _, pattern := range r.CoChange.Require {
		paths, err := Select([]string{pattern}, candidate)
		if err != nil {
			check.Status = "unverified"
			check.Diagnostics = append(check.Diagnostics, err.Error())
			return check
		}
		changed := false
		for _, name := range paths {
			before, existed := base[name]
			if !existed || !SameContent(before, candidate[name]) {
				changed = true
				break
			}
		}
		if !changed {
			if len(check.Findings) == maxFindings {
				check.Status = "unverified"
				check.Diagnostics = append(check.Diagnostics, "finding limit reached; remaining co-change findings were not reported")
				return check
			}
			check.Status = "crossed"
			check.Findings = append(check.Findings, finding(r, trigger, pattern, fmt.Sprintf("A scoped file changed; add or modify at least one file matching %s.", pattern)))
		}
	}
	return check
}

// SameContent ignores Git's LF/CRLF text checkout conversion, preserving every
// other byte change. Binary and invalid UTF-8 inputs remain byte-for-byte.
func SameContent(a, b []byte) bool {
	if bytes.Equal(a, b) {
		return true
	}
	if !utf8.Valid(a) || !utf8.Valid(b) || bytes.ContainsRune(a, 0) || bytes.ContainsRune(b, 0) {
		return false
	}
	return bytes.Equal(bytes.ReplaceAll(a, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")))
}
