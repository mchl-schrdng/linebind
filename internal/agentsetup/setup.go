// Package agentsetup installs repository-local instructions from this binary.
package agentsetup

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/mchl-schrdng/linebind/internal/repository"
)

//go:embed skill
var skill embed.FS

// Change describes one destination. Action is create, append, or unchanged.
type Change struct {
	Path   string
	Action string
}

type plannedFile struct {
	Change
	content  []byte
	previous []byte
}

const blockStart = "<!-- linebind:begin -->"
const blockEnd = "<!-- linebind:end -->"

// Setup installs the bundled skill and a small instruction block. All conflicts
// are checked before writing. Existing skill contents are never replaced.
func Setup(repoDir, agent string, dryRun bool) ([]Change, error) {
	var targets []string
	switch agent {
	case "codex", "antigravity":
		targets = []string{".agents"}
	case "claude":
		targets = []string{".claude"}
	case "all":
		targets = []string{".agents", ".claude"}
	default:
		return nil, fmt.Errorf("setup requires --agent codex, claude, antigravity, or all")
	}
	repo, err := repository.Open(repoDir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(repo.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	var plan []plannedFile
	for _, target := range targets {
		skillDir := target + "/skills/linebind"
		err := fs.WalkDir(skill, "skill", func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return walkErr
			}
			content, err := skill.ReadFile(name)
			if err != nil {
				return err
			}
			destination := skillDir + strings.TrimPrefix(name, "skill")
			previous, exists, err := readDestination(root, destination)
			if err != nil {
				return err
			}
			action := "create"
			if exists {
				if !bytes.Equal(previous, content) {
					return fmt.Errorf("%s differs from the bundled skill; preserve or reconcile it manually before setup", destination)
				}
				action = "unchanged"
			}
			plan = append(plan, plannedFile{Change{destination, action}, content, previous})
			return nil
		})
		if err != nil {
			return nil, err
		}
		instructions := "AGENTS.md"
		if target == ".claude" {
			instructions = "CLAUDE.md"
		}
		previous, exists, err := readDestination(root, instructions)
		if err != nil {
			return nil, err
		}
		block := instructionBlock(skillDir)
		if bytes.Contains(previous, []byte("\r\n")) {
			block = strings.ReplaceAll(block, "\n", "\r\n")
		}
		action, content, err := planInstructions(previous, exists, block)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", instructions, err)
		}
		plan = append(plan, plannedFile{Change{instructions, action}, content, previous})
	}
	sort.Slice(plan, func(i, j int) bool { return plan[i].Path < plan[j].Path })
	changes := make([]Change, len(plan))
	for i, file := range plan {
		changes[i] = file.Change
	}
	if dryRun {
		return changes, nil
	}
	for _, file := range plan {
		if file.Action == "unchanged" {
			continue
		}
		if err := writePlanned(root, file); err != nil {
			return nil, fmt.Errorf("setup stopped at %s: %w; inspect the changes before retrying", file.Path, err)
		}
	}
	return changes, nil
}

func instructionBlock(skillDir string) string {
	return blockStart + "\n## Linebind\n\n" +
		"Use the Linebind skill at `" + skillDir + "/SKILL.md` to create, check, or evolve this repository's `.linebind.yml` contract.\n" +
		"Choose `--base` explicitly from the repository's approved revisions; neither `HEAD` nor a candidate contract is approved by default.\n" +
		"Before finishing changes, run `linebind check --base <approved-ref> --agent` and report violations or incomplete verification.\n" +
		"Correct unintended deviations in the code. For an intentional policy change, prepare the contract diff and verification evidence for the repository's approval process, using existing explicit authorization when applicable. Never weaken rules or change the base merely to obtain a passing result.\n" +
		blockEnd + "\n"
}

func planInstructions(previous []byte, exists bool, block string) (string, []byte, error) {
	start, end := bytes.Index(previous, []byte(blockStart)), bytes.Index(previous, []byte(blockEnd))
	if start >= 0 || end >= 0 {
		if bytes.Count(previous, []byte(blockStart)) != 1 || bytes.Count(previous, []byte(blockEnd)) != 1 || end < start {
			return "", nil, fmt.Errorf("ambiguous Linebind instruction markers; reconcile them manually")
		}
		old := previous[start : end+len(blockEnd)]
		if !bytes.Equal(old, []byte(strings.TrimRight(block, "\r\n"))) {
			return "", nil, fmt.Errorf("the Linebind instruction block differs; preserve or reconcile it manually before setup")
		}
		return "unchanged", previous, nil
	}
	if !exists {
		return "create", []byte(block), nil
	}
	separator := "\n\n"
	if bytes.Contains(previous, []byte("\r\n")) {
		separator = "\r\n\r\n"
	}
	if len(previous) == 0 {
		separator = ""
	}
	return "append", []byte(separator + block), nil
}

// Inspect every component before opening anything: OpenRoot prevents escape,
// while this check also rejects links pointing elsewhere inside the repository.
func inspectDestination(root *os.Root, name string) (os.FileInfo, error) {
	parts := strings.Split(name, "/")
	for i := range parts {
		current := strings.Join(parts[:i+1], "/")
		info, err := root.Lstat(current)
		if os.IsNotExist(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if i < len(parts)-1 {
			if !info.IsDir() {
				return nil, fmt.Errorf("%s must be a real directory, not a symlink or special file", current)
			}
		} else {
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("%s must be a regular file, not a symlink or special file", name)
			}
			return info, nil
		}
	}
	return nil, nil
}

func readDestination(root *os.Root, name string) ([]byte, bool, error) {
	info, err := inspectDestination(root, name)
	if err != nil || info == nil {
		return nil, false, err
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, false, fmt.Errorf("%s changed while setup was reading it", name)
	}
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > 1<<20 {
		return nil, false, fmt.Errorf("%s exceeds the 1 MiB setup file limit", name)
	}
	return data, true, nil
}

func writePlanned(root *os.Root, file plannedFile) error {
	current, exists, err := readDestination(root, file.Path)
	if err != nil {
		return err
	}
	if (file.Action == "create" && exists) || (file.Action == "append" && (!exists || !bytes.Equal(current, file.previous))) {
		return fmt.Errorf("destination changed since setup planned the installation")
	}
	parent := path.Dir(file.Path)
	if parent != "." {
		parts := strings.Split(parent, "/")
		for i := range parts {
			name := strings.Join(parts[:i+1], "/")
			if err := root.Mkdir(name, 0755); err != nil && !os.IsExist(err) {
				return err
			}
			info, err := root.Lstat(name)
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return fmt.Errorf("%s is no longer a real directory", name)
			}
		}
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if file.Action == "append" {
		flags = os.O_WRONLY | os.O_APPEND
	}
	before, err := inspectDestination(root, file.Path)
	if err != nil {
		return err
	}
	f, err := root.OpenFile(file.Path, flags, 0644)
	if err != nil {
		return err
	}
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || (file.Action == "append" && (before == nil || !os.SameFile(before, opened))) {
		f.Close()
		return fmt.Errorf("destination changed before setup could write it")
	}
	if _, err := inspectDestination(root, file.Path); err != nil {
		f.Close()
		return err
	}
	_, writeErr := f.Write(file.content)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		if file.Action == "create" {
			root.Remove(file.Path)
		}
		return errors.Join(writeErr, closeErr)
	}
	return nil
}
