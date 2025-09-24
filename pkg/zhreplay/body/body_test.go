package body

import (
	"bytes"
	"testing"

	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
)

func TestConvertArg(t *testing.T) {
	type testCase struct {
		name        string
		input       []byte
		argType     int
		expected    interface{}
		description string
	}

	testCases := []testCase{
		{
			name:        "ArgInt",
			input:       []byte{1, 0, 0, 0}, // Little endian 1
			argType:     ArgInt,
			expected:    1,
			description: "Should read 32-bit integer",
		},
		{
			name:        "ArgFloat",
			input:       []byte{0, 0, 128, 63}, // 1.0 in IEEE 754
			argType:     ArgFloat,
			expected:    float32(1.0),
			description: "Should read 32-bit float",
		},
		{
			name:        "ArgBool_true",
			input:       []byte{1},
			argType:     ArgBool,
			expected:    true,
			description: "Should read boolean true",
		},
		{
			name:        "ArgBool_false",
			input:       []byte{0},
			argType:     ArgBool,
			expected:    false,
			description: "Should read boolean false",
		},
		{
			name:        "ArgObjectID",
			input:       []byte{42, 0, 0, 0}, // Little endian 42
			argType:     ArgObjectID,
			expected:    42,
			description: "Should read object ID as 32-bit integer",
		},
		{
			name:        "ArgUnknown4",
			input:       []byte{100, 0, 0, 0}, // Little endian 100
			argType:     ArgUnknown4,
			expected:    100,
			description: "Should read unknown4 as 32-bit integer",
		},
		{
			name:        "ArgUnknown5",
			input:       []byte{},
			argType:     ArgUnknown5,
			expected:    []byte{},
			description: "Should return empty byte slice",
		},
		{
			name:        "ArgPosition",
			input:       []byte{0, 0, 128, 63, 0, 0, 0, 64, 0, 0, 64, 64}, // 1.0, 2.0, 3.0
			argType:     ArgPosition,
			expected:    Position3D{X: float32(1.0), Y: float32(2.0), Z: float32(3.0)},
			description: "Should read position with X, Y, Z coordinates",
		},
		{
			name:        "ArgScreenPosition",
			input:       []byte{100, 0, 0, 0, 200, 0, 0, 0}, // 100, 200
			argType:     ArgScreenPosition,
			expected:    ScreenPosition{X: 100, Y: 200},
			description: "Should read screen position with X, Y coordinates",
		},
		{
			name:    "ArgScreenRectangle",
			input:   []byte{10, 0, 0, 0, 20, 0, 0, 0, 30, 0, 0, 0, 40, 0, 0, 0}, // (10,20) to (30,40)
			argType: ArgScreenRectangle,
			expected: ScreenRectangle{
				ScreenPosition{X: 10, Y: 20},
				ScreenPosition{X: 30, Y: 40},
			},
			description: "Should read screen rectangle with two positions",
		},
		{
			name:        "ArgUnknown9",
			input:       []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			argType:     ArgUnknown9,
			expected:    []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			description: "Should read 16 bytes",
		},
		{
			name:        "ArgUnknown10",
			input:       []byte{255, 0}, // Little endian 255
			argType:     ArgUnknown10,
			expected:    255,
			description: "Should read 16-bit integer",
		},
		{
			name:        "InvalidArgType",
			input:       []byte{1, 2, 3, 4},
			argType:     999,
			expected:    nil,
			description: "Should return nil for invalid argument type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := &bitparse.BitParser{
				Source: bytes.NewReader(tc.input),
			}
			result := convertArg(parser, tc.argType)

			// Handle different comparison types
			switch expected := tc.expected.(type) {
			case Position3D:
				if pos, ok := result.(Position3D); ok {
					if pos.X != expected.X || pos.Y != expected.Y || pos.Z != expected.Z {
						t.Errorf("expected: %+v, got: %+v", expected, pos)
					}
				} else {
					t.Errorf("expected Position3D, got %T", result)
				}
			case ScreenPosition:
				if pos, ok := result.(ScreenPosition); ok {
					if pos.X != expected.X || pos.Y != expected.Y {
						t.Errorf("expected: %+v, got: %+v", expected, pos)
					}
				} else {
					t.Errorf("expected ScreenPosition, got %T", result)
				}
			case ScreenRectangle:
				if rect, ok := result.(ScreenRectangle); ok {
					if len(rect) != len(expected) {
						t.Errorf("expected rectangle length %d, got %d", len(expected), len(rect))
					} else {
						for i, pos := range expected {
							if rect[i].X != pos.X || rect[i].Y != pos.Y {
								t.Errorf("expected rectangle[%d]: %+v, got: %+v", i, pos, rect[i])
							}
						}
					}
				} else {
					t.Errorf("expected ScreenRectangle, got %T", result)
				}
			case Position:
				if pos, ok := result.(Position); ok {
					if pos.X != expected.X || pos.Y != expected.Y || pos.Z != expected.Z {
						t.Errorf("expected: %+v, got: %+v", expected, pos)
					}
				} else {
					t.Errorf("expected Position, got %T", result)
				}
			case Rectangle:
				if rect, ok := result.(Rectangle); ok {
					if len(rect) != len(expected) {
						t.Errorf("expected rectangle length %d, got %d", len(expected), len(rect))
					} else {
						for i, pos := range expected {
							if rect[i].X != pos.X || rect[i].Y != pos.Y || rect[i].Z != pos.Z {
								t.Errorf("expected rectangle[%d]: %+v, got: %+v", i, pos, rect[i])
							}
						}
					}
				} else {
					t.Errorf("expected Rectangle, got %T", result)
				}
			case []byte:
				if resultBytes, ok := result.([]byte); ok {
					if !bytes.Equal(resultBytes, expected) {
						t.Errorf("expected: %v, got: %v", expected, resultBytes)
					}
				} else {
					t.Errorf("expected []byte, got %T", result)
				}
			default:
				if result != expected {
					t.Errorf("expected: %v, got: %v", expected, result)
				}
			}
		})
	}
}

