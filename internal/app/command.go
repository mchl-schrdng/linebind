package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/mchl-schrdng/linebind/internal/contract"
	"github.com/mchl-schrdng/linebind/internal/jsonstrict"
	"github.com/mchl-schrdng/linebind/internal/model"
	"github.com/mchl-schrdng/linebind/internal/repository"
)

func runCommand(rule contract.Rule, candidate, base *repository.Snapshot, allow bool) (c model.Check) {
	c = model.Check{ID: rule.ID, Kind: rule.Kind, Reason: rule.Reason, Required: rule.Required, Status: "unverified", Evidence: "command", Scope: rule.Scope, Findings: []model.Finding{}, Diagnostics: []string{}}
	start := time.Now()
	defer func() { c.DurationMS = time.Since(start).Milliseconds() }()
	if rule.Command.Format != "exit" {
		c.Evidence = "structured"
	}
	fail := func(s string) model.Check {
		c.Status = "unverified"
		c.Diagnostics = append(c.Diagnostics, s)
		return c
	}
	selected, err := contract.Select(rule.Scope, candidate.Files)
	if err != nil {
		return fail(err.Error())
	}
	c.Files = len(selected)
	if len(selected) == 0 {
		return fail("No files match the declared command scope.")
	}
	if !allow {
		return fail("Project command was not executed. Use --allow-exec only for a trusted project or inside the documented isolated CI environment.")
	}
	if len(candidate.Omitted) > 0 {
		return fail("Command was not executed: input snapshot is incomplete. Commands require the whole captured repository; narrowing scope does not exclude omitted inputs.")
	}
	temp, err := os.MkdirTemp("", "linebind-run-")
	if err != nil {
		return fail(err.Error())
	}
	defer os.RemoveAll(temp)
	source := filepath.Join(temp, "source")
	home := filepath.Join(temp, "home")
	if err := os.Mkdir(source, 0700); err != nil {
		return fail(err.Error())
	}
	if err := os.Mkdir(home, 0700); err != nil {
		return fail(err.Error())
	}
	effective := &repository.Snapshot{Files: map[string][]byte{}, Executable: map[string]bool{}}
	for name, data := range candidate.Files {
		effective.Files[name] = data
		effective.Executable[name] = candidate.Executable[name]
	}
	for _, name := range rule.Command.PolicyFiles {
		if !repository.SafePath(name) {
			return fail("Invalid policy file path: " + name)
		}
		data, ok := base.Files[name]
		if !ok {
			return fail("Approved policy file unavailable: " + name)
		}
		effective.Files[name] = data
		effective.Executable[name] = base.Executable[name]
	}
	if err := effective.Materialize(source); err != nil {
		return fail(err.Error())
	}
	argv := rule.Command.Argv
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(rule.Command.Timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = source
	cmd.Env = commandEnvironment(home, temp)
	cmd.WaitDelay = 2 * time.Second
	cleanup := configureProcess(cmd)
	stdout, stderr := &commandBuffer{limit: 8 << 20, cancel: cancel}, &commandBuffer{limit: 64 << 10, cancel: cancel}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	cleanup()
	if stdout.overflow || stderr.overflow {
		return fail("Command output exceeded its limit.")
	}
	if ctx.Err() != nil {
		return fail("Command timed out; no complete verdict is available.")
	}
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return fail("Could not execute command: " + err.Error())
		}
		code = exit.ExitCode()
	}
	// Outputs are accepted only if the captured source inputs remain unchanged.
	root, err := os.OpenRoot(source)
	if err != nil {
		return fail(err.Error())
	}
	defer root.Close()
	for _, name := range repository.Keys(effective.Files) {
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() != int64(len(effective.Files[name])) || (runtime.GOOS != "windows" && (info.Mode().Perm()&0111 != 0) != effective.Executable[name]) {
			return fail("Command modified a source input: " + name)
		}
		data, err := root.ReadFile(name)
		if err != nil || !bytes.Equal(data, effective.Files[name]) {
			return fail("Command modified a source input: " + name)
		}
	}
	c.Diagnostics = append(c.Diagnostics, fmt.Sprintf("Executed %s in a disposable copy; exit %d. Host execution is not a sandbox; tool version is not independently verified.", cmd.Path, code))
	if rule.Command.Format == "dependency-cruiser" {
		c.Diagnostics = append(c.Diagnostics, "Dependency-cruiser can recover syntax errors without reporting them. This check evaluates its reported graph; syntax validity requires a separate compiler or syntax check.")
	}
	if rule.Command.Format == "exit" {
		switch {
		case slices.Contains(rule.Command.SuccessCodes, code):
			c.Status = "passed"
		case slices.Contains(rule.Command.FailureCodes, code):
			c.Status = "crossed"
			c.Diagnostics = append(c.Diagnostics, "Declared command check failed. Its exit code does not identify a specific semantic violation.")
		default:
			return fail(fmt.Sprintf("Unexpected command exit %d; outcome is unverified.", code))
		}
		if c.Status != "passed" {
			c.Diagnostics = append(c.Diagnostics, boundedDiagnostic(stdout.Bytes(), stderr.Bytes()))
		}
	} else {
		if !slices.Contains(rule.Command.SuccessCodes, code) && !slices.Contains(rule.Command.FailureCodes, code) {
			return fail(fmt.Sprintf("Unexpected tool exit %d: %s", code, boundedDiagnostic(stdout.Bytes(), stderr.Bytes())))
		}
		var findings []model.Finding
		if rule.Command.Format == "dependency-cruiser" {
			findings, err = dependencyFindings(rule, stdout.Bytes(), source, effective.Files, selected)
		} else {
			findings, err = conftestFindings(rule, stdout.Bytes(), source, selected)
		}
		if findings != nil {
			c.Findings = findings
		}
		if err != nil {
			return fail("Tool output cannot establish a verdict: " + err.Error())
		}
		c.Status = "passed"
		if len(findings) > 0 {
			c.Status = "crossed"
		}
		if len(findings) == 0 && !slices.Contains(rule.Command.SuccessCodes, code) {
			return fail("Tool failed without structured findings; analysis is incomplete.")
		}
	}
	return c
}

