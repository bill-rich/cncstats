package body

import (
	"github.com/bill-rich/cncstats/pkg/bitparse"
)

const (
	ArgInt = iota
	ArgFloat
	ArgBool
	ArgObjectID
	ArgUnknown4
	ArgUnknown5
	ArgPosition
	ArgScreenPosition
	ArgScreenRectangle
	ArgUnknown9
	ArgUnknown10
)

type ArgType int

var argSize = map[int]int{
	ArgInt:             4,
	ArgFloat:           4,
	ArgBool:            1,
	ArgObjectID:        4,
	ArgUnknown4:        4,
	ArgUnknown5:        0,
	ArgPosition:        12, // This is weird. I think its wrong. The numbers are all too high
	ArgScreenPosition:  8,
	ArgScreenRectangle: 16,
	ArgUnknown9:        16,
	ArgUnknown10:       4,
}

func convertArg(bp *bitparse.BitParser, at int) interface{} {
	switch at {
	case ArgInt:
		return bp.ReadUInt32()
	case ArgFloat:
		return bp.ReadFloat()
	case ArgBool:
		return bp.ReadBool()
	case ArgObjectID:
		return bp.ReadUInt32()
	case ArgUnknown4:
		return bp.ReadUInt32()
	case ArgUnknown5:
		return []byte{}
	case ArgPosition:
		return Position{
			X: bp.ReadFloat(),
			Y: bp.ReadFloat(),
			Z: bp.ReadFloat(),
		}
	case ArgScreenPosition:
		return Position{
			X: bp.ReadUInt32(),
			Y: bp.ReadUInt32(),
		}
	case ArgScreenRectangle:
		return Rectangle{
			Position{
				X: bp.ReadUInt32(),
				Y: bp.ReadUInt32(),
			},
			Position{
				X: bp.ReadUInt32(),
				Y: bp.ReadUInt32(),
			},
		}
	case ArgUnknown9:
		return bp.ReadBytes(16)
	case ArgUnknown10:
		return bp.ReadBytes(4)
	default:
		return nil
	}
}

type Position struct {
	X interface{}
	Y interface{}
	Z interface{}
}

type Rectangle [2]Position

type Arg struct {
	Type  int
	Count int
	Args  []interface{}
}

type BodyChunk struct {
	TimeCode     int
	OrderType    int
	Number       int
	UniqueOrders int
	Args         []*Arg
}

func ParseBody(bp *bitparse.BitParser) []*BodyChunk {
	body := []*BodyChunk{}

	for {
		chunk := BodyChunk{
			TimeCode:     bp.ReadUInt32(),
			OrderType:    bp.ReadUInt32(),
			Number:       bp.ReadUInt32(),
			UniqueOrders: bp.ReadUInt8(),
			Args:         []*Arg{},
		}
		for i := 0; i < chunk.UniqueOrders; i++ {
			argCount := &Arg{
				Type:  bp.ReadUInt8(),
				Count: bp.ReadUInt8(),
				Args:  []interface{}{},
			}
			chunk.Args = append(chunk.Args, argCount)
		}
		for _, argType := range chunk.Args {
			for i := 0; i < argType.Count; i++ {
				//newArg := bp.ReadBytes(argSize[argType.Type])
				//argType.Args = append(argType.Args, newArg)
				argType.Args = append(argType.Args, convertArg(bp, argType.Type))
			}
		}
		if chunk.TimeCode == 0 && chunk.OrderType == 0 && chunk.Number == 0 {
			break
		}
		body = append(body, &chunk)
	}
	return body
}