func TestArgSize(t *testing.T) {
	expectedSizes := map[int]int{
		ArgInt:             4,
		ArgFloat:           4,
		ArgBool:            1,
		ArgObjectID:        4,
		ArgUnknown4:        4,
		ArgUnknown5:        0,
		ArgPosition:        12,
		ArgScreenPosition:  8,
		ArgScreenRectangle: 16,
		ArgUnknown9:        16,
		ArgUnknown10:       4,
	}

	for argType, expectedSize := range expectedSizes {
		if size, exists := argSize[argType]; !exists {
			t.Errorf("argSize missing for ArgType %d", argType)
		} else if size != expectedSize {
			t.Errorf("argSize[%d] expected %d, got %d", argType, expectedSize, size)
		}
	}
}

func TestCommandType(t *testing.T) {
	// Test some key command types
	expectedCommands := map[int]string{
		27:   "EndReplay",
		1001: "SetSelection",
		1002: "SelectAll",
		1040: "SpecialPower",
		1047: "CreateUnit",
		1049: "BuildObject",
		1051: "CancelBuild",
		1074: "Stop",
		1093: "Surrender",
	}

	for code, expectedName := range expectedCommands {
		if name, exists := CommandType[code]; !exists {
			t.Errorf("CommandType missing for code %d", code)
		} else if name != expectedName {
			t.Errorf("CommandType[%d] expected %s, got %s", code, expectedName, name)
		}
	}
}

func TestPosition(t *testing.T) {
	pos := Position{
		X: 10,
		Y: 20,
		Z: 30,
	}

	if pos.X != 10 || pos.Y != 20 || pos.Z != 30 {
		t.Errorf("Position not initialized correctly: %+v", pos)
	}
}

func TestRectangle(t *testing.T) {
	rect := Rectangle{
		Position{X: 0, Y: 0},
		Position{X: 100, Y: 100},
	}

	if len(rect) != 2 {
		t.Errorf("Rectangle should have 2 positions, got %d", len(rect))
	}

	if rect[0].X != 0 || rect[0].Y != 0 {
		t.Errorf("First position incorrect: %+v", rect[0])
	}

	if rect[1].X != 100 || rect[1].Y != 100 {
		t.Errorf("Second position incorrect: %+v", rect[1])
	}
}

