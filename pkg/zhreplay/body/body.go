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

// argSize maps argument types to their expected byte sizes
// Note: Some sizes may be incorrect and need validation
var argSize = map[int]int{
	ArgInt:             4,
	ArgFloat:           4,
	ArgBool:            1,
	ArgObjectID:        4,
	ArgUnknown4:        4,
	ArgUnknown5:        0,
	ArgPosition:        12, // TODO: Validate this size - may be incorrect
	ArgScreenPosition:  8,
	ArgScreenRectangle: 16,
	ArgUnknown9:        16,
	ArgUnknown10:       4,
}

// ValidateArgType checks if the argument type is valid
func ValidateArgType(argType int) bool {
	return argType >= ArgInt && argType <= ArgUnknown10
}

// ValidateArgCount checks if the argument count is within reasonable bounds
func ValidateArgCount(count int) bool {
	return count >= 0 && count <= 50 // Reasonable upper limit
}

// ConvertArg safely converts binary data to appropriate types based on argument type
func ConvertArg(bp *bitparse.BitParser, at int) interface{} {
	// Validate argument type
	if at < ArgInt || at > ArgUnknown10 {
		return nil
	}

	switch at {
	case ArgInt:
		val, err := bp.ReadUInt32()
		if err != nil {
			return uint32(0)
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
			return uint32(0)
		}
		return val
	case ArgUnknown4:
		val, err := bp.ReadUInt32()
		if err != nil {
			return uint32(0)
		}
		return val
	case ArgUnknown5:
		return []byte{}
	case ArgPosition:
		x, err1 := bp.ReadFloat()
		y, err2 := bp.ReadFloat()
		z, err3 := bp.ReadFloat()
		if err1 != nil || err2 != nil || err3 != nil {
			return Position3D{X: 0, Y: 0, Z: 0}
		}
		return Position3D{X: x, Y: y, Z: z}
	case ArgScreenPosition:
		x, err1 := bp.ReadUInt32()
		y, err2 := bp.ReadUInt32()
		if err1 != nil || err2 != nil {
			return ScreenPosition{X: 0, Y: 0}
		}
		return ScreenPosition{X: uint32(x), Y: uint32(y)}
	case ArgScreenRectangle:
		x1, err1 := bp.ReadUInt32()
		y1, err2 := bp.ReadUInt32()
		x2, err3 := bp.ReadUInt32()
		y2, err4 := bp.ReadUInt32()
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			return ScreenRectangle{
				ScreenPosition{X: 0, Y: 0},
				ScreenPosition{X: 0, Y: 0},
			}
		}
		return ScreenRectangle{
			ScreenPosition{X: uint32(x1), Y: uint32(y1)},
			ScreenPosition{X: uint32(x2), Y: uint32(y2)},
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
			return uint16(0)
		}
		return val
	default:
		return nil
	}
}

// Position3D represents a 3D position with float32 coordinates
type Position3D struct {
	X, Y, Z float32
}

// ScreenPosition represents a 2D screen position with uint32 coordinates
type ScreenPosition struct {
	X, Y uint32
}

// Position is kept for backward compatibility but should be replaced with specific types
type Position struct {
	X interface{}
	Y interface{}
	Z interface{}
}

// ScreenRectangle represents a rectangle on screen with two screen positions
type ScreenRectangle [2]ScreenPosition

// Rectangle is kept for backward compatibility
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

