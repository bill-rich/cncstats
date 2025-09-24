package bitparse

import (
	"bytes"
	"math"
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
		"UTF8_normal": {
			input:    []byte{76, 97, 115, 116, 32, 82, 101, 112, 108, 97, 121},
			expected: "Last Replay",
			size:     11,
		},
		"UTF8_partial": {
			input:    []byte{76, 97, 115, 116, 32, 82, 101, 112, 108, 97, 121, 0, 0},
			expected: "Last Replay",
			size:     11,
		},
		"UTF8_empty": {
			input:    []byte{},
			expected: "",
			size:     0,
		},
		"UTF8_insufficient_data": {
			input:    []byte{76, 97},
			expected: "La\x00\x00\x00\x00\x00\x00\x00\x00", // Padded with null bytes
			size:     10,
		},
		"UTF8_single_char": {
			input:    []byte{65},
			expected: "A",
			size:     1,
		},
		"UTF8_with_nulls": {
			input:    []byte{72, 101, 108, 108, 111, 0, 87, 111, 114, 108, 100},
			expected: "Hello",
			size:     5,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output := parser.ReadString(tc.size)
			if output != tc.expected {
				t.Errorf("expected: %q, got: %q", tc.expected, output)
			}
		})
	}
}

func TestUInt32(t *testing.T) {
	type testCase struct {
		input    []byte
		expected int
	}

	testCases := map[string]testCase{
		"uint32_normal": {
			input:    []byte{75, 163, 87, 98},
			expected: 1649910603,
		},
		"uint32_zero": {
			input:    []byte{0, 0, 0, 0},
			expected: 0,
		},
		"uint32_max": {
			input:    []byte{255, 255, 255, 255},
			expected: 4294967295,
		},
		"uint32_small": {
			input:    []byte{1, 0, 0, 0},
			expected: 1,
		},
		"uint32_insufficient_data": {
			input:    []byte{1, 2},
			expected: 0, // Should return 0 when insufficient data
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output := parser.ReadUInt32()
			if output != tc.expected {
				t.Errorf("expected: %d, got: %d", tc.expected, output)
			}
		})
	}
}

func TestUInt16(t *testing.T) {
	type testCase struct {
		input    []byte
		expected int
	}

	testCases := map[string]testCase{
		"uint16_normal": {
			input:    []byte{7, 0},
			expected: 7,
		},
		"uint16_zero": {
			input:    []byte{0, 0},
			expected: 0,
		},
		"uint16_max": {
			input:    []byte{255, 255},
			expected: 65535,
		},
		"uint16_small": {
			input:    []byte{1, 0},
			expected: 1,
		},
		"uint16_insufficient_data": {
			input:    []byte{1},
			expected: 0, // Should return 0 when insufficient data
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output := parser.ReadUInt16()
			if output != tc.expected {
				t.Errorf("expected: %d, got: %d", tc.expected, output)
			}
		})
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
			input:    []byte{7},
			expected: []byte{7},
			size:     1,
		},
		"two_bytes": {
			input:    []byte{7, 0},
			expected: []byte{7, 0},
			size:     2,
		},
		"insufficient_data": {
			input:    []byte{7},
			expected: []byte{7, 0}, // Padded with null bytes
			size:     2,
		},
		"empty_input": {
			input:    []byte{},
			expected: []byte{0, 0}, // Padded with null bytes
			size:     2,
		},
		"zero_size": {
			input:    []byte{1, 2, 3},
			expected: []byte{},
			size:     0,
		},
		"large_size": {
			input:    []byte{1, 2, 3, 4, 5},
			expected: []byte{1, 2, 3, 4, 5},
			size:     5,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output := parser.ReadBytes(tc.size)
			if !bytes.Equal(output, tc.expected) {
				t.Errorf("expected: %v, got: %v", tc.expected, output)
			}
		})
	}
}

