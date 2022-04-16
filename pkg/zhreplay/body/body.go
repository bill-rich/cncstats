package body

import (
	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/iniparse"
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
	OrderCode    int
	OrderName    string
	PlayerID     int // Starts at 2 for humans
	PlayerName   string
	UniqueOrders int
	Details      interface{}
	Args         []*Arg
}

var CommandType map[int]string = map[int]string{
	1047: "CreateUnit",
	1048: "CancelUnit",
	1049: "BuildObject",
	1051: "CancelBuild",
	1052: "Sell",
}

func ParseBody(bp *bitparse.BitParser, playerList []string, objectStore *iniparse.ObjectStore) []*BodyChunk {
	body := []*BodyChunk{}

	for {
		chunk := BodyChunk{
			TimeCode:     bp.ReadUInt32(),
			OrderCode:    bp.ReadUInt32(),
			PlayerID:     bp.ReadUInt32(),
			UniqueOrders: bp.ReadUInt8(),
			Args:         []*Arg{},
		}
		if chunk.PlayerID >= 2 {
			chunk.PlayerName = playerList[chunk.PlayerID-2]
		}
		chunk.OrderName = CommandType[chunk.OrderCode]
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
				argType.Args = append(argType.Args, convertArg(bp, argType.Type))
			}
		}
		chunk.addExtraData(objectStore)
		if chunk.TimeCode == 0 && chunk.OrderCode == 0 && chunk.PlayerID == 0 {
			break
		}
		body = append(body, &chunk)
	}
	return body
}

func (c *BodyChunk) addExtraData(objectStore *iniparse.ObjectStore) {
	switch c.OrderCode {
	case 1047: // Create Unit
		object := objectStore.GetObject(c.Args[0].Args[0].(int))
		c.Details = map[string]interface{}{
			"object": object.Name,
			"cost":   object.Cost,
		}
	case 1049: // Build
		object := objectStore.GetObject(c.Args[0].Args[0].(int))
		c.Details = map[string]interface{}{
			"object": object.Name,
			"cost":   object.Cost,
		}
	}
}
