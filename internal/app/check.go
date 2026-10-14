package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mchl-schrdng/linebind/internal/contract"
	"github.com/mchl-schrdng/linebind/internal/model"
	"github.com/mchl-schrdng/linebind/internal/repository"
)

type Options struct {
	Repo, Base, Head, Contract, Version string
	AllowExec                           bool
}

func Check(opt Options) model.Report {
	r := model.Report{SchemaVersion: 1, Version: opt.Version, Repository: filepath.Base(opt.Repo), Base: opt.Base, Candidate: opt.Head,
		ContractPath: opt.Contract, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Status: "unverified", ExitCode: 2,
		Changes: []model.ContractChange{}, Checks: []model.Check{}, Diagnostics: []string{}}
	fail := func(err error) model.Report {
		r.Diagnostics = append(r.Diagnostics, err.Error())
		if len(r.Changes) > 0 {
			r.Status = "crossed"
			r.ExitCode = 1
		}
		return r
	}
	if !repository.SafePath(opt.Contract) {
		return fail(fmt.Errorf("contract must be a safe repository-relative path"))
	}
	repo, err := repository.Open(opt.Repo)
	if err != nil {
		return fail(err)
	}
	r.Repository = filepath.Base(repo.Root)
	base, err := repo.Commit(opt.Base)
	if err != nil {
		return fail(err)
	}
	r.Base = base.Identity
	var candidate *repository.Snapshot
	if opt.Head == "" || opt.Head == "worktree" {
		candidate, err = repo.WorkingTree()
	} else {
		candidate, err = repo.Commit(opt.Head)
	}
	if err != nil {
		return fail(err)
	}
	r.Candidate = candidate.Identity
	r.InputDigest = candidate.Digest
	raw, ok := base.Files[opt.Contract]
	if !ok {
		return fail(fmt.Errorf("approved base has no readable %s; commit and approve a contract before checking changes", opt.Contract))
	}
	r.ContractHash = digest(raw)
	c, err := contract.Load(raw)
	if err != nil {
		return fail(fmt.Errorf("approved contract invalid: %w", err))
	}
	policy := map[string]bool{opt.Contract: true}
	for _, rule := range c.Rules {
		if rule.Command != nil {
			for _, name := range rule.Command.PolicyFiles {
				policy[name] = true
			}
		}
	}
	names := make([]string, 0, len(policy))
	for name := range policy {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		approved, exists := base.Files[name]
		if !exists {
			return fail(fmt.Errorf("approved policy file %s is unavailable", name))
		}
		proposed, exists := candidate.Files[name]
		if !exists {
			r.Changes = append(r.Changes, model.ContractChange{Path: name, Change: "removed", Detail: "Approved policy file is absent from the candidate; the approved version remains effective."})
		} else if !policyEqual(approved, proposed) || base.Executable[name] != candidate.Executable[name] {
			r.Changes = append(r.Changes, model.ContractChange{Path: name, Change: "modified", Detail: "Candidate policy differs from the approved revision; the approved version remains effective."})
		}
	}
	r.Checks = contract.Evaluate(c, candidate.Files)
	previous := contract.Evaluate(c, base.Files)
	for i, rule := range c.Rules {
		if rule.Kind == "command" {
			r.Checks[i] = runCommand(rule, candidate, base, opt.AllowExec)
			if rule.Compare == "new" {
				previous[i] = runCommand(rule, base, base, opt.AllowExec)
			}
		}
		markOmissions(rule, candidate, &r.Checks[i])
		markOmissions(rule, base, &previous[i])
		if rule.Compare == "new" {
			compare(&r.Checks[i], previous[i])
		} else {
			for j := range r.Checks[i].Findings {
				r.Checks[i].Findings[j].Evolution = "not_comparable"
			}
		}
	}
	finalize(&r)
	return r
}

func policyEqual(a, b []byte) bool {
	if bytes.Equal(a, b) {
		return true
	}
	// Git may check text out with CRLF. Preserve literal comparison for every
	// other byte change, including comments and whitespace within lines.
	if !utf8.Valid(a) || !utf8.Valid(b) || bytes.ContainsRune(a, 0) || bytes.ContainsRune(b, 0) {
		return false
	}
	return bytes.Equal(bytes.ReplaceAll(a, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")))
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func markOmissions(rule contract.Rule, s *repository.Snapshot, c *model.Check) {
	patterns := rule.Scope
	if rule.Files != nil {
		patterns = rule.Files.Require
	}
	names := make([]string, 0, len(s.Omitted))
	for name := range s.Omitted {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, pattern := range patterns {
			matches, err := contract.Match(pattern, name)
			// A symlink or submodule can hide a directory. Matching only its own
			// name would silently miss files beneath a declared glob.
			if strings.Contains(s.Omitted[name], "symlink") {
				prefix := pattern
				if i := strings.IndexAny(prefix, "*?[{"); i >= 0 {
					prefix = prefix[:i]
					if slash := strings.LastIndex(prefix, "/"); slash >= 0 {
						prefix = prefix[:slash]
					} else {
						prefix = ""
					}
				}
				matches = matches || prefix == "" || name == prefix || strings.HasPrefix(name, prefix+"/") || strings.HasPrefix(prefix, name+"/")
			}
			if err != nil || matches {
				c.Status = "unverified"
				c.Diagnostics = append(c.Diagnostics, fmt.Sprintf("%s: %s", name, s.Omitted[name]))
				break
			}
		}
	}
}

func compare(current *model.Check, base model.Check) {
	if base.Status == "unverified" || current.Status == "unverified" {
		current.Status = "unverified"
		current.Diagnostics = append(current.Diagnostics, "Cannot compare findings: base or candidate verification is incomplete.")
		for i := range current.Findings {
			current.Findings[i].Evolution = "not_comparable"
		}
		return
	}
	remaining := map[string][]model.Finding{}
	for _, f := range base.Findings {
		remaining[f.Fingerprint] = append(remaining[f.Fingerprint], f)
	}
	current.Status = "passed"
	for i := range current.Findings {
		f := &current.Findings[i]
		if f.Fingerprint != "" && len(remaining[f.Fingerprint]) > 0 {
			f.Evolution = "existing"
			remaining[f.Fingerprint] = remaining[f.Fingerprint][1:]
		} else {
			f.Evolution = "new"
			current.Status = "crossed"
		}
	}
	keys := make([]string, 0, len(remaining))
	for key := range remaining {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, f := range remaining[key] {
			f.Evolution = "resolved"
			current.Findings = append(current.Findings, f)
		}
	}
}

func finalize(r *model.Report) {
	r.Status, r.ExitCode = Outcome(*r)
	if len(r.Changes) > 0 {
		r.Diagnostics = append(r.Diagnostics, "Contract changes require review. This run uses the approved base; candidate policy cannot authorize itself.")
	}
}

// Outcome derives the aggregate gate from completed check records. Fatal runs
// without check records retain their explicit operational-error result instead.
func Outcome(r model.Report) (status string, exit int) {
	status = "passed"
	if len(r.Changes) > 0 {
		status = "crossed"
		exit = 1
	}
	for _, c := range r.Checks {
		if c.Status == "unverified" {
			if status == "passed" {
				status = "unverified"
			}
			if c.Required && exit == 0 {
				exit = 2
			}
		}
		if c.Status == "crossed" {
			status = "crossed"
			if c.Required {
				exit = 1
			}
		}
	}
	return
}
