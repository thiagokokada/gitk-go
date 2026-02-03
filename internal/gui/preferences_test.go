package gui

import (
	"reflect"
	"testing"
)

func TestPreferencesJSONRoundTrip(t *testing.T) {
	input := preferences{
		UIFont:    []string{"DejaVu Sans", "11", "bold"},
		FixedFont: []string{"DejaVu Sans Mono", "10"},
	}
	data, err := encodePreferencesJSON(input)
	if err != nil {
		t.Fatalf("encode prefs: %v", err)
	}
	parsed, err := parsePreferencesJSON(data)
	if err != nil {
		t.Fatalf("parse prefs: %v", err)
	}
	if !reflect.DeepEqual(parsed, input) {
		t.Fatalf("parsed prefs = %#v, want %#v", parsed, input)
	}
}

func TestPreferencesJSONEmpty(t *testing.T) {
	parsed, err := parsePreferencesJSON([]byte(" \n\t"))
	if err != nil {
		t.Fatalf("parse empty prefs: %v", err)
	}
	if !reflect.DeepEqual(parsed, preferences{}) {
		t.Fatalf("parsed prefs = %#v, want empty", parsed)
	}
}
