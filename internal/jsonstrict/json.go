// Package jsonstrict rejects ambiguous JSON before decoding typed results.
package jsonstrict

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

// Decode accepts one JSON value with unique object keys and at most 128 nested
// containers. Known struct fields must use their exact JSON names; Go's
// case-insensitive field matching must not hide contradictory evidence.
// Unknown fields remain allowed for forward-compatible tool output.
func Decode(data []byte, target any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("JSON is not valid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := value(d, 0, reflect.TypeOf(target)); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON value")
	}
	return json.Unmarshal(data, target)
}

func value(d *json.Decoder, depth int, target reflect.Type) error {
	if depth > 128 {
		return fmt.Errorf("JSON nesting exceeds 128 levels")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, container := token.(json.Delim)
	if !container {
		return nil
	}
	if delim != '{' && delim != '[' {
		return fmt.Errorf("unexpected JSON delimiter")
	}
	for target != nil && target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	seen := map[string]bool{}
	for d.More() {
		var child reflect.Type
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate or invalid JSON object key %q", name)
			}
			seen[name] = true
			child, err = fieldType(target, name)
			if err != nil {
				return err
			}
		} else if target != nil && (target.Kind() == reflect.Slice || target.Kind() == reflect.Array) {
			child = target.Elem()
		}
		if err := value(d, depth+1, child); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}

func fieldType(target reflect.Type, name string) (reflect.Type, error) {
	if target == nil {
		return nil, nil
	}
	if target.Kind() == reflect.Map {
		return target.Elem(), nil
	}
	if target.Kind() != reflect.Struct {
		return nil, nil
	}
	var folded string
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		key, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if field.PkgPath != "" || key == "-" {
			continue
		}
		if key == "" {
			key = field.Name
		}
		if name == key {
			return field.Type, nil
		}
		if strings.EqualFold(name, key) {
			folded = key
		}
	}
	if folded != "" {
		return nil, fmt.Errorf("JSON field %q must use the exact spelling %q", name, folded)
	}
	return nil, nil
}