func TestUInt8(t *testing.T) {
	type testCase struct {
		input    []byte
		expected int
	}

	testCases := map[string]testCase{
		"uint8_normal": {
			input:    []byte{42},
			expected: 42,
		},
		"uint8_zero": {
			input:    []byte{0},
			expected: 0,
		},
		"uint8_max": {
			input:    []byte{255},
			expected: 255,
		},
		"uint8_insufficient_data": {
			input:    []byte{},
			expected: 0, // Should return 0 when insufficient data
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output := parser.ReadUInt8()
			if output != tc.expected {
				t.Errorf("expected: %d, got: %d", tc.expected, output)
			}
		})
	}
}

func TestUInt(t *testing.T) {
	type testCase struct {
		input     []byte
		expected  int
		byteCount int
	}

	testCases := map[string]testCase{
		"uint_1_byte": {
			input:     []byte{42},
			expected:  42,
			byteCount: 1,
		},
		"uint_2_bytes": {
			input:     []byte{7, 0},
			expected:  7,
			byteCount: 2,
		},
		"uint_3_bytes": {
			input:     []byte{1, 0, 0},
			expected:  1,
			byteCount: 3,
		},
		"uint_4_bytes": {
			input:     []byte{1, 0, 0, 0},
			expected:  1,
			byteCount: 4,
		},
		"uint_zero": {
			input:     []byte{0, 0, 0, 0},
			expected:  0,
			byteCount: 4,
		},
		"uint_insufficient_data": {
			input:     []byte{1, 2},
			expected:  0, // Should return 0 when insufficient data
			byteCount: 4,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output := parser.ReadUInt(tc.byteCount)
			if output != tc.expected {
				t.Errorf("expected: %d, got: %d", tc.expected, output)
			}
		})
	}
}

func TestReadInt(t *testing.T) {
	type testCase struct {
		input     []byte
		expected  uint32
		byteCount int
	}

	testCases := map[string]testCase{
		"int_1_byte": {
			input:     []byte{42},
			expected:  42,
			byteCount: 1,
		},
		"int_2_bytes": {
			input:     []byte{7, 0},
			expected:  7,
			byteCount: 2,
		},
		"int_4_bytes": {
			input:     []byte{1, 0, 0, 0},
			expected:  1,
			byteCount: 4,
		},
		"int_zero": {
			input:     []byte{0, 0, 0, 0},
			expected:  0,
			byteCount: 4,
		},
		"int_insufficient_data": {
			input:     []byte{1, 2},
			expected:  0, // Should return 0 when insufficient data
			byteCount: 4,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output := parser.ReadInt(tc.byteCount)
			if output != tc.expected {
				t.Errorf("expected: %d, got: %d", tc.expected, output)
			}
		})
	}
}

func TestReadFloat(t *testing.T) {
	type testCase struct {
		input    []byte
		expected float32
	}

	testCases := map[string]testCase{
		"float_zero": {
			input:    []byte{0, 0, 0, 0},
			expected: 0.0,
		},
		"float_one": {
			input:    []byte{0, 0, 128, 63}, // 1.0 in IEEE 754
			expected: 1.0,
		},
		"float_negative": {
			input:    []byte{0, 0, 128, 191}, // -1.0 in IEEE 754
			expected: -1.0,
		},
		"float_small": {
			input:    []byte{0, 0, 0, 60}, // 0.125 in IEEE 754
			expected: 0.125,
		},
		"float_insufficient_data": {
			input:    []byte{1, 2},
			expected: 0.0, // Should return 0 when insufficient data
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output := parser.ReadFloat()
			if !math.IsNaN(float64(tc.expected)) && !math.IsNaN(float64(output)) {
				// Both are NaN, consider them equal
			} else if output != tc.expected {
				t.Errorf("expected: %f, got: %f", tc.expected, output)
			}
		})
	}
}

func TestReadBool(t *testing.T) {
	type testCase struct {
		input    []byte
		expected bool
	}

	testCases := map[string]testCase{
		"bool_true": {
			input:    []byte{1},
			expected: true,
		},
		"bool_false": {
			input:    []byte{0},
			expected: false,
		},
		"bool_non_zero": {
			input:    []byte{42},
			expected: true,
		},
		"bool_max": {
			input:    []byte{255},
			expected: true,
		},
		"bool_insufficient_data": {
			input:    []byte{},
			expected: false, // Should return false when insufficient data
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output := parser.ReadBool()
			if output != tc.expected {
				t.Errorf("expected: %t, got: %t", tc.expected, output)
			}
		})
	}
}

