// Linebind checks repository changes against an explicitly approved contract.
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/mchl-schrdng/linebind/internal/agentsetup"
	"github.com/mchl-schrdng/linebind/internal/app"
	"github.com/mchl-schrdng/linebind/internal/contract"
	"github.com/mchl-schrdng/linebind/internal/jsonstrict"
	"github.com/mchl-schrdng/linebind/internal/model"
	"github.com/mchl-schrdng/linebind/internal/report"
	"github.com/mchl-schrdng/linebind/internal/repository"
)

var version = "dev"
var commit = "unknown"

const help = `Linebind — Your repository. Your contract.

Usage:
  linebind setup --agent codex|claude|antigravity|all [--repo DIR] [--dry-run]
  linebind init [--repo DIR] [--contract .linebind.yml]
  linebind validate [--repo DIR] [--contract .linebind.yml]
  linebind check --base REF [--head REF|worktree] [options]
  linebind preview --base REF [--head REF|worktree] [options]
  linebind report INPUT.json [--output .linebind/report.html]
  linebind --version

Check and preview options:
  --repo DIR         Git repository (default .)
  --contract PATH    Repository-relative contract (default .linebind.yml)
  --base REF         Required reference; check reads its approved contract
  --head REF         Candidate revision, or worktree (default worktree)
  --format FORMAT    text or json (default text)
  --agent           Compact plain-text evidence with input identifiers
  --output PATH     Save text/JSON instead of writing it to stdout
  --html PATH       Also save the standalone HTML report
  --allow-exec      Allow policy commands on this host (not a sandbox)

Setup installs the bundled skill and repository instructions without network
access. It preserves existing content and does not create or approve a contract.
Preview uses the candidate contract; its base is only a comparison reference.
Preview can validate proposed rules in CI; it does not approve their adoption.
Native checks never execute project code. Missing or incomplete evidence is
unverified. Exit codes: 0 = required checks passed, 1 = required violation
(or policy change in check), 2 = required verification incomplete or operational error.
Advisory findings are reported without blocking the gate.
`