var PassiveCommands = map[int]bool{
	27:   true, // EndReplay
	1001: true, // SetSelection
	1002: true, // SelectAll
	1003: true, // ClearSelection
	1016: true, // SelectGroup0
	1017: true, // SelectGroup1
	1018: true, // SelectGroup2
	1019: true, // SelectGroup3
	1020: true, // SelectGroup4
	1021: true, // SelectGroup5
	1022: true, // SelectGroup6
	1023: true, // SelectGroup7
	1024: true, // SelectGroup8
	1025: true, // SelectGroup9
	1048: true, // CancelUnit
	1051: true, // CancelBuild
	1052: true, // Sell; Not really passive, but doesn't imply winning
	1058: true, // SelectBox
	1092: true, // SetCameraPosition
	1095: true, // Checksum
	2000: true, // MoneyValueChange
	2001: true, // MoneyEarnedChange
	2002: true, // UnitsBuiltChange
	2003: true, // UnitsLostChange
	2004: true, // BuildingsBuiltChange
	2005: true, // BuildingsLostChange
	2006: true, // BuildingsKilledChange
	2007: true, // UnitsKilledChange
	2008: true, // GeneralsPointsTotalChange
	2009: true, // GeneralsPointsUsedChange
	2010: true, // RadarsBuiltChange
	2011: true, // SearchAndDestroyChange
	2012: true, // HoldTheLineChange
	2013: true, // BombardmentChange
	2014: true, // XPChange
	2015: true, // XPLevelChange
	2016: true, // TechBuildingsCapturedChange
	2017: true, // FactionBuildingsCapturedChange
	2018: true, // PowerTotalChange
	2019: true, // PowerUsedChange
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
	1097: "DeclareUserId",    // Seems to be the case.
	2000: "MoneyValueChange", // Money change events sent by client
	2001: "MoneyEarnedChange",
	2002: "UnitsBuiltChange",
	2003: "UnitsLostChange",
	2004: "BuildingsBuiltChange",
	2005: "BuildingsLostChange",
	2006: "BuildingsKilledChange",
	2007: "UnitsKilledChange",
	2008: "GeneralsPointsTotalChange",
	2009: "GeneralsPointsUsedChange",
	2010: "RadarsBuiltChange",
	2011: "SearchAndDestroyChange",
	2012: "HoldTheLineChange",
	2013: "BombardmentChange",
	2014: "XPChange",
	2015: "XPLevelChange",
	2016: "TechBuildingsCapturedChange",
	2017: "FactionBuildingsCapturedChange",
	2018: "PowerTotalChange",
	2019: "PowerUsedChange",
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

		// Validate reasonable bounds for numberOfArguments
		if !ValidateArgCount(int(numberOfArguments)) {
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
			// Validate argument type and count
			if !ValidateArgType(int(argType)) || !ValidateArgCount(int(argCount)) {
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
				chunk.Arguments = append(chunk.Arguments, ConvertArg(bp, argData.Type))
			}
		}

		chunk.AddExtraData(objectStore, powerStore, upgradeStore)
		if chunk.TimeCode == 0 && chunk.OrderCode == 0 && chunk.PlayerID == 0 {
			break
		}
		body = append(body, &chunk)
	}
	return body
}

func (c *BodyChunk) AddExtraData(objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore) {
	if len(c.Arguments) == 0 {
		return
	}

	switch c.OrderCode {
	case 1047: // Create Unit
		c.setUnitDetails(objectStore)
	case 1049: // Build
		c.setBuildingDetails(objectStore)
	case 1040, 1041, 1042: // SpecialPower variants
		c.setPowerDetails(powerStore)
	case 1045: // Upgrades
		c.setUpgradeDetails(upgradeStore)
	}
}

// setUnitDetails safely sets unit details from object store
func (c *BodyChunk) setUnitDetails(objectStore *iniparse.ObjectStore) {
	if objectStore == nil {
		return
	}
	if arg, ok := c.Arguments[0].(int); ok {
		if newObject, err := objectStore.GetObject(arg); err == nil && newObject != nil {
			c.Details = &object.Unit{
				Name: newObject.Name,
				Cost: newObject.Cost,
			}
		}
	}
}

// setBuildingDetails safely sets building details from object store
func (c *BodyChunk) setBuildingDetails(objectStore *iniparse.ObjectStore) {
	if objectStore == nil {
		return
	}
	if arg, ok := c.Arguments[0].(int); ok {
		if newObject, err := objectStore.GetObject(arg); err == nil && newObject != nil {
			c.Details = &object.Building{
				Name: newObject.Name,
				Cost: newObject.Cost,
			}
		}
	}
}

// setPowerDetails safely sets power details from power store
func (c *BodyChunk) setPowerDetails(powerStore *iniparse.PowerStore) {
	if powerStore == nil {
		return
	}
	if arg, ok := c.Arguments[0].(int); ok {
		if newObject, err := powerStore.GetObject(arg); err == nil && newObject != nil {
			c.Details = &object.Power{
				Name: newObject.Name,
			}
		}
	}
}

// setUpgradeDetails safely sets upgrade details from upgrade store
func (c *BodyChunk) setUpgradeDetails(upgradeStore *iniparse.UpgradeStore) {
	if upgradeStore == nil {
		return
	}
	if len(c.Arguments) < 2 {
		c.Details = &object.Upgrade{Name: "dummy"}
		return
	}

	if arg, ok := c.Arguments[1].(int); ok {
		if newObject, err := upgradeStore.GetObject(arg); err == nil && newObject != nil {
			c.Details = &object.Upgrade{
				Name: newObject.Name,
				Cost: newObject.Cost,
			}
		} else {
			c.Details = &object.Upgrade{Name: "dummy"}
		}
	} else {
		c.Details = &object.Upgrade{Name: "dummy"}
	}
}
