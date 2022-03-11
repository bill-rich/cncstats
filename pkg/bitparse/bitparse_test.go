package bitparse

import (
	"bytes"
	"testing"
)

func TestNullTermString(t *testing.T) {

	type testCase struct {
		input    []byte
		expected string
		encoding string
	}

	testCases := map[string]testCase{
		"UTF16_null_end": {
			input:    []byte{76, 0, 97, 0, 115, 0, 116, 0, 32, 0, 82, 0, 101, 0, 112, 0, 108, 0, 97, 0, 121, 0, 0},
			expected: "Last Replay",
			encoding: "utf16",
		},
		"UTF16_repeat": {
			input:    []byte{76, 0, 97, 0, 115, 0, 116, 0, 32, 0, 82, 0, 101, 0, 112, 0, 108, 0, 97, 0, 121, 0, 0, 0, 76, 0, 97, 0},
			expected: "Last Replay",
			encoding: "utf16",
		},
		"UTF16_no termination": {
			input:    []byte{76, 0, 97, 0, 115, 0, 116, 0, 32, 0, 82, 0, 101, 0, 112, 0, 108, 0, 97, 0, 121},
			expected: "Last Replay",
			encoding: "utf16",
		},
		"UTF8_null_end": {
			input:    []byte{76, 97, 115, 116, 32, 82, 101, 112, 108, 97, 121, 0, 0},
			expected: "Last Replay",
			encoding: "utf8",
		},
		"UTF8_repeat": {
			input:    []byte{76, 97, 115, 116, 32, 82, 101, 112, 108, 97, 121, 0, 0, 76, 97},
			expected: "Last Replay",
			encoding: "utf8",
		},
		"UTF8_no_termination": {
			input:    []byte{76, 97, 115, 116, 32, 82, 101, 112, 108, 97, 121},
			expected: "Last Replay",
			encoding: "utf8",
		},
	}

	for name, tc := range testCases {
		parser := BitParser{
			Source: bytes.NewReader(tc.input),
		}
		output := parser.ReadNullTermString(tc.encoding)
		if output != tc.expected {
			t.Errorf("%s - expected: %q, got: %q", name, tc.expected, output)
		}
	}
}

func TestString(t *testing.T) {

	type testCase struct {
		input    []byte
		expected string
		size     int
	}

	testCases := map[string]testCase{
		"UTF8_null_end": {
			input:    []byte{76, 97, 115, 116, 32, 82, 101, 112, 108, 97, 121},
			expected: "Last Replay",
			size:     11,
		},
	}

	for name, tc := range testCases {
		parser := BitParser{
			Source: bytes.NewReader(tc.input),
		}
		output := parser.ReadString(tc.size)
		if output != tc.expected {
			t.Errorf("%s - expected: %q, got: %q", name, tc.expected, output)
		}
	}
}

func TestInt32(t *testing.T) {

	type testCase struct {
		input    []byte
		expected int
	}

	testCases := map[string]testCase{
		"uint32": {
			input:    []byte{75, 163, 87, 98},
			expected: 1649910603,
		},
	}

	for name, tc := range testCases {
		parser := BitParser{
			Source: bytes.NewReader(tc.input),
		}
		output := parser.ReadUInt32()
		if output != tc.expected {
			t.Errorf("%s - expected: %d, got: %d", name, tc.expected, output)
		}
	}
}

func TestInt16(t *testing.T) {
	type testCase struct {
		input    []byte
		expected int
	}

	testCases := map[string]testCase{
		"uint16": {
			input:    []byte{07, 00},
			expected: 7,
		},
	}

	for name, tc := range testCases {
		parser := BitParser{
			Source: bytes.NewReader(tc.input),
		}
		output := parser.ReadUInt16()
		if output != tc.expected {
			t.Errorf("%s - expected: %d, got: %d", name, tc.expected, output)
		}
	}
}

func TestBytes(t *testing.T) {
	type testCase struct {
		input    []byte
		expected []byte
		size     int
	}

	testCases := map[string]testCase{
		"one_byte": {
			input:    []byte{07},
			expected: []byte{07},
			size:     1,
		},
		"two_bytes": {
			input:    []byte{07, 00},
			expected: []byte{07, 00},
			size:     2,
		},
		"not_enough": {
			input:    []byte{07},
			expected: []byte{07, 00},
			size:     2,
		},
	}

	for name, tc := range testCases {
		parser := BitParser{
			Source: bytes.NewReader(tc.input),
		}
		output := parser.ReadBytes(tc.size)
		if !bytes.Equal(output, tc.expected) {
			t.Errorf("%s - expected: %v, got: %v", name, tc.expected, output)
		}
	}
}