func commandEnvironment(home, temp string) []string {
	var env []string
	for _, key := range []string{"PATH", "SystemRoot", "SYSTEMROOT", "WINDIR", "PATHEXT"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return append(env, "HOME="+home, "USERPROFILE="+home, "TMPDIR="+temp, "TMP="+temp, "TEMP="+temp, "LANG=C", "TERM=dumb", "NO_COLOR=1", "CI=true", "PYTHONDONTWRITEBYTECODE=1")
}

type commandBuffer struct {
	buffer   bytes.Buffer
	limit    int
	overflow bool
	cancel   context.CancelFunc
}

func (b *commandBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buffer.Len() {
		b.overflow = true
		b.cancel()
		return 0, fmt.Errorf("output limit exceeded")
	}
	return b.buffer.Write(p)
}

func (b *commandBuffer) Bytes() []byte { return b.buffer.Bytes() }

func boundedDiagnostic(out, err []byte) string {
	b := append(append([]byte{}, err...), out...)
	if len(b) > 4096 {
		b = append(b[:4096], []byte("\n[diagnostic truncated]")...)
	}
	return strings.TrimSpace(string(b))
}

func decodeOutput(data []byte, target any) error {
	return jsonstrict.Decode(data, target)
}

func findingPath(name, root string) (string, error) {
	if filepath.IsAbs(name) {
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return "", err
		}
		name = relative
	}
	name = filepath.ToSlash(name)
	name = strings.TrimPrefix(name, "./")
	if !repository.SafePath(name) {
		return "", fmt.Errorf("invalid finding path %q", name)
	}
	return name, nil
}

func externalFinding(rule contract.Rule, path, target, message string) model.Finding {
	key := contract.SemanticDigest(rule) + "\x00" + path + "\x00" + target + "\x00" + message
	return model.Finding{Fingerprint: digest([]byte(key)), Path: path, Target: target, Message: message, Remediation: rule.Remediation, Evolution: "not_comparable"}
}

