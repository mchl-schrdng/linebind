package contract

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/mchl-schrdng/linebind/internal/model"
	"golang.org/x/mod/modfile"
)

func evaluateDependencies(r Rule, selected []string, files map[string][]byte, check *model.Check) {
	for _, name := range selected {
		if len(files[name]) > 16<<20 {
			check.Diagnostics = append(check.Diagnostics, name+": manifest exceeds 16 MiB")
			continue
		}
		var deps []string
		var diagnostics []string
		var err error
		switch r.Dependencies.Format {
		case "npm":
			deps, diagnostics, err = npmDependencies(files[name], r.Dependencies.Sections)
		case "go":
			deps, diagnostics, err = goDependencies(name, files[name])
		case "python":
			deps, diagnostics, err = pythonDependencies(files[name], r.Dependencies.Sections)
		case "cargo":
			deps, diagnostics, err = cargoDependencies(files[name], r.Dependencies.Sections)
		}
		if err != nil {
			check.Diagnostics = append(check.Diagnostics, name+": "+err.Error())
			continue
		}
		for _, diagnostic := range diagnostics {
			check.Diagnostics = append(check.Diagnostics, name+": "+diagnostic)
		}
		sort.Strings(deps)
		deps = slices.Compact(deps)
		for _, dep := range deps {
			denied := slices.Contains(r.Dependencies.Deny, dep)
			if r.Dependencies.Allow != nil {
				denied = !slices.Contains(r.Dependencies.Allow, dep)
			}
			if denied {
				if len(check.Findings) == maxFindings {
					check.Diagnostics = append(check.Diagnostics, "finding limit reached; remaining dependency findings were not reported")
					return
				}
				check.Findings = append(check.Findings, finding(r, name, dep, "Direct dependency is not permitted: "+dep))
			}
		}
	}
}

func npmDependencies(data []byte, sections []string) ([]string, []string, error) {
	if !json.Valid(data) {
		return nil, nil, fmt.Errorf("invalid npm JSON manifest")
	}
	// The YAML node pass rejects duplicate JSON keys before encoding/json can
	// silently keep only the last definition.
	if _, err := parseDocuments(data); err != nil {
		return nil, nil, err
	}
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(data, &manifest); err != nil || manifest == nil {
		return nil, nil, fmt.Errorf("npm manifest must be an object")
	}
	var deps, diagnostics []string
	for _, section := range sections {
		raw, present := manifest[section]
		if !present {
			continue
		}
		var entries map[string]string
		if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
			return nil, nil, fmt.Errorf("npm %s must map package names to string specifications", section)
		}
		names := make([]string, 0, len(entries))
		for name := range entries {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			spec := entries[name]
			if name == "" || strings.TrimSpace(spec) == "" {
				return nil, nil, fmt.Errorf("npm %s contains an empty package name or specification", section)
			}
			deps = append(deps, name)
			lowerSpec := strings.ToLower(spec)
			if strings.HasPrefix(spec, "npm:") {
				alias := strings.TrimPrefix(spec, "npm:")
				if at := strings.LastIndex(alias, "@"); at > 0 {
					alias = alias[:at]
				}
				if alias == "" || strings.ContainsAny(alias, " :\t\r\n") || (strings.HasPrefix(alias, "@") && !strings.Contains(alias, "/")) {
					return nil, nil, fmt.Errorf("invalid npm alias for %s", name)
				}
				deps = append(deps, alias)
			} else if strings.ContainsAny(spec, ":/\\") || strings.HasPrefix(spec, ".") ||
				strings.HasSuffix(lowerSpec, ".tgz") || strings.HasSuffix(lowerSpec, ".tar.gz") || strings.HasSuffix(lowerSpec, ".tar") {
				diagnostics = append(diagnostics, "dependency "+name+" uses a non-registry source whose package identity is unverified")
			}
		}
	}
	return deps, diagnostics, nil
}

func goDependencies(name string, data []byte) ([]string, []string, error) {
	f, err := modfile.Parse(name, data, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid go.mod: %w", err)
	}
	if f.Module == nil {
		return nil, nil, fmt.Errorf("go.mod has no module declaration")
	}
	var deps, diagnostics []string
	for _, require := range f.Require {
		if require.Indirect {
			continue
		}
		deps = append(deps, require.Mod.Path)
		for _, replace := range f.Replace {
			if replace.Old.Path != require.Mod.Path || (replace.Old.Version != "" && replace.Old.Version != require.Mod.Version) {
				continue
			}
			if replace.New.Version == "" {
				diagnostics = append(diagnostics, "direct module "+require.Mod.Path+" has a local replacement whose package identity is unverified")
			} else {
				deps = append(deps, replace.New.Path)
			}
		}
	}
	return deps, diagnostics, nil
}
