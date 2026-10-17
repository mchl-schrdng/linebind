package app

import (
	"slices"
	"sort"

	"github.com/mchl-schrdng/linebind/internal/contract"
	"github.com/mchl-schrdng/linebind/internal/model"
	"github.com/mchl-schrdng/linebind/internal/repository"
)

// Review evidence must never change which policy runs or the CI outcome.
func describeRuleChanges(base, candidate *repository.Snapshot, path string) ([]model.RuleChange, string) {
	sides := [2]map[string]contract.Rule{}
	ids := map[string]bool{}
	for i, snapshot := range []*repository.Snapshot{base, candidate} {
		sides[i] = map[string]contract.Rule{}
		if raw, exists := snapshot.Files[path]; exists {
			c, err := contract.Load(raw)
			if err != nil {
				return nil, "Rule comparison unavailable: a contract could not be parsed. Review the policy file changes."
			}
			for _, rule := range c.Rules {
				sides[i][rule.ID] = rule
				ids[rule.ID] = true
			}
		}
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	var changes []model.RuleChange
	for _, id := range ordered {
		before, had := sides[0][id]
		after, has := sides[1][id]
		change := model.RuleChange{ID: id, Before: before.Specification, After: after.Specification}
		var files []string
		for _, rule := range []contract.Rule{before, after} {
			if rule.Command != nil {
				files = append(files, rule.Command.PolicyFiles...)
			}
		}
		sort.Strings(files)
		for _, file := range slices.Compact(files) {
			a, aOK := base.Files[file]
			b, bOK := candidate.Files[file]
			if aOK != bOK || !policyEqual(a, b) || base.Executable[file] != candidate.Executable[file] {
				change.PolicyFiles = append(change.PolicyFiles, file)
			}
		}
		switch {
		case !had:
			change.Change = "added"
		case !has:
			change.Change = "removed"
		case change.Before != change.After || len(change.PolicyFiles) > 0:
			change.Change = "modified"
		default:
			continue
		}
		changes = append(changes, change)
	}
	return changes, ""
}