const starter = `version: 1
name: Repository contract
rules:
  - id: readme-required
    kind: require-files
    reason: Keep the project entry point available to contributors.
    remediation: Add or restore README.md, then describe the actual project rules in this contract.
    files:
      require: [README.md]
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	fail := func(err error) int {
		fmt.Fprintln(errOut, "linebind:", safeText(err.Error()))
		return 2
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		if _, err := io.WriteString(out, help); err != nil {
			return fail(err)
		}
		return 0
	}
	if args[0] == "--version" || args[0] == "version" {
		if _, err := fmt.Fprintf(out, "linebind %s (%s)\n", version, commit); err != nil {
			return fail(err)
		}
		return 0
	}
	command := args[0]
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() { fmt.Fprint(errOut, help) }
	if command == "report" {
		output := fs.String("output", ".linebind/report.html", "HTML report path")
		rest := args[1:]
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			rest = append(append([]string{}, rest[1:]...), rest[0])
		}
		if err := fs.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if fs.NArg() != 1 {
			return fail(fmt.Errorf("report requires one JSON input file"))
		}
		data, err := readLimited(fs.Arg(0), 32<<20)
		if err != nil {
			return fail(err)
		}
		r, err := loadReport(data)
		if err != nil {
			return fail(err)
		}
		var html bytes.Buffer
		if err := renderHTML(&html, r); err != nil {
			return fail(err)
		}
		if err := writeArtifact(*output, html.Bytes(), "html"); err != nil {
			return fail(err)
		}
		fmt.Fprintln(errOut, "Report saved:", safeText(*output))
		return 0
	}
	if command == "setup" {
		repoDir := fs.String("repo", ".", "Git repository")
		agent := fs.String("agent", "", "codex, claude, antigravity or all")
		dryRun := fs.Bool("dry-run", false, "show planned writes without changing files")
		if err := fs.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if fs.NArg() != 0 {
			return fail(fmt.Errorf("setup accepts options only; use --agent codex|claude|antigravity|all"))
		}
		changes, err := agentsetup.Setup(*repoDir, *agent, *dryRun)
		if err != nil {
			return fail(err)
		}
		if *dryRun {
			fmt.Fprintln(out, "Setup preview; no files written.")
		}
		for _, change := range changes {
			fmt.Fprintf(out, "%s %s\n", safeText(change.Action), safeText(change.Path))
		}
		fmt.Fprintln(out, "The bundled skill uses this Linebind CLI. No contract was created or approved. Review and commit the integration files; reload skills or start a new agent session if needed.")
		return 0
	}
	if command != "init" && command != "validate" && command != "check" && command != "preview" {
		return fail(fmt.Errorf("unknown command %q; use --help", command))
	}
	repoDir := fs.String("repo", ".", "Git repository")
	contractPath := fs.String("contract", ".linebind.yml", "repository-relative contract")
	var base, head, format, output, html string
	var agent, allowExec bool
	if command == "check" || command == "preview" {
		fs.StringVar(&base, "base", "", "reference revision; approved policy source for check")
		fs.StringVar(&head, "head", "worktree", "candidate revision")
		fs.StringVar(&format, "format", "text", "text or json")
		fs.StringVar(&output, "output", "", "text or JSON report path")
		fs.StringVar(&html, "html", "", "HTML report path")
		fs.BoolVar(&agent, "agent", false, "plain-text evidence for an agent")
		fs.BoolVar(&allowExec, "allow-exec", false, "execute policy commands on this host")
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		return fail(fmt.Errorf("unexpected positional arguments; place options after the command"))
	}
	if !repository.SafePath(*contractPath) {
		return fail(fmt.Errorf("contract must be a safe repository-relative path"))
	}
	if command == "init" {
		if err := initialize(*repoDir, *contractPath); err != nil {
			return fail(err)
		}
		fmt.Fprintf(out, "Created %s and ignored /.linebind/.\nThis starter checks README.md only. Adapt it, run linebind validate, then review and commit the contract before checking changes.\n", safeText(*contractPath))
		return 0
	}
	if command == "validate" {
		repo, err := repository.Open(*repoDir)
		if err != nil {
			return fail(err)
		}
		root, err := os.OpenRoot(repo.Root)
		if err != nil {
			return fail(err)
		}
		defer root.Close()
		info, err := root.Lstat(*contractPath)
		if err != nil {
			return fail(err)
		}
		if !info.Mode().IsRegular() {
			return fail(fmt.Errorf("contract must be a regular file"))
		}
		f, err := root.Open(*contractPath)
		if err != nil {
			return fail(err)
		}
		defer f.Close()
		opened, err := f.Stat()
		if err != nil {
			return fail(err)
		}
		if !opened.Mode().IsRegular() {
			return fail(fmt.Errorf("contract must be a regular file"))
		}
		data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
		if err != nil {
			return fail(err)
		}
		c, err := contract.Load(data)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintf(out, "Valid contract: %s (%d rules). No checks were executed.\n", safeText(c.Name), len(c.Rules))
		return 0
	}
	if base == "" {
		return fail(fmt.Errorf("%s requires --base: choose the reference revision explicitly (an approved policy source for check)", command))
	}
	if format != "text" && format != "json" {
		return fail(fmt.Errorf("format must be text or json"))
	}
	if agent && format != "text" {
		return fail(fmt.Errorf("--agent uses text; omit --format json"))
	}
	if output != "" && html != "" {
		a, _ := filepath.Abs(output)
		b, _ := filepath.Abs(html)
		if a == b {
			return fail(fmt.Errorf("--output and --html must be different paths"))
		}
	}
	opt := app.Options{Repo: *repoDir, Base: base, Head: head, Contract: *contractPath, Version: version, AllowExec: allowExec}
	var r model.Report
	if command == "preview" {
		r = app.Preview(opt)
	} else {
		r = app.Check(opt)
	}
	var data bytes.Buffer
	if format == "json" {
		e := json.NewEncoder(&data)
		e.SetIndent("", "  ")
		if err := e.Encode(r); err != nil {
			return fail(err)
		}
	} else if err := report.Text(&data, r, agent); err != nil {
		return fail(err)
	}
	if output != "" {
		if err := writeArtifact(output, data.Bytes(), format); err != nil {
			return fail(err)
		}
	} else {
		if _, err := out.Write(data.Bytes()); err != nil {
			return fail(err)
		}
	}
	if html != "" {
		var b bytes.Buffer
		if err := renderHTML(&b, r); err != nil {
			return fail(err)
		}
		if err := writeArtifact(html, b.Bytes(), "html"); err != nil {
			return fail(err)
		}
	}
	return r.ExitCode
}

func initialize(dir, name string) error {
	repo, err := repository.Open(dir)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(repo.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := root.Lstat(name); err == nil {
		return fmt.Errorf("%s already exists; init never replaces a contract", name)
	} else if !os.IsNotExist(err) {
		return err
	}
	var ignored bool
	if info, err := root.Lstat(".gitignore"); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf(".gitignore must be a regular file")
		}
		data, err := root.ReadFile(".gitignore")
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == "/.linebind/" {
				ignored = true
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := root.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(f, starter)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		root.Remove(name)
		return errors.Join(writeErr, closeErr)
	}
	if !ignored {
		ignore, err := root.OpenFile(".gitignore", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			root.Remove(name)
			return err
		}
		_, writeErr := io.WriteString(ignore, "\n# Local Linebind reports\n/.linebind/\n")
		closeErr := ignore.Close()
		if writeErr != nil || closeErr != nil {
			root.Remove(name)
			return errors.Join(writeErr, closeErr)
		}
	}
	return nil
}

func safeText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
			return -1
		}
		return r
	}, s)
}

func readLimited(name string, limit int64) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("input must be a regular file")
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("input must be a regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("input exceeds %d bytes", limit)
	}
	return b, nil
}

const htmlMarker = "<!-- Generated by Linebind; report schema 1. -->\n"

func renderHTML(w io.Writer, r model.Report) error {
	if _, err := io.WriteString(w, htmlMarker); err != nil {
		return err
	}
	return report.Render(w, r)
}

// Explicit report paths may replace an earlier Linebind report, never an
// unrelated file. A temporary sibling keeps interrupted writes out of reports.
func writeArtifact(name string, data []byte, kind string) error {
	if info, err := os.Lstat(name); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace non-regular output %s", name)
		}
		previous, err := readLimited(name, 32<<20)
		if err != nil {
			return err
		}
		recognized := false
		switch kind {
		case "html":
			recognized = bytes.HasPrefix(previous, []byte(htmlMarker))
		case "json":
			_, err := loadReport(previous)
			recognized = err == nil
		case "text":
			recognized = bytes.HasPrefix(previous, []byte("Linebind ")) && bytes.Contains(previous, []byte("Results apply only to the declared checks and recorded inputs."))
		}
		if !recognized {
			return fmt.Errorf("refusing to overwrite %s: it is not a recognized Linebind %s report", name, kind)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(name), ".linebind-report-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		return errors.Join(writeErr, closeErr)
	}
	return os.Rename(temp, name)
}

func loadReport(data []byte) (model.Report, error) {
	var r model.Report
	if err := jsonstrict.Decode(data, &r); err != nil {
		return r, fmt.Errorf("invalid report: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		return r, fmt.Errorf("invalid report: %w", err)
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return r, fmt.Errorf("expected one JSON report")
	}
	_, dateErr := time.Parse(time.RFC3339, r.GeneratedAt)
	if r.SchemaVersion != 1 || (r.Purpose != "" && r.Purpose != "preview") || r.Version == "" || dateErr != nil || !validStatus(r.Status) || r.ExitCode < 0 || r.ExitCode > 2 {
		return r, fmt.Errorf("unsupported or incomplete Linebind report")
	}
	if len(r.Checks) == 0 {
		if len(r.Diagnostics) == 0 || r.ExitCode == 0 || (len(r.Changes) == 0 && (r.Status != "unverified" || r.ExitCode != 2)) {
			return r, fmt.Errorf("report has neither verified checks nor an operational diagnostic")
		}
	} else {
		if r.Repository == "" || !revisionHash(r.Base) || !validCandidate(r.Candidate, r.InputDigest) || !hexHash(r.InputDigest, 64) || !hexHash(r.ContractHash, 64) || !repository.SafePath(r.ContractPath) {
			return r, fmt.Errorf("report lacks complete input and contract provenance")
		}
		status, exit := app.Outcome(r)
		if r.Status != status || r.ExitCode != exit {
			return r, fmt.Errorf("report summary contradicts its checks")
		}
	}
	seen := map[string]bool{}
	for _, c := range r.Checks {
		if c.ID == "" || seen[c.ID] || !validStatus(c.Status) || c.Files < 0 || c.DurationMS < 0 || strings.TrimSpace(c.Reason) == "" || len(c.Scope) == 0 || !slices.Contains([]string{"native", "structured", "command"}, c.Evidence) || !slices.Contains([]string{"require-files", "forbid-text", "dependencies", "document", "command"}, c.Kind) {
			return r, fmt.Errorf("invalid or duplicate report check")
		}
		seen[c.ID] = true
		for _, f := range c.Findings {
			if !hexHash(f.Fingerprint, 64) || f.Path == "" || f.Message == "" || f.Line < 0 || !slices.Contains([]string{"new", "existing", "resolved", "not_comparable"}, f.Evolution) {
				return r, fmt.Errorf("incomplete report finding")
			}
			if c.Status == "passed" && f.Evolution != "existing" && f.Evolution != "resolved" {
				return r, fmt.Errorf("passed check contains a blocking finding")
			}
		}
	}
	if r.Purpose != "preview" && len(r.Changes) > 0 && (r.Status != "crossed" || r.ExitCode != 1) {
		return r, fmt.Errorf("report gate contradicts contract changes")
	}
	for _, change := range r.Changes {
		if !repository.SafePath(change.Path) || !slices.Contains([]string{"added", "modified", "removed"}, change.Change) {
			return r, fmt.Errorf("invalid contract change record")
		}
	}
	return r, nil
}

func hexHash(s string, length int) bool {
	if len(s) != length {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func revisionHash(s string) bool { return hexHash(s, 40) || hexHash(s, 64) }
func validCandidate(s, input string) bool {
	if revisionHash(s) {
		return true
	}
	parts := strings.Split(s, ":")
	return len(parts) == 3 && parts[0] == "worktree" && revisionHash(parts[1]) && len(parts[2]) == 12 && strings.HasPrefix(input, parts[2])
}
func validStatus(s string) bool { return s == "passed" || s == "crossed" || s == "unverified" }
