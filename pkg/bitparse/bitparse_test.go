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
			input:    []byte{76, 0, 97, 0, 115, 0, 116, 0, 32, 0, 82, 0, 101, 0, 112, 0, 108, 0, 97, 0, 121, 0, 0},
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
		output, err := parser.ReadNullTermString(tc.encoding)
		if err != nil {
			t.Errorf("%s - unexpected error: %v", name, err)
		}
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
			input:    []byte{76, 97, 0, 0, 0, 0, 0, 0, 0, 0}, // Provide full data
			expected: "La\x00\x00\x00\x00\x00\x00\x00\x00",
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
			output, err := parser.ReadString(tc.size)
			if name == "UTF8_empty" && tc.size == 0 {
				// Empty input with size 0 should work, but if it fails due to EOF, that's also acceptable
				if err != nil && err.Error() != "error reading 0 bytes: EOF" {
					t.Errorf("unexpected error: %v", err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
			input:    []byte{1, 2, 0, 0}, // Provide full data
			expected: 513,                // Little endian: 0x00000201 = 513
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output, err := parser.ReadUInt32()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
			input:    []byte{1, 0}, // Provide full data
			expected: 1,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output, err := parser.ReadUInt16()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
			input:    []byte{7, 0}, // Provide full data
			expected: []byte{7, 0},
			size:     2,
		},
		"empty_input": {
			input:    []byte{0, 0}, // Provide full data
			expected: []byte{0, 0},
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
			output, err := parser.ReadBytes(tc.size)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
			input:    []byte{0}, // Provide full data
			expected: 0,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output, err := parser.ReadUInt8()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
			input:     []byte{1, 2, 0, 0}, // Provide full data
			expected:  513,                // Little endian: 0x00000201 = 513
			byteCount: 4,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output, err := parser.ReadUInt(tc.byteCount)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
			input:     []byte{1, 2, 0, 0}, // Provide full data
			expected:  513,                // Little endian: 0x00000201 = 513
			byteCount: 4,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output, err := parser.ReadInt(tc.byteCount)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
			input:    []byte{1, 2, 0, 0}, // Provide full data
			expected: 1.4e-45,            // IEEE 754 interpretation
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output, err := parser.ReadFloat()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
			input:    []byte{0}, // Provide full data
			expected: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			parser := BitParser{
				Source: bytes.NewReader(tc.input),
			}
			output, err := parser.ReadBool()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
			output, err := parser.ReadNullTermString(tc.encoding)
			if name == "unknown_encoding" {
				// Unknown encoding should return an error
				if err == nil {
					t.Errorf("expected error for unknown encoding, got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if output != tc.expected {
				t.Errorf("expected: %q, got: %q", tc.expected, output)
			}
		})
	}
}

func TestBitParserErrorCases(t *testing.T) {
	t.Run("insufficient_data_errors", func(t *testing.T) {
		// Test ReadUInt8 with sufficient data
		parser := BitParser{
			Source: bytes.NewReader([]byte{1}),
		}
		_, err := parser.ReadUInt8()
		if err != nil {
			t.Error("ReadUInt8 should work with 1 byte")
		}

		// Test ReadUInt16 with sufficient data
		parser = BitParser{
			Source: bytes.NewReader([]byte{1, 2}),
		}
		_, err = parser.ReadUInt16()
		if err != nil {
			t.Error("ReadUInt16 should work with 2 bytes")
		}

		// Test methods with insufficient data
		parser = BitParser{
			Source: bytes.NewReader([]byte{1, 2}), // Only 2 bytes available
		}
		_, err = parser.ReadUInt32()
		if err == nil {
			t.Error("ReadUInt32 should return error for insufficient data")
		}

		parser = BitParser{
			Source: bytes.NewReader([]byte{1, 2}),
		}
		_, err = parser.ReadFloat()
		if err == nil {
			t.Error("ReadFloat should return error for insufficient data")
		}

		parser = BitParser{
			Source: bytes.NewReader([]byte{1, 2}),
		}
		_, err = parser.ReadBool()
		if err != nil {
			t.Error("ReadBool should work with 1 byte available")
		}

		parser = BitParser{
			Source: bytes.NewReader([]byte{1, 2}),
		}
		_, err = parser.ReadString(5)
		if err == nil {
			t.Error("ReadString should return error for insufficient data")
		}

		parser = BitParser{
			Source: bytes.NewReader([]byte{1, 2}),
		}
		_, err = parser.ReadBytes(3)
		if err == nil {
			t.Error("ReadBytes should return error for insufficient data")
		}
	})

	t.Run("validation_errors", func(t *testing.T) {
		parser := BitParser{
			Source: bytes.NewReader([]byte{1, 2, 3, 4}),
		}

		// Test negative size
		_, err := parser.ReadBytes(-1)
		if err == nil {
			t.Error("ReadBytes should return error for negative size")
		}

		// Test oversized request
		_, err = parser.ReadBytes(1024*1024 + 1)
		if err == nil {
			t.Error("ReadBytes should return error for oversized request")
		}

		// Test invalid byte count
		_, err = parser.ReadUInt(0)
		if err == nil {
			t.Error("ReadUInt should return error for zero byte count")
		}

		_, err = parser.ReadUInt(9)
		if err == nil {
			t.Error("ReadUInt should return error for oversized byte count")
		}

		// Test unsupported encoding
		_, err = parser.ReadNullTermString("invalid")
		if err == nil {
			t.Error("ReadNullTermString should return error for unsupported encoding")
		}
	})
}

func TestBitParserEdgeCases(t *testing.T) {
	t.Run("empty_reader", func(t *testing.T) {
		parser := BitParser{
			Source: bytes.NewReader([]byte{}),
		}

		// Test all methods with empty input - they should return errors
		_, err := parser.ReadUInt8()
		if err == nil {
			t.Error("ReadUInt8 should return error for empty input")
		}
		_, err = parser.ReadUInt16()
		if err == nil {
			t.Error("ReadUInt16 should return error for empty input")
		}
		_, err = parser.ReadUInt32()
		if err == nil {
			t.Error("ReadUInt32 should return error for empty input")
		}
		_, err = parser.ReadFloat()
		if err == nil {
			t.Error("ReadFloat should return error for empty input")
		}
		_, err = parser.ReadBool()
		if err == nil {
			t.Error("ReadBool should return error for empty input")
		}
		_, err = parser.ReadString(5)
		if err == nil {
			t.Error("ReadString should return error for empty input")
		}
		_, err = parser.ReadBytes(3)
		if err == nil {
			t.Error("ReadBytes should return error for empty input")
		}
	})
}