func TestArgMetadata(t *testing.T) {
	metadata := &ArgMetadata{
		Type:  ArgInt,
		Count: 5,
	}

	if metadata.Type != ArgInt {
		t.Errorf("expected Type %d, got %d", ArgInt, metadata.Type)
	}

	if metadata.Count != 5 {
		t.Errorf("expected Count %d, got %d", 5, metadata.Count)
	}
}

func TestBodyChunk(t *testing.T) {
	chunk := BodyChunk{
		TimeCode:          1000,
		OrderCode:         1047,
		OrderName:         "CreateUnit",
		PlayerID:          2,
		PlayerName:        "Player1",
		NumberOfArguments: 2,
		ArgMetadata:       []*ArgMetadata{},
		Arguments:         []interface{}{},
	}

	if chunk.TimeCode != 1000 {
		t.Errorf("expected TimeCode %d, got %d", 1000, chunk.TimeCode)
	}

	if chunk.OrderCode != 1047 {
		t.Errorf("expected OrderCode %d, got %d", 1047, chunk.OrderCode)
	}

	if chunk.PlayerID != 2 {
		t.Errorf("expected PlayerID %d, got %d", 2, chunk.PlayerID)
	}

	if chunk.NumberOfArguments != 2 {
		t.Errorf("expected NumberOfArguments %d, got %d", 2, chunk.NumberOfArguments)
	}
}

func TestBodyChunkEasyUnmarshall(t *testing.T) {
	chunk := BodyChunkEasyUnmarshall{
		TimeCode:          2000,
		OrderCode:         1049,
		OrderName:         "BuildObject",
		PlayerID:          3,
		PlayerName:        "Player2",
		NumberOfArguments: 1,
		Details: GeneralDetail{
			Cost: 500,
			Name: "Barracks",
		},
		ArgMetadata: []*ArgMetadata{},
		Arguments:   []interface{}{},
	}

	if chunk.TimeCode != 2000 {
		t.Errorf("expected TimeCode %d, got %d", 2000, chunk.TimeCode)
	}

	if chunk.Details.Cost != 500 {
		t.Errorf("expected Details.Cost %d, got %d", 500, chunk.Details.Cost)
	}

	if chunk.Details.Name != "Barracks" {
		t.Errorf("expected Details.Name %s, got %s", "Barracks", chunk.Details.Name)
	}
}

func TestGeneralDetail(t *testing.T) {
	detail := GeneralDetail{
		Cost: 1000,
		Name: "Tank",
	}

	if detail.Cost != 1000 {
		t.Errorf("expected Cost %d, got %d", 1000, detail.Cost)
	}

	if detail.Name != "Tank" {
		t.Errorf("expected Name %s, got %s", "Tank", detail.Name)
	}
}

