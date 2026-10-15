package app

import "testing"

// Temporary negative test: removed after verifying the required GitHub CI blocks merging.
func TestRequiredCIGateProbe(t *testing.T) {
	t.Fatal("intentional CI failure to verify main cannot merge failing tests")
}
