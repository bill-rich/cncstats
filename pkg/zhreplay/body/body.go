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
		val, err := bp.ReadUInt32()
		if err != nil {
			return 0
		}
		return val
	case ArgFloat:
		val, err := bp.ReadFloat()
		if err != nil {
			return float32(0)
		}
		return val
	case ArgBool:
		val, err := bp.ReadBool()
		if err != nil {
			return false
		}
		return val
	case ArgObjectID:
		val, err := bp.ReadUInt32()
		if err != nil {
			return 0
		}
		return val
	case ArgUnknown4:
		val, err := bp.ReadUInt32()
		if err != nil {
			return 0
		}
		return val
	case ArgUnknown5:
		return []byte{}
	case ArgPosition:
		x, err1 := bp.ReadFloat()
		y, err2 := bp.ReadFloat()
		z, err3 := bp.ReadFloat()
		if err1 != nil || err2 != nil || err3 != nil {
			return Position{X: float32(0), Y: float32(0), Z: float32(0)}
		}
		return Position{X: x, Y: y, Z: z}
	case ArgScreenPosition:
		x, err1 := bp.ReadUInt32()
		y, err2 := bp.ReadUInt32()
		if err1 != nil || err2 != nil {
			return Position{X: 0, Y: 0}
		}
		return Position{X: x, Y: y}
	case ArgScreenRectangle:
		x1, err1 := bp.ReadUInt32()
		y1, err2 := bp.ReadUInt32()
		x2, err3 := bp.ReadUInt32()
		y2, err4 := bp.ReadUInt32()
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			return Rectangle{
				Position{X: 0, Y: 0},
				Position{X: 0, Y: 0},
			}
		}
		return Rectangle{
			Position{X: x1, Y: y1},
			Position{X: x2, Y: y2},
		}
	case ArgUnknown9:
		val, err := bp.ReadBytes(16)
		if err != nil {
			return make([]byte, 16)
		}
		return val
	case ArgUnknown10:
		val, err := bp.ReadUInt16()
		if err != nil {
			return 0
		}
		return val
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

type BodyChunkEasyUnmarshall struct {
	TimeCode          int
	OrderCode         int
	OrderName         string
	PlayerID          int // Starts at 2 for humans
	PlayerName        string
	NumberOfArguments int
	Details           GeneralDetail
	ArgMetadata       []*ArgMetadata
	Arguments         []interface{}
}

type GeneralDetail struct {
	Cost int
	Name string
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
	1037: "DetonateNow",                   // For bomb truck
	1038: "FlamewallRocketPodContaminate", // Flamewall for flametank, rocket pods for comanche, contaminate for tractor
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
	1053: "EvacSingleUnit",
	1054: "EvacAll",
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
	1069: "AttackMove",
	1072: "Guard",
	1074: "Stop",
	1075: "Scatter",
	1076: "HackInternet",
	1078: "ToggleOvercharge",
	1079: "ToggleUnitMode", // Ranger: machinegun(0)/flashbang(1), Demotrap:detonate(0),proxy(1),manual(2), Scud:explosive(0)/anthrax(1)
	1087: "Unknown1087",    // Takes one position arg
	1092: "SetCameraPosition",
	1093: "Surrender",
	1095: "Checksum",
	1097: "DeclareUserId", // Seems to be the case.
}

func ParseBody(bp *bitparse.BitParser, playerList []*object.PlayerSummary, objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore) []*BodyChunk {
	body := []*BodyChunk{}

	for {
		// Read basic chunk data with error handling
		timeCode, err := bp.ReadUInt32()
		if err != nil {
			break // End of data or error
		}

		orderCode, err := bp.ReadUInt32()
		if err != nil {
			break
		}

		playerID, err := bp.ReadUInt32()
		if err != nil {
			break
		}

		numberOfArguments, err := bp.ReadUInt8()
		if err != nil {
			break
		}

		chunk := BodyChunk{
			TimeCode:          timeCode,
			OrderCode:         orderCode,
			PlayerID:          playerID,
			NumberOfArguments: numberOfArguments,
			ArgMetadata:       []*ArgMetadata{},
			Arguments:         []interface{}{},
		}
		chunk.OrderName = CommandType[chunk.OrderCode]

		// Read argument metadata
		for i := 0; i < chunk.NumberOfArguments; i++ {
			argType, err1 := bp.ReadUInt8()
			argCount, err2 := bp.ReadUInt8()
			if err1 != nil || err2 != nil {
				break
			}
			argCountData := &ArgMetadata{
				Type:  argType,
				Count: argCount,
			}
			chunk.ArgMetadata = append(chunk.ArgMetadata, argCountData)
		}

		// Read arguments
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
		newObject, err := objectStore.GetObject(c.Arguments[0].(int))
		if err == nil && newObject != nil {
			c.Details = &object.Unit{
				Name: newObject.Name,
				Cost: newObject.Cost,
			}
		}
	case 1049: // Build
		newObject, err := objectStore.GetObject(c.Arguments[0].(int))
		if err == nil && newObject != nil {
			c.Details = &object.Building{
				Name: newObject.Name,
				Cost: newObject.Cost,
			}
		}
	case 1040: // SpecialPower
		newObject, err := powerStore.GetObject(c.Arguments[0].(int))
		if err == nil && newObject != nil {
			c.Details = &object.Power{
				Name: newObject.Name,
			}
		}
	case 1041: // SpecialPower
		newObject, err := powerStore.GetObject(c.Arguments[0].(int))
		if err == nil && newObject != nil {
			c.Details = &object.Power{
				Name: newObject.Name,
			}
		}
	case 1042: // SpecialPower
		newObject, err := powerStore.GetObject(c.Arguments[0].(int))
		if err == nil && newObject != nil {
			c.Details = &object.Power{
				Name: newObject.Name,
			}
		}
	case 1045: // Upgrades
		newObject, err := upgradeStore.GetObject(c.Arguments[1].(int))
		if err != nil || newObject == nil {
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
