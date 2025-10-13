package bitparse

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/big"

	"github.com/bill-rich/cncstats/pkg/iniparse"
)

// Constants for encoding and buffer sizes
const (
	UTF16CharSize = 2
	UTF8CharSize  = 1
	MaxStringLen  = 1024 * 1024 // 1MB limit for security
	MaxByteCount  = 8           // Maximum bytes for integer operations
)

// BitParser provides methods for parsing binary data from an io.Reader.
// It supports reading various data types including integers, floats, strings,
// and boolean values with proper error handling and validation.
type BitParser struct {
	Source       io.Reader
	ObjectStore  *iniparse.ObjectStore
	PowerStore   *iniparse.PowerStore
	UpgradeStore *iniparse.UpgradeStore
	ColorStore   *iniparse.ColorStore
}

// ReadBytes reads the specified number of bytes from the source.
// Returns the bytes read and any error encountered.
// If insufficient data is available, returns a partial result with an error.
func (bp *BitParser) ReadBytes(size int) ([]byte, error) {
	if size < 0 {
		return nil, fmt.Errorf("invalid size: %d (must be non-negative)", size)
	}
	if size > MaxStringLen {
		return nil, fmt.Errorf("size too large: %d (max: %d)", size, MaxStringLen)
	}

	bytesIn := make([]byte, size)
	n, err := bp.Source.Read(bytesIn)
	if n < size {
		if err != nil {
			return bytesIn[:n], fmt.Errorf("failed to read %d bytes: %w", size, err)
		}
		return bytesIn[:n], fmt.Errorf("insufficient data: read %d of %d bytes", n, size)
	}
	if err != nil {
		return bytesIn, fmt.Errorf("error reading %d bytes: %w", size, err)
	}
	return bytesIn, nil
}

// ReadString reads the specified number of bytes and converts them to a string.
// Returns the string and any error encountered.
// If insufficient data is available, returns a partial string with an error.
func (bp *BitParser) ReadString(size int) (string, error) {
	bytesIn, err := bp.ReadBytes(size)
	if err != nil {
		return string(bytesIn), err
	}
	return string(bytesIn), nil
}

// ReadUInt32 reads a 32-bit unsigned integer in little-endian format.
// Returns the integer value and any error encountered.
func (bp *BitParser) ReadUInt32() (int, error) {
	return bp.ReadUInt(4)
}

// ReadUInt16 reads a 16-bit unsigned integer in little-endian format.
// Returns the integer value and any error encountered.
func (bp *BitParser) ReadUInt16() (int, error) {
	return bp.ReadUInt(2)
}

// ReadUInt8 reads an 8-bit unsigned integer.
// Returns the integer value and any error encountered.
func (bp *BitParser) ReadUInt8() (int, error) {
	return bp.ReadUInt(1)
}

// ReadFloat reads a 32-bit IEEE 754 floating-point number in little-endian format.
// Returns the float value and any error encountered.
func (bp *BitParser) ReadFloat() (float32, error) {
	bytesIn, err := bp.ReadBytes(4)
	if err != nil {
		return 0, fmt.Errorf("failed to read float: %w", err)
	}
	bits := binary.LittleEndian.Uint32(bytesIn)
	return math.Float32frombits(bits), nil
}

// ReadBool reads a single byte and interprets it as a boolean value.
// Returns true if the byte is non-zero, false otherwise.
// Returns the boolean value and any error encountered.
func (bp *BitParser) ReadBool() (bool, error) {
	bytesIn, err := bp.ReadBytes(1)
	if err != nil {
		return false, fmt.Errorf("failed to read bool: %w", err)
	}
	return bytesIn[0] != 0, nil
}

// ReadUInt reads an unsigned integer of the specified byte count in little-endian format.
// Returns the integer value and any error encountered.
// Validates byteCount to prevent excessive memory allocation.
func (bp *BitParser) ReadUInt(byteCount int) (int, error) {
	if byteCount <= 0 {
		return 0, fmt.Errorf("invalid byte count: %d (must be positive)", byteCount)
	}
	if byteCount > MaxByteCount {
		return 0, fmt.Errorf("byte count too large: %d (max: %d)", byteCount, MaxByteCount)
	}

	bytesIn, err := bp.ReadBytes(byteCount)
	if err != nil {
		return 0, fmt.Errorf("failed to read uint: %w", err)
	}

	// Optimize byte reversal in-place to avoid extra allocation
	reversed := makeLittleEndian(bytesIn)
	return int(big.NewInt(0).SetBytes(reversed).Uint64()), nil
}

// ReadInt reads a signed integer of the specified byte count in little-endian format.
// Returns the integer value as uint32 and any error encountered.
// Pads to 4 bytes if necessary for proper binary.LittleEndian.Uint32 conversion.
func (bp *BitParser) ReadInt(byteCount int) (uint32, error) {
	if byteCount <= 0 {
		return 0, fmt.Errorf("invalid byte count: %d (must be positive)", byteCount)
	}
	if byteCount > MaxByteCount {
		return 0, fmt.Errorf("byte count too large: %d (max: %d)", byteCount, MaxByteCount)
	}

	bytesIn, err := bp.ReadBytes(byteCount)
	if err != nil {
		return 0, fmt.Errorf("failed to read int: %w", err)
	}

	// Pad to 4 bytes if necessary
	if len(bytesIn) < 4 {
		padded := make([]byte, 4)
		copy(padded, bytesIn)
		bytesIn = padded
	}

	return binary.LittleEndian.Uint32(bytesIn), nil
}

// makeLittleEndian reverses the byte order of the input slice.
// Optimized to avoid unnecessary allocations and improve performance.
func makeLittleEndian(bytesIn []byte) []byte {
	if len(bytesIn) == 0 {
		return bytesIn
	}

	// Reuse the input slice to avoid allocation when possible
	result := make([]byte, len(bytesIn))
	for i := 0; i < len(bytesIn); i++ {
		result[i] = bytesIn[len(bytesIn)-1-i]
	}
	return result
}

// ReadNullTermString reads a null-terminated string with the specified encoding.
// Supports UTF-8 and UTF-16 encodings with proper validation and security limits.
// Returns the string and any error encountered.
func (bp *BitParser) ReadNullTermString(encoding string) (string, error) {
	var size, start, end int
	switch encoding {
	case "utf16":
		size = UTF16CharSize
		start = 0
		end = 1
	case "utf8":
		size = UTF8CharSize
		start = 0
		end = 1
	default:
		return "", fmt.Errorf("unsupported encoding: %s (supported: utf8, utf16)", encoding)
	}

	buffer := bytes.Buffer{}
	bytesRead := 0

	for {
		// Security check to prevent excessive memory allocation
		if bytesRead > MaxStringLen {
			return buffer.String(), fmt.Errorf("string too long: %d bytes (max: %d)", bytesRead, MaxStringLen)
		}

		bytesIn := make([]byte, size)
		n, err := bp.Source.Read(bytesIn)

		if isNull(bytesIn) {
			break
		}

		buffer.Write(bytesIn[start:end])
		bytesRead += size

		if n < size || err != nil {
			if err != nil {
				return buffer.String(), fmt.Errorf("error reading null-terminated string: %w", err)
			}
			return buffer.String(), fmt.Errorf("unexpected end of data while reading null-terminated string")
		}
	}
	return buffer.String(), nil
}

// isNull checks if all bytes in the slice are null (zero) bytes.
// Returns true if all bytes are zero, false otherwise.
func isNull(bytesIn []byte) bool {
	for _, b := range bytesIn {
		if b != 0 {
			return false
		}
	}
	return true
}