func TestParseBody(t *testing.T) {
	// Create mock data for a simple body chunk
	// TimeCode: 1000, OrderCode: 1047, PlayerID: 2, NumberOfArguments: 1
	// ArgType: ArgInt, ArgCount: 1, ArgValue: 42
	input := []byte{
		232, 3, 0, 0, // TimeCode: 1000 (little endian)
		23, 4, 0, 0, // OrderCode: 1047 (little endian)
		2, 0, 0, 0, // PlayerID: 2 (little endian)
		1,          // NumberOfArguments: 1
		0,          // ArgType: ArgInt
		1,          // ArgCount: 1
		2, 0, 0, 0, // ArgValue: 2 (little endian)
		// End marker
		0, 0, 0, 0, // TimeCode: 0
		0, 0, 0, 0, // OrderCode: 0
		0, 0, 0, 0, // PlayerID: 0
		0, // NumberOfArguments: 0
	}

	parser := &bitparse.BitParser{
		Source: bytes.NewReader(input),
	}

	// Create mock stores
	objectStore := &iniparse.ObjectStore{
		Object: []iniparse.Object{
			{Name: "TestUnit", Cost: 100},
		},
	}
	powerStore := &iniparse.PowerStore{
		Power: []iniparse.Power{
			{Name: "TestPower"},
		},
	}
	upgradeStore := &iniparse.UpgradeStore{
		Upgrade: []iniparse.Upgrade{
			{Name: "TestUpgrade", Cost: 200},
		},
	}

	playerList := []*object.PlayerSummary{}

	body := ParseBody(parser, playerList, objectStore, powerStore, upgradeStore)

	if len(body) != 1 {
		t.Errorf("expected 1 body chunk, got %d", len(body))
		return
	}

	chunk := body[0]
	if chunk.TimeCode != 1000 {
		t.Errorf("expected TimeCode %d, got %d", 1000, chunk.TimeCode)
	}

	if chunk.OrderCode != 1047 {
		t.Errorf("expected OrderCode %d, got %d", 1047, chunk.OrderCode)
	}

	if chunk.PlayerID != 2 {
		t.Errorf("expected PlayerID %d, got %d", 2, chunk.PlayerID)
	}

	if chunk.NumberOfArguments != 1 {
		t.Errorf("expected NumberOfArguments %d, got %d", 1, chunk.NumberOfArguments)
	}

	if len(chunk.ArgMetadata) != 1 {
		t.Errorf("expected 1 ArgMetadata, got %d", len(chunk.ArgMetadata))
	}

	if chunk.ArgMetadata[0].Type != ArgInt {
		t.Errorf("expected ArgType %d, got %d", ArgInt, chunk.ArgMetadata[0].Type)
	}

	if chunk.ArgMetadata[0].Count != 1 {
		t.Errorf("expected ArgCount %d, got %d", 1, chunk.ArgMetadata[0].Count)
	}

	if len(chunk.Arguments) != 1 {
		t.Errorf("expected 1 Argument, got %d", len(chunk.Arguments))
	}

	if chunk.Arguments[0] != 2 {
		t.Errorf("expected Argument %d, got %d", 2, chunk.Arguments[0])
	}
}

func TestParseBodyEmpty(t *testing.T) {
	// Test with empty input (immediate end marker)
	input := []byte{
		0, 0, 0, 0, // TimeCode: 0
		0, 0, 0, 0, // OrderCode: 0
		0, 0, 0, 0, // PlayerID: 0
		0, // NumberOfArguments: 0
	}

	parser := &bitparse.BitParser{
		Source: bytes.NewReader(input),
	}

	body := ParseBody(parser, []*object.PlayerSummary{}, &iniparse.ObjectStore{}, &iniparse.PowerStore{}, &iniparse.UpgradeStore{})

	if len(body) != 0 {
		t.Errorf("expected 0 body chunks, got %d", len(body))
	}
}

