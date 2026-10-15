package jsonstrict

import (
	"strings"
	"testing"
)

func TestDecodeRejectsAmbiguousJSON(t *testing.T) {
	for _, data := range []string{`{"x":1,"x":2}`, `{"x":{"a":1,"\u0061":2}}`, `{} {}`, `[]oops`, `{"x":"` + string([]byte{255}) + `"}`, strings.Repeat("[", 130) + "0" + strings.Repeat("]", 130), "", `{"x":`} {
		var target any
		if err := Decode([]byte(data), &target); err == nil {
			t.Errorf("accepted invalid JSON %q", data)
		}
	}
	var target struct {
		X int `json:"x"`
	}
	if err := Decode([]byte(`{"x":1,"future":true}`), &target); err != nil || target.X != 1 {
		t.Fatalf("valid extensible output rejected: %+v %v", target, err)
	}
}

func TestKnownFieldsRejectCaseAliasesWithoutChangingMapKeys(t *testing.T) {
	type item struct {
		Kind   string `json:"kind"`
		Status string `json:"status"`
	}
	type output struct {
		Items  []*item           `json:"items"`
		Labels map[string]string `json:"labels"`
	}
	for _, data := range []string{
		`{"items":[{"status":"crossed","STATUS":"passed"}]}`,
		`{"items":[{"STATUS":"passed"}]}`,
		`{"items":[{"status":"crossed","ſtatus":"passed"}]}`,
		`{"items":[{"kind":"command","Kind":"native"}]}`,
		`{"items":[],"ITEMS":[]}`,
	} {
		var result output
		if err := Decode([]byte(data), &result); err == nil {
			t.Errorf("accepted a known field under another spelling: %s", data)
		}
	}
	var result output
	data := `{"items":[{"status":"passed","future":{"A":1,"a":2}}],"labels":{"A":"one","a":"two"},"Future":true,"future":false}`
	if err := Decode([]byte(data), &result); err != nil || result.Labels["A"] != "one" || result.Labels["a"] != "two" {
		t.Fatalf("unknown fields or case-sensitive map keys rejected: %+v %v", result, err)
	}
}