func TestMakeLittleEndian(t *testing.T) {
	type testCase struct {
		input    []byte
		expected []byte
	}

	testCases := map[string]testCase{
		"single_byte": {
			input:    []byte{42},
			expected: []byte{42},
		},
		"two_bytes": {
			input:    []byte{1, 2},
			expected: []byte{2, 1},
		},
		"four_bytes": {
			input:    []byte{1, 2, 3, 4},
			expected: []byte{4, 3, 2, 1},
		},
		"empty": {
			input:    []byte{},
			expected: []byte{},
		},
		"three_bytes": {
			input:    []byte{1, 2, 3},
			expected: []byte{3, 2, 1},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			output := makeLittleEndian(tc.input)
			if !bytes.Equal(output, tc.expected) {
				t.Errorf("expected: %v, got: %v", tc.expected, output)
			}
		})
	}
}

func TestIsNull(t *testing.T) {
	type testCase struct {
		input    []byte
		expected bool
	}

	testCases := map[string]testCase{
		"all_null": {
			input:    []byte{0, 0, 0},
			expected: true,
		},
		"single_null": {
			input:    []byte{0},
			expected: true,
		},
		"mixed": {
			input:    []byte{0, 1, 0},
			expected: false,
		},
		"no_null": {
			input:    []byte{1, 2, 3},
			expected: false,
		},
		"empty": {
			input:    []byte{},
			expected: true,
		},
		"single_non_null": {
			input:    []byte{1},
			expected: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			output := isNull(tc.input)
			if output != tc.expected {
				t.Errorf("expected: %t, got: %t", tc.expected, output)
			}
		})
	}
}

func TestNullTermStringEdgeCases(t *testing.T) {
	type testCase struct {
		input    []byte
		expected string
		encoding string
	}

	testCases := map[string]testCase{
		"empty_input": {
			input:    []byte{},
			expected: "",
			encoding: "utf8",
		},
		"immediate_null_utf8": {
			input:    []byte{0},
			expected: "",
			encoding: "utf8",
		},
		"immediate_null_utf16": {
			input:    []byte{0, 0},
			expected: "",
			encoding: "utf16",
		},
		"unknown_encoding": {
			input:    []byte{72, 101, 108, 108, 111, 0},
			expected: "",
			encoding: "unknown",
		},
		"utf16_partial": {
			input:    []byte{72, 0}, // Only one byte of UTF16
			expected: "H",
			encoding: "utf16",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output := parser.ReadNullTermString(tc.encoding)
			if output != tc.expected {
				t.Errorf("expected: %q, got: %q", tc.expected, output)
			}
		})
	}
}

func TestBitParserEdgeCases(t *testing.T) {
	t.Run("empty_reader", func(t *testing.T) {
		parser := BitParser{
			Source: bytes.NewReader([]byte{}),
		}

		// Test all methods with empty input
		if parser.ReadUInt8() != 0 {
			t.Error("ReadUInt8 should return 0 for empty input")
		}
		if parser.ReadUInt16() != 0 {
			t.Error("ReadUInt16 should return 0 for empty input")
		}
		if parser.ReadUInt32() != 0 {
			t.Error("ReadUInt32 should return 0 for empty input")
		}
		if parser.ReadFloat() != 0.0 {
			t.Error("ReadFloat should return 0.0 for empty input")
		}
		if parser.ReadBool() != false {
			t.Error("ReadBool should return false for empty input")
		}
		if parser.ReadString(5) != "\x00\x00\x00\x00\x00" {
			t.Error("ReadString should return null-padded string for empty input")
		}
		if len(parser.ReadBytes(3)) != 3 {
			t.Error("ReadBytes should return requested size even for empty input")
		}
	})
}
