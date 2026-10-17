package app

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/contract"
	"github.com/mchl-schrdng/linebind/internal/model"
	"github.com/mchl-schrdng/linebind/internal/repository"
)

func TestRealArchitecturePanel(t *testing.T) {
	bin := os.Getenv("LINEBIND_DEPCRUISE")
	if bin == "" {
		t.Skip("set LINEBIND_DEPCRUISE to pinned dependency-cruiser 17.3.8 executable")
	}
	version, err := exec.Command(bin, "--version").Output()
	if err != nil || strings.TrimSpace(string(version)) != "17.3.8" {
		t.Fatalf("expected dependency-cruiser 17.3.8: %s %v", version, err)
	}
	base := loadExampleSnapshot(t, "dependency-cruiser", "base")
	config, err := contract.Load(base.Files[".linebind.yml"])
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range config.Rules {
		if rule.Command.Format == "dependency-cruiser" {
			rule.Command.Argv[0] = bin
		}
	}
	for _, tc := range []struct {
		name  string
		files map[string]string
		exit  int
	}{
		{name: "baseline", exit: 0},
		{name: "client-split", exit: 0, files: map[string]string{
			"src/client/view.mjs":  `import { label } from "./label.mjs"; export const renderName = label;`,
			"src/client/label.mjs": `import { getProfile } from "../api/profile.mjs"; export const label = () => getProfile().name;`,
		}},
		{name: "api-barrel", exit: 0, files: map[string]string{
			"src/api/index.mjs":   `export { getProfile } from "./profile.mjs";`,
			"src/client/view.mjs": `import { getProfile } from "../api/index.mjs"; export const renderName = () => getProfile().name;`,
		}},
		{name: "shared-helper", exit: 0, files: map[string]string{
			"src/shared/case.mjs": `export const title = (name) => name;`,
			"src/shared/name.mjs": `import { title } from "./case.mjs"; export const cleanName = (name) => title(name.trim());`,
		}},
		{name: "adapter-refactor", exit: 0, files: map[string]string{
			"src/adapters/store.mjs":   `export const record = { name: "Ada" };`,
			"src/adapters/profile.mjs": `import { record } from "./store.mjs"; export const loadProfile = () => record;`,
		}},
		{name: "comment-import", exit: 0, files: map[string]string{
			"src/client/view.mjs": `import { getProfile } from "../api/profile.mjs";
export const renderName = () => getProfile().name;
// import { formatProfile } from "../domain/profile.mjs";
`,
		}},
		{name: "known-layer-orphan", exit: 0, files: map[string]string{
			"src/domain/unused.mjs": `export const VERSION = 1;`,
		}},
		{name: "javascript-extensions", exit: 0, files: map[string]string{
			"src/shared/title.js": `export const title = (name) => name;`,
			"src/shared/trim.cjs": `module.exports = (name) => name.trim();`,
		}},
		{name: "dynamic-api-literal", exit: 0, files: map[string]string{
			"src/client/view.mjs": `export const renderName = async () => (await import("../api/profile.mjs")).getProfile().name;`,
		}},
		{name: "direct-domain", exit: 1, files: map[string]string{
			"src/client/view.mjs": `import { formatProfile } from "../domain/profile.mjs"; export const renderName = () => formatProfile({ name: "Ada" }).name;`,
		}},
		{name: "shared-relay", exit: 1, files: map[string]string{
			"src/shared/relay.mjs": `import { formatProfile } from "../domain/profile.mjs"; export const readName = () => formatProfile({ name: "Ada" }).name;`,
			"src/client/view.mjs":  `import { readName } from "../shared/relay.mjs"; export const renderName = readName;`,
		}},
		{name: "shared-reexport", exit: 1, files: map[string]string{
			"src/shared/relay.mjs": `export { formatProfile } from "../domain/profile.mjs";`,
			"src/client/view.mjs":  `import { formatProfile } from "../shared/relay.mjs"; export const renderName = () => formatProfile({ name: "Ada" }).name;`,
		}},
		{name: "api-client", exit: 1, files: map[string]string{
			"src/client/constants.mjs": `export const displayName = "Ada";`,
			"src/api/profile.mjs":      `import { displayName } from "../client/constants.mjs"; export const getProfile = () => ({ name: displayName });`,
		}},
		{name: "domain-adapter", exit: 1, files: map[string]string{
			"src/domain/profile.mjs": `import { loadProfile } from "../adapters/profile.mjs"; export const formatProfile = () => loadProfile();`,
		}},
		{name: "client-adapter", exit: 1, files: map[string]string{
			"src/client/view.mjs": `import { loadProfile } from "../adapters/profile.mjs"; export const renderName = () => loadProfile().name;`,
		}},
		{name: "cycle", exit: 1, files: map[string]string{
			"src/shared/name.mjs": `import { title } from "./case.mjs"; export const cleanName = (name) => title(name.trim());`,
			"src/shared/case.mjs": `import { cleanName } from "./name.mjs"; export const title = cleanName;`,
		}},
		{name: "three-module-cycle", exit: 1, files: map[string]string{
			"src/shared/name.mjs": `import { title } from './case.mjs'; export const cleanName = title;`,
			"src/shared/case.mjs": `import { trim } from './trim.mjs'; export const title = trim;`,
			"src/shared/trim.mjs": `import { cleanName } from './name.mjs'; export const trim = cleanName;`,
		}},
		{name: "unmapped-connected", exit: 1, files: map[string]string{
			"src/services/profile.mjs": `export const getProfile = () => ({ name: "Ada" });`,
			"src/client/view.mjs":      `import { getProfile } from "../services/profile.mjs"; export const renderName = () => getProfile().name;`,
		}},
		{name: "unmapped-orphan", exit: 1, files: map[string]string{
			"src/services/profile.mjs": `export const getProfile = () => ({ name: "Ada" });`,
		}},
		{name: "unmapped-root", exit: 1, files: map[string]string{
			"src/profile.mjs": `export const getProfile = () => ({ name: "Ada" });`,
		}},
		{name: "core-in-client", exit: 1, files: map[string]string{
			"src/client/view.mjs": `import fs from "node:fs"; export const renderName = () => fs.readFileSync("profile.txt", "utf8");`,
		}},
		{name: "dynamic-domain-literal", exit: 1, files: map[string]string{
			"src/client/view.mjs": `export const renderName = async () => (await import("../domain/profile.mjs")).formatProfile({ name: "Ada" }).name;`,
		}},
		{name: "unresolved", exit: 2, files: map[string]string{
			"src/client/view.mjs": `import { getProfile } from "../api/missing.mjs"; export const renderName = () => getProfile().name;`,
		}},
		{name: "invalid-syntax", exit: 1, files: map[string]string{
			"src/client/view.mjs": `export const = ;`,
		}},
		{name: "invalid-cjs-syntax", exit: 1, files: map[string]string{
			"src/shared/trim.cjs": `module.exports = ;`,
		}},
		{name: "invalid-later-module", exit: 1, files: map[string]string{
			"src/shared/a-valid.mjs":   `export const value = 1;`,
			"src/shared/z-invalid.mjs": `export const = ;`,
		}},
		// Known blind spot: the graph tool omits computed import targets. A pass cannot prove this boundary.
		{name: "known-limit-dynamic-domain-computed", exit: 0, files: map[string]string{
			"src/client/view.mjs": `const target = "../domain/profile.mjs"; export const renderName = async () => (await import(target)).formatProfile({ name: "Ada" }).name;`,
		}},
		{name: "unsupported-extension", exit: 2, files: map[string]string{
			"src/client/template.vue": `<script>import { formatProfile } from "../domain/profile.mjs";</script>`,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := &repository.Snapshot{Files: map[string][]byte{}}
			for path, data := range base.Files {
				candidate.Files[path] = data
			}
			for path, data := range tc.files {
				candidate.Files[path] = []byte(data)
			}
			var report model.Report
			for _, rule := range config.Rules {
				report.Checks = append(report.Checks, runCommand(rule, candidate, base, true))
			}
			finalize(&report)
			if report.ExitCode != tc.exit {
				t.Fatalf("architecture policy %s: %+v", tc.name, report)
			}
		})
	}
}
