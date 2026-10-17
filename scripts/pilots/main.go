// Command pilots checks synthetic fixtures and, optionally, one pinned public
// repository through an installed Linebind binary. Upstream code is never run.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mchl-schrdng/linebind/internal/model"
)

const upstreamURL = "https://github.com/sindresorhus/p-limit.git"
const upstreamSHA = "a8a6fbec4e0e866d6d779b10889bb4f5567e70eb"

type result struct {
	Name           string `json:"name"`
	Source         string `json:"source"`
	Variant        string `json:"variant"`
	Status         string `json:"status"`
	ExitCode       int    `json:"exit_code"`
	Base           string `json:"base"`
	UpstreamCommit string `json:"upstream_commit,omitempty"`
	Report         string `json:"report"`
	HTML           string `json:"html"`
}

type runner struct {
	binary, examples, out, hooks string
	results                      []result
}

func main() {
	binary := flag.String("binary", "", "installed or extracted Linebind binary")
	examples := flag.String("examples", "examples", "path to this project's example fixtures")
	out := flag.String("out", "", "new evidence directory (defaults to a retained temporary directory)")
	public := flag.Bool("public", false, "also fetch the pinned p-limit repository; requires network access")
	dataCommand := flag.Bool("data-command", false, "also execute the trusted SQLite fixture checker; requires python3")
	flag.Parse()
	if err := run(*binary, *examples, *out, *public, *dataCommand); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(binary, examples, out string, public, dataCommand bool) error {
	if binary == "" {
		return fmt.Errorf("-binary is required")
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(binary, ".exe") {
		binary += ".exe"
	}
	var err error
	binary, err = filepath.Abs(binary)
	if err != nil {
		return err
	}
	examples, err = filepath.Abs(examples)
	if err != nil {
		return err
	}
	if out == "" {
		out, err = os.MkdirTemp("", "linebind-pilots-")
	} else {
		out, err = filepath.Abs(out)
		if err == nil {
			err = os.Mkdir(out, 0o755)
		}
	}
	if err != nil {
		return err
	}
	fmt.Println("Evidence:", out)
	r := runner{binary: binary, examples: examples, out: out, hooks: filepath.Join(out, "empty-hooks")}
	if err := os.Mkdir(r.hooks, 0o755); err != nil {
		return err
	}
	version, err := successful(out, binary, "--version")
	if err != nil {
		return err
	}
	binaryData, err := os.ReadFile(binary)
	if err != nil {
		return err
	}
	for _, name := range []string{"frontend", "backend", "data", "infrastructure"} {
		if err := r.fixture(name); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	var python string
	if dataCommand {
		version, err := successful(out, "python3", "--version")
		if err != nil {
			return fmt.Errorf("data command fixture requires python3: %w", err)
		}
		python = strings.TrimSpace(string(version))
		if err := r.dataCommand(); err != nil {
			return fmt.Errorf("data command fixture: %w", err)
		}
	}
	if public {
		if err := r.publicPilot(); err != nil {
			return fmt.Errorf("public p-limit pilot: %w", err)
		}
	}
	evidence := struct {
		GeneratedAt  string   `json:"generated_at"`
		Binary       string   `json:"binary_version"`
		BinarySHA256 string   `json:"binary_sha256"`
		Platform     string   `json:"platform"`
		RunnerGo     string   `json:"runner_go_version"`
		Python       string   `json:"python_version,omitempty"`
		Results      []result `json:"results"`
	}{time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(string(version)), fmt.Sprintf("%x", sha256.Sum256(binaryData)), runtime.GOOS + "/" + runtime.GOARCH, runtime.Version(), python, r.results}
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "results.json"), append(data, '\n'), 0o644)
}

func (r *runner) fixture(name string) error {
	dir := filepath.Join(r.out, "work", name)
	source := filepath.Join(r.examples, name)
	if err := copyTree(filepath.Join(source, "base"), dir); err != nil {
		return err
	}
	if _, err := r.git(dir, "init", "--quiet", "--initial-branch=main"); err != nil {
		return err
	}
	base, hash, err := r.approve(dir)
	if err != nil {
		return err
	}
	for _, variant := range []string{"passed", "crossed", "unverified", "contract-removed"} {
		if err := copyTree(filepath.Join(source, "base"), dir); err != nil {
			return err
		}
		overlay := variant
		if variant == "contract-removed" {
			overlay = "crossed"
		}
		if variant != "passed" {
			if err := copyTree(filepath.Join(source, overlay), dir); err != nil {
				return err
			}
		}
		if variant == "contract-removed" {
			if err := os.Remove(filepath.Join(dir, ".linebind.yml")); err != nil {
				return err
			}
		}
		if err := r.check(name, "synthetic-fixture", variant, dir, base, hash, "", false); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) dataCommand() error {
	dir := filepath.Join(r.out, "work", "data-command")
	source := filepath.Join(r.examples, "data-command")
	if err := copyTree(filepath.Join(source, "base"), dir); err != nil {
		return err
	}
	if _, err := r.git(dir, "init", "--quiet", "--initial-branch=main"); err != nil {
		return err
	}
	base, hash, err := r.approve(dir)
	if err != nil {
		return err
	}
	for _, variant := range []string{"passed", "duplicate", "null", "unverified", "write-attempt", "exec-disabled"} {
		if err := copyTree(filepath.Join(source, "base"), dir); err != nil {
			return err
		}
		if variant != "passed" && variant != "exec-disabled" {
			if err := copyTree(filepath.Join(source, variant), dir); err != nil {
				return err
			}
		}
		if err := r.check("data-command", "synthetic-fixture-with-trusted-checker", variant, dir, base, hash, "", variant != "exec-disabled"); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) publicPilot() error {
	dir := filepath.Join(r.out, "work", "p-limit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := r.git(dir, "init", "--quiet", "--initial-branch=main"); err != nil {
		return err
	}
	if _, err := r.git(dir, "fetch", "--quiet", "--no-tags", "--depth=1", upstreamURL, upstreamSHA); err != nil {
		return err
	}
	if _, err := r.git(dir, "checkout", "--quiet", "--detach", upstreamSHA); err != nil {
		return err
	}
	actual, err := r.git(dir, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(actual)) != upstreamSHA {
		return fmt.Errorf("upstream checkout did not resolve to the pinned SHA: %s (%v)", actual, err)
	}
	policy, err := os.ReadFile(filepath.Join(r.examples, "public-p-limit", ".linebind.yml"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, ".linebind.yml"), policy, 0o644); err != nil {
		return err
	}
	base, hash, err := r.approve(dir)
	if err != nil {
		return err
	}
	manifest := filepath.Join(dir, "package.json")
	original, err := os.ReadFile(manifest)
	if err != nil {
		return err
	}
	var pkg map[string]any
	if err := json.Unmarshal(original, &pkg); err != nil {
		return err
	}
	if pkg["type"] != "module" {
		return fmt.Errorf("pinned package no longer has the expected ESM declaration")
	}
	pkg["type"] = "commonjs"
	changed, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}
	for _, variant := range []string{"passed", "crossed", "unverified", "contract-removed"} {
		data := original
		if variant == "crossed" || variant == "contract-removed" {
			data = changed
		} else if variant == "unverified" {
			data = []byte(`{"type":`)
		}
		if err := os.WriteFile(manifest, data, 0o644); err != nil {
			return err
		}
		if variant == "contract-removed" {
			if err := os.Remove(filepath.Join(dir, ".linebind.yml")); err != nil {
				return err
			}
		}
		if err := r.check("p-limit", "public-snapshot-with-seeded-mutations", variant, dir, base, hash, upstreamSHA, false); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) approve(dir string) (string, string, error) {
	if _, err := successful(dir, r.binary, "validate"); err != nil {
		return "", "", err
	}
	if _, err := r.git(dir, "add", "--all"); err != nil {
		return "", "", err
	}
	if _, err := r.git(dir, "commit", "--quiet", "-m", "Linebind-authored experimental contract"); err != nil {
		return "", "", err
	}
	base, err := r.git(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	policy, err := os.ReadFile(filepath.Join(dir, ".linebind.yml"))
	return strings.TrimSpace(string(base)), fmt.Sprintf("%x", sha256.Sum256(policy)), err
}

func (r *runner) check(name, source, variant, dir, base, hash, upstream string, allowExec bool) error {
	status, code := variant, 0
	switch variant {
	case "crossed", "contract-removed", "duplicate", "null":
		status, code = "crossed", 1
	case "unverified", "write-attempt", "exec-disabled":
		status, code = "unverified", 2
	}
	stem := filepath.Join(r.out, name+"-"+variant)
	args := []string{r.binary, "check", "--base", base, "--head", "worktree", "--repo", dir,
		"--format", "json", "--output", stem + ".json", "--html", stem + ".html"}
	if allowExec {
		args = append(args, "--allow-exec")
	}
	output, exit, err := invoke(dir, args...)
	if err != nil || exit != code {
		return fmt.Errorf("%s/%s: exit=%d, expected=%d: %v\n%s", name, variant, exit, code, err, output)
	}
	data, err := os.ReadFile(stem + ".json")
	if err != nil {
		return err
	}
	var report model.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return err
	}
	if (report.SchemaVersion != 1 && report.SchemaVersion != 2) || report.Status != status || report.ExitCode != code || report.Base != base || report.ContractHash != hash {
		return fmt.Errorf("%s/%s: unexpected verdict or approved-policy provenance: %s", name, variant, data)
	}
	if len(report.Checks) != 1 || report.Checks[0].Status != status {
		return fmt.Errorf("%s/%s: expected one %s check, received %s", name, variant, status, data)
	}
	if status == "crossed" && report.Checks[0].Evidence != "command" && len(report.Checks[0].Findings) == 0 {
		return fmt.Errorf("%s/%s: crossed check has no findings", name, variant)
	}
	if name == "data-command" {
		check := report.Checks[0]
		if check.Evidence != "command" || len(check.Findings) != 0 {
			return fmt.Errorf("%s/%s: an exit-code check must not invent structured findings", name, variant)
		}
		diagnostic := strings.Join(check.Diagnostics, "\n")
		expected := map[string]string{"duplicate": "duplicate order_id=2", "null": "null order_id=3", "unverified": "Unexpected command exit 2", "write-attempt": "Unexpected command exit 2", "exec-disabled": "Project command was not executed"}
		if fragment := expected[variant]; fragment != "" && !strings.Contains(diagnostic, fragment) {
			return fmt.Errorf("%s/%s: missing expected diagnostic %q: %s", name, variant, fragment, diagnostic)
		}
		if variant == "exec-disabled" && strings.Contains(diagnostic, "Executed ") {
			return fmt.Errorf("%s: checker unexpectedly executed without opt-in", name)
		}
	}
	if status == "unverified" && len(report.Checks[0].Diagnostics) == 0 {
		return fmt.Errorf("%s/%s: unverified check has no explanation", name, variant)
	}
	if variant == "contract-removed" && (len(report.Changes) != 1 || report.Changes[0].Path != ".linebind.yml" || report.Changes[0].Change != "removed") {
		return fmt.Errorf("%s: candidate contract deletion was not reported", name)
	}
	if _, err := successful(dir, r.binary, "report", stem+".json", "--output", stem+".rerendered.html"); err != nil {
		return err
	}
	originalHTML, err := os.ReadFile(stem + ".html")
	if err != nil {
		return err
	}
	rerendered, err := os.ReadFile(stem + ".rerendered.html")
	if err != nil {
		return err
	}
	if !bytes.Equal(originalHTML, rerendered) || !bytes.Contains(bytes.ToLower(originalHTML), []byte("<html")) {
		return fmt.Errorf("%s/%s: report did not reproduce the original HTML", name, variant)
	}
	r.results = append(r.results, result{name, source, variant, status, code, base, upstream, stem + ".json", stem + ".html"})
	fmt.Printf("PASS %-14s %-18s %s (exit %d)\n", name, variant, status, code)
	return nil
}

func (r *runner) git(dir string, args ...string) ([]byte, error) {
	options := []string{"git", "-c", "core.hooksPath=" + r.hooks, "-c", "core.autocrlf=false", "-c", "commit.gpgsign=false",
		"-c", "user.name=Linebind Examples", "-c", "user.email=examples@linebind.invalid"}
	return successful(dir, append(options, args...)...)
}

func successful(dir string, args ...string) ([]byte, error) {
	output, code, err := invoke(dir, args...)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("%s: exit %d: %v\n%s", strings.Join(args, " "), code, err, output)
	}
	return output, nil
}

func invoke(dir string, args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return output, -1, ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return output, exit.ExitCode(), nil
	}
	return output, 0, err
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("fixture contains a non-regular file: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
