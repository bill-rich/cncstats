package iniparse

import (
	"bytes"
	"testing"
)

func TestMatchKey(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"BuildCost_asExpected", "  BuildCost", "BuildCost"},
		{"BuildCost_tooManySpaces", "       BuildCost", ""},
		{"BuildCost_noSpaces", "BuildCost", ""},
		{"BuildCost_endlineJunk", "  BuildCost OMG", "BuildCost"},
		{"Object_asExpected", "Object del cool", "Object"},
		{"Object_noExtra", "Object", "Object"},
		{"Object_leadingSpaces", "  Object", ""},
	}

	for _, tc := range cases {
		output := matchKey(tc.input)
		if tc.expected != output {
			t.Errorf("%s: unexpected output. got: %s, expected: %s", tc.name, output, tc.expected)
		}
	}
}

func TestParseFile(t *testing.T) {
	reader := bytes.NewReader([]byte("Object LazrGun\n  junk=nothing\n  BuildCost=123\nEnd"))
	objectStore := &ObjectStore{}
	objectStore.parseFile(reader)
	obj := objectStore.GetObject(2)
	if obj.Name != "LazrGun" && obj.Cost != 123 {
		t.Errorf("parsed object returned bad result: %+v", obj)
	}
}

func TestGetObjectTooLow(t *testing.T) {
	objectStore := &ObjectStore{}
	obj := objectStore.GetObject(1)
	if obj != nil {
		t.Errorf("expected nil when object ID is too low")
	}
}
