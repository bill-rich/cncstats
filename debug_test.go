package main

import (
	"bytes"
	"fmt"

	"github.com/bill-rich/cncstats/pkg/bitparse"
)

func main() {
	// Test the byte values that should represent 1047
	input := []byte{39, 4, 0, 0} // This should be 1047 in little endian

	parser := &bitparse.BitParser{
		Source: bytes.NewReader(input),
	}

	result := parser.ReadUInt32()
	fmt.Printf("Expected: 1047, Got: %d\n", result)

	// Let's also test what 39, 4, 0, 0 actually represents
	// 39 + 4*256 = 39 + 1024 = 1063
	fmt.Printf("Manual calculation: 39 + 4*256 = %d\n", 39+4*256)

	// What should 1047 be in little endian?
	// 1047 = 0x0417 = 23 + 4*256 = 23 + 1024 = 1047
	fmt.Printf("1047 in little endian should be: 23, 4, 0, 0\n")
	fmt.Printf("23 + 4*256 = %d\n", 23+4*256)
}