func dependencyFindings(rule contract.Rule, data []byte, root string, files map[string][]byte, selected []string) ([]model.Finding, error) {
	var result struct {
		Modules *[]struct {
			Source             string `json:"source"`
			Valid              *bool  `json:"valid"`
			CoreModule         bool   `json:"coreModule"`
			CouldNotResolve    bool   `json:"couldNotResolve"`
			MatchesDoNotFollow bool   `json:"matchesDoNotFollow"`
			Consolidated       bool   `json:"consolidated"`
			Dependencies       *[]struct {
				Resolved           string `json:"resolved"`
				CoreModule         bool   `json:"coreModule"`
				CouldNotResolve    *bool  `json:"couldNotResolve"`
				Valid              *bool  `json:"valid"`
				MatchesDoNotFollow bool   `json:"matchesDoNotFollow"`
			} `json:"dependencies"`
		} `json:"modules"`
		Summary *struct {
			TotalCruised *int                       `json:"totalCruised"`
			Error        *int                       `json:"error"`
			Warn         *int                       `json:"warn"`
			Info         *int                       `json:"info"`
			OptionsUsed  map[string]json.RawMessage `json:"optionsUsed"`
			RuleSetUsed  struct {
				Forbidden       []json.RawMessage `json:"forbidden"`
				Required        []json.RawMessage `json:"required"`
				Allowed         []json.RawMessage `json:"allowed"`
				AllowedSeverity string            `json:"allowedSeverity"`
			} `json:"ruleSetUsed"`
			Violations *[]struct {
				From string `json:"from"`
				To   string `json:"to"`
				Type string `json:"type"`
				Rule struct {
					Name     string `json:"name"`
					Severity string `json:"severity"`
				} `json:"rule"`
			} `json:"violations"`
		} `json:"summary"`
	}
	if err := decodeOutput(data, &result); err != nil {
		return nil, err
	}
	if result.Modules == nil || result.Summary == nil || result.Summary.Violations == nil || len(*result.Modules) == 0 {
		return nil, fmt.Errorf("expected nonempty modules and summary.violations")
	}
	summary := result.Summary
	if summary.TotalCruised == nil || *summary.TotalCruised != len(*result.Modules) || summary.Error == nil || summary.Warn == nil || summary.Info == nil || *summary.Error < 0 || *summary.Warn < 0 || *summary.Info < 0 || summary.OptionsUsed == nil {
		return nil, fmt.Errorf("missing or inconsistent dependency summary")
	}
	activeRules := 0
	for _, rules := range [][]json.RawMessage{summary.RuleSetUsed.Forbidden, summary.RuleSetUsed.Required} {
		for _, raw := range rules {
			var identity struct {
				Name     string `json:"name"`
				Severity string `json:"severity"`
			}
			if err := decodeOutput(raw, &identity); err != nil {
				return nil, err
			}
			if strings.TrimSpace(identity.Name) == "" || !slices.Contains([]string{"error", "warn", "info", "ignore"}, identity.Severity) {
				return nil, fmt.Errorf("reported dependency rule lacks a name or recognized severity")
			}
			if identity.Severity != "ignore" {
				activeRules++
			}
		}
	}
	// dependency-cruiser omits names from allowed rules and records their
	// shared severity separately; do not apply the named-rule schema to them.
	for _, raw := range summary.RuleSetUsed.Allowed {
		var allowed struct {
			From map[string]json.RawMessage `json:"from"`
			To   map[string]json.RawMessage `json:"to"`
		}
		if err := decodeOutput(raw, &allowed); err != nil {
			return nil, err
		}
		if allowed.From == nil || allowed.To == nil || !slices.Contains([]string{"error", "warn", "info", "ignore"}, summary.RuleSetUsed.AllowedSeverity) {
			return nil, fmt.Errorf("reported allowed rule lacks restrictions or a recognized allowedSeverity")
		}
		if summary.RuleSetUsed.AllowedSeverity != "ignore" {
			activeRules++
		}
	}
	if activeRules == 0 {
		return nil, fmt.Errorf("no active dependency rules were reported")
	}
	for _, option := range []string{"exclude", "includeOnly", "focus", "reaches", "collapse", "knownViolations", "ignoreKnown", "skipAnalysisNotInRules", "maxDepth"} {
		if raw, ok := summary.OptionsUsed[option]; ok && string(raw) != "false" && string(raw) != "null" && string(raw) != "0" && string(raw) != `""` && string(raw) != "[]" {
			return nil, fmt.Errorf("dependency option %s can hide graph coverage", option)
		}
	}
	covered, cores := map[string]bool{}, map[string]bool{}
	referenced, invalid := map[string]bool{}, map[string]bool{}
	var incomplete error
	for _, module := range *result.Modules {
		if module.Valid == nil || module.Dependencies == nil {
			return nil, fmt.Errorf("module lacks validation or dependencies")
		}
		if module.Source == "" || module.CouldNotResolve || module.MatchesDoNotFollow || module.Consolidated {
			incomplete = fmt.Errorf("unresolved, unvisited or consolidated module %q", module.Source)
		}
		if module.CoreModule {
			cores[module.Source] = true
			continue
		}
		p, err := findingPath(module.Source, root)
		if err != nil {
			return nil, err
		}
		if _, exists := files[p]; !exists {
			return nil, fmt.Errorf("reported module is outside the captured snapshot: %s", p)
		}
		if covered[p] {
			return nil, fmt.Errorf("duplicate module %s", p)
		}
		covered[p] = true
		if !*module.Valid {
			invalid[p+"\x00"] = true
		}
		for _, dep := range *module.Dependencies {
			if dep.Valid == nil || dep.CouldNotResolve == nil || dep.Resolved == "" {
				return nil, fmt.Errorf("incomplete dependency in %s", p)
			}
			if *dep.CouldNotResolve || dep.MatchesDoNotFollow {
				incomplete = fmt.Errorf("unresolved or unvisited dependency in %s", p)
				continue
			}
			target := dep.Resolved
			if !dep.CoreModule {
				target, err = findingPath(dep.Resolved, root)
				if err != nil {
					return nil, err
				}
				if _, exists := files[target]; !exists {
					return nil, fmt.Errorf("dependency is outside the captured snapshot: %s", target)
				}
				referenced[target] = true
			}
			if !*dep.Valid {
				invalid[p+"\x00"+target] = true
			}
		}
	}
	findings := []model.Finding{}
	counts := map[string]int{}
	for _, v := range *result.Summary.Violations {
		p, err := findingPath(v.From, root)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(selected, p) {
			return findings, fmt.Errorf("violation is outside the declared scope: %s", p)
		}
		if v.Rule.Name == "" || !slices.Contains([]string{"error", "warn", "info", "ignore"}, v.Rule.Severity) {
			return nil, fmt.Errorf("violation lacks rule identity")
		}
		if v.Rule.Severity == "ignore" {
			continue
		}
		counts[v.Rule.Severity]++
		target := v.To
		if !cores[target] && !(target == "" && v.Type == "module") {
			target, err = findingPath(target, root)
			if err != nil {
				return findings, err
			}
			if _, exists := files[target]; !exists {
				return findings, fmt.Errorf("violation target is outside the captured snapshot: %s", target)
			}
		}
		findings = append(findings, externalFinding(rule, p, target, v.Rule.Name+": dependency rule violated"))
		delete(invalid, p+"\x00"+target)
		if v.Type == "module" {
			delete(invalid, p+"\x00")
		}
	}
	if counts["error"] != *summary.Error || counts["warn"] != *summary.Warn || counts["info"] != *summary.Info {
		return findings, fmt.Errorf("violation counts disagree with dependency summary")
	}
	if len(invalid) > 0 {
		return findings, fmt.Errorf("invalid modules or dependencies lack summary violations")
	}
	for name := range referenced {
		if !covered[name] {
			return findings, fmt.Errorf("dependency target lacks a module result: %s", name)
		}
	}
	for _, name := range selected {
		if !covered[name] {
			return findings, fmt.Errorf("declared file has no dependency analysis result: %s", name)
		}
	}
	return findings, incomplete
}