func TestParseBodyMultipleChunks(t *testing.T) {
	// Create mock data for multiple body chunks
	input := []byte{
		// First chunk: CreateUnit
		232, 3, 0, 0, // TimeCode: 1000 (little endian)
		23, 4, 0, 0, // OrderCode: 1047 (CreateUnit)
		2, 0, 0, 0, // PlayerID: 2
		1,          // NumberOfArguments: 1
		0,          // ArgType: ArgInt
		1,          // ArgCount: 1
		2, 0, 0, 0, // ArgValue: 2

		// Second chunk: BuildObject
		208, 7, 0, 0, // TimeCode: 2000 (little endian)
		25, 4, 0, 0, // OrderCode: 1049 (BuildObject)
		3, 0, 0, 0, // PlayerID: 3
		1,          // NumberOfArguments: 1
		0,          // ArgType: ArgInt
		1,          // ArgCount: 1
		2, 0, 0, 0, // ArgValue: 2

		// End marker
		0, 0, 0, 0, // TimeCode: 0
		0, 0, 0, 0, // OrderCode: 0
		0, 0, 0, 0, // PlayerID: 0
		0, // NumberOfArguments: 0
	}

	parser := &bitparse.BitParser{
		Source: bytes.NewReader(input),
	}

	// Create mock stores
	objectStore := &iniparse.ObjectStore{
		Object: []iniparse.Object{
			{Name: "TestUnit", Cost: 100},
		},
	}
	powerStore := &iniparse.PowerStore{
		Power: []iniparse.Power{
			{Name: "TestPower"},
		},
	}
	upgradeStore := &iniparse.UpgradeStore{
		Upgrade: []iniparse.Upgrade{
			{Name: "TestUpgrade", Cost: 200},
		},
	}

	body := ParseBody(parser, []*object.PlayerSummary{}, objectStore, powerStore, upgradeStore)

	if len(body) != 2 {
		t.Errorf("expected 2 body chunks, got %d", len(body))
		return
	}

	// Check first chunk
	if body[0].TimeCode != 1000 {
		t.Errorf("expected first chunk TimeCode %d, got %d", 1000, body[0].TimeCode)
	}

	if body[0].OrderCode != 1047 {
		t.Errorf("expected first chunk OrderCode %d, got %d", 1047, body[0].OrderCode)
	}

	// Check second chunk
	if body[1].TimeCode != 2000 {
		t.Errorf("expected second chunk TimeCode %d, got %d", 2000, body[1].TimeCode)
	}

	if body[1].OrderCode != 1049 {
		t.Errorf("expected second chunk OrderCode %d, got %d", 1049, body[1].OrderCode)
	}
}

