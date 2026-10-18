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
	return evaluate(opt, "")
}

// Preview evaluates candidate policy without approving it or changing Check's gate.
func Preview(opt Options) model.Report {
	return evaluate(opt, "preview")
}

func evaluate(opt Options, purpose string) model.Report {
	r := model.Report{SchemaVersion: 2, Purpose: purpose, Version: opt.Version, Repository: filepath.Base(opt.Repo), Base: opt.Base, Candidate: opt.Head,
		ContractPath: opt.Contract, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Status: "unverified", ExitCode: 2,
		Changes: []model.ContractChange{}, Checks: []model.Check{}, Diagnostics: []string{}}
	if purpose == "preview" {
		r.Diagnostics = append(r.Diagnostics, "Preview only: the candidate supplies the proposed contract and policy files. This analysis does not approve policy changes or replace an approved-base check.")
	}
	fail := func(err error) model.Report {
		r.Diagnostics = append(r.Diagnostics, err.Error())
		if len(r.Changes) > 0 && purpose != "preview" {
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
	// Resolve the candidate once so a moving ref cannot change the ancestry
	// comparison between base selection and snapshot capture.
	if opt.Head != "" && opt.Head != "worktree" {
		opt.Head, err = repo.Resolve(opt.Head)
		if err != nil {
			return fail(err)
		}
	}
	baseSHA, err := repo.ResolveBase(opt.Base, opt.Head)
	if err != nil {
		return fail(err)
	}
	base, err := repo.Commit(baseSHA)
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
	policySource, policyLabel := base, "approved"
	if purpose == "preview" {
		policySource, policyLabel = candidate, "proposed"
	}
	raw, ok := policySource.Files[opt.Contract]
	if !ok {
		if purpose == "preview" {
			return fail(fmt.Errorf("candidate has no readable proposed contract at %s", opt.Contract))
		}
		return fail(fmt.Errorf("approved base has no readable %s; commit and approve a contract before checking changes", opt.Contract))
	}
	r.ContractHash = digest(raw)
	c, err := contract.Load(raw)
	if err != nil {
		return fail(fmt.Errorf("%s contract invalid: %w", policyLabel, err))
	}
	r.RuleChanges, r.RuleChangeNote = describeRuleChanges(base, candidate, opt.Contract)
	policy := map[string]bool{opt.Contract: true}
	for _, rule := range c.Rules {
		if rule.Command != nil {
			for _, name := range rule.Command.PolicyFiles {
				policy[name] = true
			}
		}
	}
	if purpose == "preview" {
		for _, rule := range c.Rules {
			if rule.Command != nil {
				for _, name := range rule.Command.PolicyFiles {
					if _, exists := candidate.Files[name]; !exists {
						return fail(fmt.Errorf("proposed policy file %s is unavailable", name))
					}
				}
			}
		}
		// Include removed declarations in the review diff without requiring an old contract at bootstrap.
		if previous, err := contract.Load(base.Files[opt.Contract]); err == nil {
			for _, rule := range previous.Rules {
				if rule.Command != nil {
					for _, name := range rule.Command.PolicyFiles {
						policy[name] = true
					}
				}
			}
		}
	}
	names := make([]string, 0, len(policy))
	for name := range policy {
		names = append(names, name)
	}
	sort.Strings(names)
	var missingPolicy string
	for _, name := range names {
		approved, approvedExists := base.Files[name]
		if purpose != "preview" && !approvedExists && missingPolicy == "" {
			missingPolicy = name
		}
		proposed, proposedExists := candidate.Files[name]
		change, detail := "", ""
		switch {
		case !approvedExists && proposedExists:
			change, detail = "added", "Candidate policy file is absent from the comparison base."
		case approvedExists && !proposedExists:
			change, detail = "removed", "Policy file from the base is absent from the candidate."
		case approvedExists && proposedExists && (!policyEqual(approved, proposed) || base.Executable[name] != candidate.Executable[name]):
			change, detail = "modified", "Candidate policy differs from the base revision."
		}
		if change != "" {
			if purpose == "preview" {
				detail += " This preview uses the proposed policy; review is still required."
			} else {
				detail += " The approved version remains effective."
			}
			r.Changes = append(r.Changes, model.ContractChange{Path: name, Change: change, Detail: detail})
		}
	}
	if missingPolicy != "" {
		return fail(fmt.Errorf("approved policy file %s is unavailable", missingPolicy))
	}
	r.Checks = contract.Evaluate(c, candidate.Files)
	previous := contract.Evaluate(c, base.Files)
	for i, rule := range c.Rules {
		if rule.Kind == "command" {
			r.Checks[i] = runCommand(rule, candidate, policySource, opt.AllowExec)
			if rule.Compare == "new" {
				previous[i] = runCommand(rule, base, policySource, opt.AllowExec)
			}
		}
		r.Checks[i].Specification = rule.Specification
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
		if !omissionMatches(name, s.Omitted[name], patterns) {
			continue
		}
		if rule.Files != nil && len(rule.Scope) > 0 && !omissionMatches(name, s.Omitted[name], rule.Scope) {
			continue
		}
		c.Status = "unverified"
		c.Diagnostics = append(c.Diagnostics, fmt.Sprintf("%s: %s", name, s.Omitted[name]))
	}
}

func omissionMatches(name, detail string, patterns []string) bool {
	for _, pattern := range patterns {
		matches, err := contract.Match(pattern, name)
		// A symlink or submodule can hide a directory. Matching only its own
		// name would silently miss files beneath a declared glob.
		if strings.Contains(detail, "symlink") {
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
			return true
		}
	}
	return false
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
	if len(r.Changes) > 0 && r.Purpose != "preview" {
		r.Diagnostics = append(r.Diagnostics, "Contract changes require review. This run uses the approved base; candidate policy cannot authorize itself.")
	}
}

// Outcome aggregates completed checks. Preview policy changes are review evidence,
// not check failures; fatal runs without checks retain their operational error.
func Outcome(r model.Report) (status string, exit int) {
	status = "passed"
	if len(r.Changes) > 0 && r.Purpose != "preview" {
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