func conftestFindings(rule contract.Rule, data []byte, root string, selected []string) ([]model.Finding, error) {
	type message struct {
		Message string `json:"msg"`
	}
	var results []struct {
		Filename   string    `json:"filename"`
		Namespace  string    `json:"namespace"`
		Successes  *int      `json:"successes"`
		Failures   []message `json:"failures"`
		Warnings   []message `json:"warnings"`
		Exceptions []message `json:"exceptions"`
		Skipped    []message `json:"skipped"`
	}
	if err := decodeOutput(data, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no policy results")
	}
	findings := []model.Finding{}
	covered, namespaces := map[string]bool{}, map[string]bool{}
	var incomplete error
	for _, item := range results {
		if item.Successes == nil || *item.Successes < 0 || item.Namespace == "" {
			return nil, fmt.Errorf("result lacks successes or policy namespace")
		}
		p, err := findingPath(item.Filename, root)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(selected, p) {
			return findings, fmt.Errorf("policy result is outside the declared scope: %s", p)
		}
		key := p + "\x00" + item.Namespace
		if namespaces[key] {
			return findings, fmt.Errorf("duplicate policy result for %s", p)
		}
		namespaces[key] = true
		if len(item.Exceptions) > 0 || len(item.Skipped) > 0 {
			incomplete = fmt.Errorf("policy result for %s contains exceptions or skipped checks", p)
		}
		if *item.Successes+len(item.Failures)+len(item.Warnings) == 0 {
			incomplete = fmt.Errorf("no policy checks evaluated for %s", p)
		} else {
			covered[p] = true
		}
		for _, m := range append(item.Failures, item.Warnings...) {
			if m.Message == "" {
				return nil, fmt.Errorf("empty policy finding")
			}
			findings = append(findings, externalFinding(rule, p, item.Namespace, m.Message))
		}
	}
	for _, name := range selected {
		if !covered[name] {
			return findings, fmt.Errorf("declared file has no evaluated policy result: %s", name)
		}
	}
	return findings, incomplete
}
