package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/model"
	"github.com/mchl-schrdng/linebind/internal/repository"
)

func TestNewComparisonFromEmptyBaseScope(t *testing.T) {
	for _, kind := range []struct {
		name, block, clean, crossed string
	}{
		{"forbid-text", "text:\n      pattern: banned", "allowed", "banned"},
		{"dependencies", "dependencies:\n      format: npm\n      deny: [banned]", `{"dependencies":{}}`, `{"dependencies":{"banned":"1"}}`},
		{"document", "document:\n      selector: /mode\n      equals: safe", `{"mode":"safe"}`, `{"mode":"unsafe"}`},
	} {
		policy := "version: 1\nname: new scope\nrules:\n  - id: first\n    kind: " + kind.name + "\n    reason: Preserve the decision for new files.\n    scope: [new/*.json]\n    compare: new\n    " + kind.block + "\n"
		for _, tc := range []struct {
			name, content string
			exit          int
		}{{"clean", kind.clean, 0}, {"violation", kind.crossed, 1}, {"invalid", string([]byte{0xff}), 2}} {
			t.Run(kind.name+"/"+tc.name, func(t *testing.T) {
				dir, opt := checkRepo(t, policy, "unrelated")
				put(t, dir, "new/config.json", tc.content)
				for _, run := range []func(Options) model.Report{Check, Preview} {
					got := run(opt)
					if got.ExitCode != tc.exit {
						t.Fatalf("want exit %d: %+v", tc.exit, got)
					}
					if tc.exit == 1 && got.Checks[0].Findings[0].Evolution != "new" {
						t.Fatalf("first finding was not new: %+v", got)
					}
				}
			})
		}
	}
}

func TestEmptyBaselineDoesNotHideOmittedInputs(t *testing.T) {
	for _, location := range []string{"base", "candidate"} {
		t.Run(location, func(t *testing.T) {
			dir, opt := checkRepo(t, testContract, "allowed")
			put(t, dir, "src/large.txt", strings.Repeat("x", repository.MaxFileBytes+1))
			if location == "base" {
				if err := os.Remove(filepath.Join(dir, "src/code.txt")); err != nil {
					t.Fatal(err)
				}
				testGit(t, dir, "add", ".")
				testGit(t, dir, "commit", "-qm", "omitted base")
				opt.Base = testGit(t, dir, "rev-parse", "HEAD")
				put(t, dir, "src/large.txt", "allowed")
			}
			got := Check(opt)
			if got.ExitCode != 2 || got.Checks[0].Status != "unverified" {
				t.Fatalf("omitted evidence became a known empty scope: %+v", got)
			}
			if location == "base" && !strings.Contains(strings.Join(got.Checks[0].Diagnostics, "\n"), "Base: src/large.txt: file exceeds") {
				t.Fatalf("lost base diagnostic: %+v", got)
			}
		})
	}
}

const coChangePolicy = `version: 1
name: Migration delivery
rules:
  - id: migration-guide
    kind: co-change
    reason: Ship migration instructions with schema changes.
    scope: [migrations/**]
    co-change:
      require: [docs/migrations.md]
`

func TestCoChangeUsesTrustedPolicyAndBothEndpoints(t *testing.T) {
	dir, opt := checkRepo(t, coChangePolicy, "unrelated")
	put(t, dir, "migrations/001.sql", "create table orders(id int);")
	for _, run := range []func(Options) model.Report{Check, Preview} {
		got := run(opt)
		if got.ExitCode != 1 || got.Checks[0].Kind != "co-change" || got.Checks[0].Findings[0].Path != "migrations/001.sql" {
			t.Fatal(got)
		}
	}
	put(t, dir, ".linebind.yml", strings.Replace(coChangePolicy, "scope: [migrations/**]", "scope: [unrelated/**]", 1))
	if got := Check(opt); got.ExitCode != 1 || got.Checks[0].Status != "crossed" || len(got.Changes) != 1 {
		t.Fatalf("candidate weakened its own co-change policy: %+v", got)
	}
	if got := Preview(opt); got.ExitCode != 0 || len(got.Changes) != 1 {
		t.Fatalf("proposal semantics lost: %+v", got)
	}
	put(t, dir, ".linebind.yml", coChangePolicy)
	put(t, dir, "docs/migrations.md", "Apply migration 001.")
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-qm", "complete delivery")
	opt.Head = testGit(t, dir, "rev-parse", "HEAD")
	if got := Check(opt); got.ExitCode != 0 {
		t.Fatalf("complete delivery failed: %+v", got)
	}
}

func TestCoChangeOmissionsOnEitherSideRemainUnverified(t *testing.T) {
	for _, name := range []string{"migrations/001.sql", "docs/migrations.md"} {
		for _, side := range []string{"base", "candidate"} {
			t.Run(side+"/"+name, func(t *testing.T) {
				dir, opt := checkRepo(t, coChangePolicy, "unrelated")
				put(t, dir, name, strings.Repeat("x", repository.MaxFileBytes+1))
				if side == "base" {
					testGit(t, dir, "add", ".")
					testGit(t, dir, "commit", "-qm", "oversized input")
					opt.Base = testGit(t, dir, "rev-parse", "HEAD")
					put(t, dir, name, "captured")
				}
				if got := Check(opt); got.ExitCode != 2 {
					t.Fatalf("incomplete %s evidence accepted: %+v", side, got)
				}
			})
		}
	}
}
