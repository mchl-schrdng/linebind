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
	var target struct{ X int }
	if err := Decode([]byte(`{"x":1,"future":true}`), &target); err != nil || target.X != 1 {
		t.Fatalf("valid extensible output rejected: %+v %v", target, err)
	}
}
