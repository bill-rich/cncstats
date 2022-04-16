package bitparse

import (
	"bytes"
	"encoding/binary"
	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/sirupsen/logrus"
	"io"
	"math"
	"math/big"
)

type BitParser struct {
	Source      io.Reader
	ObjectStore *iniparse.ObjectStore
}

func (bp *BitParser) ReadBytes(size int) []byte {
	bytesIn := make([]byte, size)
	n, err := bp.Source.Read(bytesIn)
	if n < size || err != nil {
		logrus.WithError(err).Debugf("could not read %d bytes", size)
	}
	return bytesIn
}

func (bp *BitParser) ReadString(size int) string {
	bytesIn := make([]byte, size)
	n, err := bp.Source.Read(bytesIn)
	if n < size || err != nil {
		logrus.WithError(err).Debugf("could not read %d bytes for string", size)
	}
	return string(bytesIn)
}

func (bp *BitParser) ReadUInt32() int {
	return bp.ReadUInt(4)
}

func (bp *BitParser) ReadUInt16() int {
	return bp.ReadUInt(2)
}

func (bp *BitParser) ReadUInt8() int {
	return bp.ReadUInt(1)
}

// TODO: Find examples of how this should work so tests can be written.
func (bp *BitParser) ReadFloat() float32 {
	bytesIn := make([]byte, 4)
	n, err := bp.Source.Read(bytesIn)
	if n < 4 || err != nil {
		logrus.WithError(err).Debug("failed to read float")
		return 0
	}
	bits := binary.LittleEndian.Uint32(bytesIn)
	float := math.Float32frombits(bits)
	return float

}

// TODO: Find examples of how this should work so tests can be written.
func (bp *BitParser) ReadBool() bool {
	byteIn := bp.ReadBytes(1)
	if byteIn[0] == byte(0) {
		return false
	}
	return true
}

func (bp *BitParser) ReadUInt(byteCount int) int {
	bytesIn := make([]byte, byteCount)
	n, err := bp.Source.Read(bytesIn)
	if n < byteCount || err != nil {
		logrus.WithError(err).Debug("failed to read int")
		return 0
	}
	return int(big.NewInt(0).SetBytes(makeLittleEndian(bytesIn)).Uint64())
}

func (bp *BitParser) ReadInt(byteCount int) uint32 {
	bytesIn := make([]byte, byteCount)
	n, err := bp.Source.Read(bytesIn)
	if n < byteCount || err != nil {
		logrus.WithError(err).Debug("failed to read int")
		return 0
	}
	return binary.LittleEndian.Uint32(bytesIn)
	//return int(big.NewInt(0).SetBytes(makeLittleEndian(bytesIn)).Int64())
}

func makeLittleEndian(bytesIn []byte) []byte {
	newBytes := make([]byte, len(bytesIn))
	for i, bi := range bytesIn {
		newBytes[len(bytesIn)-i-1] = bi
	}
	return newBytes
}

func (bp *BitParser) ReadNullTermString(encoding string) string {
	var size, start, end int
	switch encoding {
	case "utf16":
		size = 2
		start = 0
		end = 1
	case "utf8":
		size = 1
		start = 0
		end = 1
	default:
		logrus.Debugf("unexpected encoding type: %s", encoding)
	}

	buffer := bytes.Buffer{}
	for {
		bytesIn := make([]byte, size)
		n, err := bp.Source.Read(bytesIn)

		if isNull(bytesIn) {
			logrus.Trace("reached end of null terminated string\n")
			break
		}

		buffer.Write(bytesIn[start:end])

		if n < size || err != nil {
			logrus.WithError(err).Warnf("unexpected end to null terminated string: %s", buffer.String())
			break
		}
	}
	return buffer.String()
}

func isNull(bytesIn []byte) bool {
	allNull := true
	for _, b := range bytesIn {
		if b != byte(0) {
			allNull = false
			break
		}
	}
	return allNull
}
