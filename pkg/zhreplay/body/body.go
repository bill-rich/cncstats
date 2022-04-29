package body

import (
	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
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

type ArgMetadata struct {
	Type  int
	Count int
}

type BodyChunk struct {
	TimeCode          int
	OrderCode         int
	OrderName         string
	PlayerID          int // Starts at 2 for humans
	PlayerName        string
	NumberOfArguments int
	Details           object.Object
	ArgMetadata       []*ArgMetadata
	Arguments         []interface{}
}

var CommandType map[int]string = map[int]string{
	27:   "EndReplay",
	1001: "SetSelection",
	1002: "SelectAll", // Pretty sure this is "e", the bool is everywhere or just on screen.
	1003: "ClearSelection",
	1006: "CreateGroup0",
	1007: "CreateGroup1",
	1008: "CreateGroup2",
	1009: "CreateGroup3",
	1010: "CreateGroup4",
	1011: "CreateGroup5",
	1012: "CreateGroup6",
	1013: "CreateGroup7",
	1014: "CreateGroup8",
	1015: "CreateGroup9",
	1016: "SelectGroup0",
	1017: "SelectGroup1",
	1018: "SelectGroup2",
	1019: "SelectGroup3",
	1020: "SelectGroup4",
	1021: "SelectGroup5",
	1022: "SelectGroup6",
	1023: "SelectGroup7",
	1024: "SelectGroup8",
	1025: "SelectGroup9",
	1038: "Unknown1038_Flamewall", // Has a position arg. Seems to only be available to China.
	1040: "SpecialPower",
	1041: "SpecialPowerAtLocation",
	1042: "SpecialPowerAtObject",
	1043: "SetRallyPoint",
	1044: "PurchaseScience",
	1045: "BuildUpgrade",
	1046: "CancelUpgrade",
	1047: "CreateUnit",
	1048: "CancelUnit",
	1049: "BuildObject",
	1051: "CancelBuild",
	1052: "Sell",
	1053: "Unknown1053", // Takes one int arg
	1054: "Unknown1054", // Only used by Jared (China). No args
	1058: "SelectBox",
	1059: "AttackObject",
	1060: "ForceAttackObject",
	1061: "ForceAttackGround",
	1062: "Unknown1062", // 555 or 554 for USAs, 972 for china
	1064: "Unknown1064", // Arg can be 628 or 630. Fairly uncommon (~3 in one game checked). Maybe attack move?
	1065: "ResumeBuild",
	1066: "Enter",
	1067: "Unknown1067", // Something only USA has. Arg can be 402 or 409
	1068: "MoveTo",
	1069: "Unknown1069", // Only used by Brendan (USA). 1 position arg
	1072: "Unknown1072", // Mostly used by Brendan and occasionally Bill (As USA). Maybe guard?
	1074: "StopMoving",
	1076: "Unknown1076_HackInternet", // No args
	1078: "ToggleOvercharge",
	1079: "Unknown1079",
	1087: "Unknown1087", // Takes one position arg
	1092: "SetCameraPosition",
	1093: "Surrender",
	1095: "Checksum",
	1097: "DeclareUserId", // Seems to be the case.
}

func ParseBody(bp *bitparse.BitParser, playerList []*object.PlayerSummary, objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore) []*BodyChunk {
	body := []*BodyChunk{}

	for {
		chunk := BodyChunk{
			TimeCode:          bp.ReadUInt32(),
			OrderCode:         bp.ReadUInt32(),
			PlayerID:          bp.ReadUInt32(),
			NumberOfArguments: bp.ReadUInt8(),
			ArgMetadata:       []*ArgMetadata{},
			Arguments:         []interface{}{},
		}
		chunk.OrderName = CommandType[chunk.OrderCode]
		for i := 0; i < chunk.NumberOfArguments; i++ {
			argCount := &ArgMetadata{
				Type:  bp.ReadUInt8(),
				Count: bp.ReadUInt8(),
			}
			chunk.ArgMetadata = append(chunk.ArgMetadata, argCount)
		}
		for _, argData := range chunk.ArgMetadata {
			for i := 0; i < argData.Count; i++ {
				chunk.Arguments = append(chunk.Arguments, convertArg(bp, argData.Type))
			}
		}
		chunk.addExtraData(objectStore, powerStore, upgradeStore)
		if chunk.TimeCode == 0 && chunk.OrderCode == 0 && chunk.PlayerID == 0 {
			break
		}
		body = append(body, &chunk)
	}
	return body
}

func (c *BodyChunk) addExtraData(objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore) {
	switch c.OrderCode {
	case 1047: // Create Unit
		newObject := objectStore.GetObject(c.Arguments[0].(int))
		c.Details = &object.Unit{
			Name: newObject.Name,
			Cost: newObject.Cost,
		}
	case 1049: // Build
		newObject := objectStore.GetObject(c.Arguments[0].(int))
		c.Details = &object.Building{
			Name: newObject.Name,
			Cost: newObject.Cost,
		}
	case 1040: // SpecialPower
		newObject := powerStore.GetObject(c.Arguments[0].(int))
		c.Details = &object.Power{
			Name: newObject.Name,
		}
	case 1041: // SpecialPower
		newObject := powerStore.GetObject(c.Arguments[0].(int))
		c.Details = &object.Power{
			Name: newObject.Name,
		}
	case 1042: // SpecialPower
		newObject := powerStore.GetObject(c.Arguments[0].(int))
		c.Details = &object.Power{
			Name: newObject.Name,
		}
	case 1045: // Upgrades
		newObject := upgradeStore.GetObject(c.Arguments[1].(int))
		if newObject == nil {
			c.Details = &object.Upgrade{
				Name: "dummy",
			}
		} else {
			c.Details = &object.Upgrade{
				Name: newObject.Name,
				Cost: newObject.Cost,
			}
		}
	}
}
