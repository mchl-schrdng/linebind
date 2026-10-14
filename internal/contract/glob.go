package contract

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func safePath(p string) bool {
	drivePath := len(p) > 1 && p[1] == ':' && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z'))
	return p != "" && p != "." && !drivePath && !strings.ContainsAny(p, "\\\x00") && !strings.HasPrefix(p, "/") && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../")
}

func validatePattern(p string) error {
	if !safePath(p) || !doublestar.ValidatePattern(p) {
		return fmt.Errorf("invalid repository-relative glob %q", p)
	}
	return nil
}

// Match uses slash-separated doublestar globs, independent of the host OS.
func Match(pattern, name string) (bool, error) {
	if err := validatePattern(pattern); err != nil {
		return false, err
	}
	if !safePath(name) {
		return false, fmt.Errorf("invalid snapshot path %q", name)
	}
	return doublestar.Match(pattern, name)
}

// Select returns a sorted, deduplicated selection from an in-memory snapshot.
func Select(scope []string, files map[string][]byte) ([]string, error) {
	if len(scope) == 0 {
		return nil, fmt.Errorf("scope must not be empty")
	}
	for _, p := range scope {
		if err := validatePattern(p); err != nil {
			return nil, err
		}
	}
	var selected []string
	for name := range files {
		if !safePath(name) {
			return nil, fmt.Errorf("invalid snapshot path %q", name)
		}
		for _, p := range scope {
			matched, _ := doublestar.Match(p, name)
			if matched {
				selected = append(selected, name)
				break
			}
		}
	}
	sort.Strings(selected)
	return selected, nil
}