func TestAddExtraData(t *testing.T) {
	// Create mock stores
	objectStore := &iniparse.ObjectStore{
		Object: []iniparse.Object{
			{Name: "TestUnit", Cost: 100},
		},
	}
	powerStore := &iniparse.PowerStore{
		Power: []iniparse.Power{
			{Name: "TestPower"},
		},
	}
	upgradeStore := &iniparse.UpgradeStore{
		Upgrade: []iniparse.Upgrade{
			{Name: "TestUpgrade", Cost: 200},
		},
	}

	t.Run("CreateUnit", func(t *testing.T) {
		chunk := &BodyChunk{
			OrderCode: 1047,             // CreateUnit
			Arguments: []interface{}{2}, // Object ID 2
		}

		chunk.addExtraData(objectStore, powerStore, upgradeStore)

		if chunk.Details == nil {
			t.Error("expected Details to be set")
			return
		}

		if unit, ok := chunk.Details.(*object.Unit); ok {
			if unit.Name != "TestUnit" {
				t.Errorf("expected unit name %s, got %s", "TestUnit", unit.Name)
			}
			if unit.Cost != 100 {
				t.Errorf("expected unit cost %d, got %d", 100, unit.Cost)
			}
		} else {
			t.Errorf("expected *object.Unit, got %T", chunk.Details)
		}
	})

	t.Run("BuildObject", func(t *testing.T) {
		chunk := &BodyChunk{
			OrderCode: 1049,             // BuildObject
			Arguments: []interface{}{2}, // Object ID 2
		}

		chunk.addExtraData(objectStore, powerStore, upgradeStore)

		if chunk.Details == nil {
			t.Error("expected Details to be set")
			return
		}

		if building, ok := chunk.Details.(*object.Building); ok {
			if building.Name != "TestUnit" {
				t.Errorf("expected building name %s, got %s", "TestUnit", building.Name)
			}
			if building.Cost != 100 {
				t.Errorf("expected building cost %d, got %d", 100, building.Cost)
			}
		} else {
			t.Errorf("expected *object.Building, got %T", chunk.Details)
		}
	})

	t.Run("SpecialPower", func(t *testing.T) {
		chunk := &BodyChunk{
			OrderCode: 1040,             // SpecialPower
			Arguments: []interface{}{2}, // Power ID 2
		}

		chunk.addExtraData(objectStore, powerStore, upgradeStore)

		if chunk.Details == nil {
			t.Error("expected Details to be set")
			return
		}

		if power, ok := chunk.Details.(*object.Power); ok {
			if power.Name != "TestPower" {
				t.Errorf("expected power name %s, got %s", "TestPower", power.Name)
			}
		} else {
			t.Errorf("expected *object.Power, got %T", chunk.Details)
		}
	})

	t.Run("SpecialPowerAtLocation", func(t *testing.T) {
		chunk := &BodyChunk{
			OrderCode: 1041,             // SpecialPowerAtLocation
			Arguments: []interface{}{2}, // Power ID 2
		}

		chunk.addExtraData(objectStore, powerStore, upgradeStore)

		if chunk.Details == nil {
			t.Error("expected Details to be set")
			return
		}

		if power, ok := chunk.Details.(*object.Power); ok {
			if power.Name != "TestPower" {
				t.Errorf("expected power name %s, got %s", "TestPower", power.Name)
			}
		} else {
			t.Errorf("expected *object.Power, got %T", chunk.Details)
		}
	})

	t.Run("SpecialPowerAtObject", func(t *testing.T) {
		chunk := &BodyChunk{
			OrderCode: 1042,             // SpecialPowerAtObject
			Arguments: []interface{}{2}, // Power ID 2
		}

		chunk.addExtraData(objectStore, powerStore, upgradeStore)

		if chunk.Details == nil {
			t.Error("expected Details to be set")
			return
		}

		if power, ok := chunk.Details.(*object.Power); ok {
			if power.Name != "TestPower" {
				t.Errorf("expected power name %s, got %s", "TestPower", power.Name)
			}
		} else {
			t.Errorf("expected *object.Power, got %T", chunk.Details)
		}
	})

	t.Run("Upgrade", func(t *testing.T) {
		chunk := &BodyChunk{
			OrderCode: 1045,                   // Upgrade
			Arguments: []interface{}{1, 2270}, // Player ID 1, Upgrade ID 2270 (offset)
		}

		chunk.addExtraData(objectStore, powerStore, upgradeStore)

		if chunk.Details == nil {
			t.Error("expected Details to be set")
			return
		}

		if upgrade, ok := chunk.Details.(*object.Upgrade); ok {
			if upgrade.Name != "TestUpgrade" {
				t.Errorf("expected upgrade name %s, got %s", "TestUpgrade", upgrade.Name)
			}
			if upgrade.Cost != 200 {
				t.Errorf("expected upgrade cost %d, got %d", 200, upgrade.Cost)
			}
		} else {
			t.Errorf("expected *object.Upgrade, got %T", chunk.Details)
		}
	})

	t.Run("UpgradeWithNilObject", func(t *testing.T) {
		// Create empty upgrade store
		emptyUpgradeStore := &iniparse.UpgradeStore{
			Upgrade: []iniparse.Upgrade{},
		}

		chunk := &BodyChunk{
			OrderCode: 1045,                  // Upgrade
			Arguments: []interface{}{1, 999}, // Player ID 1, Upgrade ID 999 (non-existent)
		}

		chunk.addExtraData(objectStore, powerStore, emptyUpgradeStore)

		if chunk.Details == nil {
			t.Error("expected Details to be set")
			return
		}

		if upgrade, ok := chunk.Details.(*object.Upgrade); ok {
			if upgrade.Name != "dummy" {
				t.Errorf("expected upgrade name %s, got %s", "dummy", upgrade.Name)
			}
		} else {
			t.Errorf("expected *object.Upgrade, got %T", chunk.Details)
		}
	})

	t.Run("UnknownOrderCode", func(t *testing.T) {
		chunk := &BodyChunk{
			OrderCode: 9999, // Unknown order code
			Arguments: []interface{}{},
		}

		chunk.addExtraData(objectStore, powerStore, upgradeStore)

		if chunk.Details != nil {
			t.Errorf("expected Details to be nil for unknown order code, got %T", chunk.Details)
		}
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("ParseBodyWithInsufficientData", func(t *testing.T) {
		// Test with insufficient data (should handle gracefully)
		input := []byte{1, 2, 3} // Not enough data for a complete chunk

		parser := &bitparse.BitParser{
			Source: bytes.NewReader(input),
		}

		// This should not panic and should return empty or partial results
		body := ParseBody(parser, []*object.PlayerSummary{}, &iniparse.ObjectStore{}, &iniparse.PowerStore{}, &iniparse.UpgradeStore{})

		// The exact behavior depends on the implementation, but it shouldn't panic
		_ = body // Use the result to avoid unused variable warning
	})

	t.Run("ConvertArgWithInsufficientData", func(t *testing.T) {
		// Test convertArg with insufficient data
		parser := &bitparse.BitParser{
			Source: bytes.NewReader([]byte{1}), // Only 1 byte, but ArgInt needs 4
		}

		result := convertArg(parser, ArgInt)
		// Should return 0 or handle gracefully
		_ = result
	})

	t.Run("ConvertArgWithEmptyInput", func(t *testing.T) {
		parser := &bitparse.BitParser{
			Source: bytes.NewReader([]byte{}), // Empty input
		}

		result := convertArg(parser, ArgInt)
		// Should return 0 or handle gracefully
		_ = result
	})
}

func TestIntegrationScenarios(t *testing.T) {
	t.Run("ComplexBodyChunk", func(t *testing.T) {
		// Test a simple body chunk with one argument
		input := []byte{
			// TimeCode: 1000
			232, 3, 0, 0,
			// OrderCode: 1047 (CreateUnit)
			39, 4, 0, 0,
			// PlayerID: 2
			2, 0, 0, 0,
			// NumberOfArguments: 1
			1,
			// Arg1: ArgInt, Count: 1
			0, 1, // ArgInt=0, Count=1
			// Arg1 Value: 42
			42, 0, 0, 0,
			// End marker
			0, 0, 0, 0,
			0, 0, 0, 0,
			0, 0, 0, 0,
			0,
		}

		parser := &bitparse.BitParser{
			Source: bytes.NewReader(input),
		}

		body := ParseBody(parser, []*object.PlayerSummary{}, &iniparse.ObjectStore{}, &iniparse.PowerStore{}, &iniparse.UpgradeStore{})

		if len(body) != 1 {
			t.Errorf("expected 1 body chunk, got %d", len(body))
			return
		}

		chunk := body[0]
		if len(chunk.Arguments) != 1 {
			t.Errorf("expected 1 argument, got %d", len(chunk.Arguments))
		}

		// Check argument
		if chunk.Arguments[0] != 42 {
			t.Errorf("expected argument 42, got %v", chunk.Arguments[0])
		}
	})

	t.Run("PositionArgument", func(t *testing.T) {
		// Test with position argument
		input := []byte{
			// TimeCode: 1000
			232, 3, 0, 0,
			// OrderCode: 1068 (MoveTo)
			84, 4, 0, 0,
			// PlayerID: 2
			2, 0, 0, 0,
			// NumberOfArguments: 1
			1,
			// Arg1: ArgPosition, Count: 1
			ArgPosition, 1,
			// Arg1 Value: Position(1.0, 2.0, 3.0)
			0, 0, 128, 63, // 1.0
			0, 0, 0, 64, // 2.0
			0, 0, 64, 64, // 3.0
			// End marker
			0, 0, 0, 0,
			0, 0, 0, 0,
			0, 0, 0, 0,
			0,
		}

		parser := &bitparse.BitParser{
			Source: bytes.NewReader(input),
		}

		body := ParseBody(parser, []*object.PlayerSummary{}, &iniparse.ObjectStore{}, &iniparse.PowerStore{}, &iniparse.UpgradeStore{})

		if len(body) != 1 {
			t.Errorf("expected 1 body chunk, got %d", len(body))
			return
		}

		chunk := body[0]
		if len(chunk.Arguments) != 1 {
			t.Errorf("expected 1 argument, got %d", len(chunk.Arguments))
			return
		}

		if pos, ok := chunk.Arguments[0].(Position3D); ok {
			if pos.X != float32(1.0) || pos.Y != float32(2.0) || pos.Z != float32(3.0) {
				t.Errorf("expected position (1.0, 2.0, 3.0), got (%v, %v, %v)", pos.X, pos.Y, pos.Z)
			}
		} else {
			t.Errorf("expected Position3D, got %T", chunk.Arguments[0])
		}
	})
}
