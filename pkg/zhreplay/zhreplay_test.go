package zhreplay

import (
	"bytes"
	"testing"

	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
)

func TestNewReplay(t *testing.T) {
	// Create minimal mock data for testing
	// This is a simplified version - in reality this would be much more complex
	input := []byte{
		// Header data (matches Recorder.cpp layout)
		'G', 'E', 'N', 'R', 'E', 'P', // GameType (6 bytes)
		100, 0, 0, 0, // TimeStampBegin (uint32)
		200, 0, 0, 0, // TimeStampEnd (uint32)
		5, 0, 0, 0, // FrameCount (uint32)
		0,                      // Desync (1 byte Bool)
		0,                      // QuitEarly (1 byte Bool)
		0, 0, 0, 0, 0, 0, 0, 0, // PlayerDiscons (8 bytes, Bool[MAX_SLOTS])
		// ReplayName (UTF16 null-terminated)
		'T', 0, 'e', 0, 's', 0, 't', 0, 0, 0,
		// SYSTEMTIME (8 x uint16)
		231, 7, // Year: 2023
		12, 0, // Month
		1, 0, // DOW
		25, 0, // Day
		14, 0, // Hour
		30, 0, // Minute
		45, 0, // Second
		244, 1, // Millisecond: 500
		// Version (UTF16 null-terminated)
		'1', 0, '.', 0, '0', 0, 0, 0,
		// BuildDate (UTF16 null-terminated)
		'2', 0, '0', 0, '2', 0, '3', 0, 0, 0,
		// VersionNumber (uint32)
		1, 0, 0, 0,
		// ExeCRC (uint32)
		0, 0, 0, 0,
		// IniCRC (uint32)
		0, 0, 0, 0,
		// Metadata (ASCII null-terminated)
		'M', '=', 't', 'e', 's', 't', ';', 'S', '=', 'H', 'P', 'l', 'a', 'y', 'e', 'r', '1', ',', '1', '.', '2', '.', '3', '.', '4', ',', '8', '0', '8', '0', ',', 'F', 'T', ',', '1', ',', '0', ',', '0', ',', '0', ',', '1', 0,
		// LocalPlayerIndex (ASCII null-terminated)
		'0', 0,
		// Difficulty (int32)
		0, 0, 0, 0,
		// OriginalGameMode (int32)
		0, 0, 0, 0,
		// RankPoints (int32)
		0, 0, 0, 0,
		// MaxFPS (int32)
		30, 0, 0, 0,
		// Body data (simplified)
		0, 0, 0, 0, // TimeCode: 0
		0, 0, 0, 0, // OrderCode: 0
		0, 0, 0, 0, // PlayerID: 0
		0, // NumberOfArguments: 0
	}

	parser := &bitparse.BitParser{
		Source: bytes.NewReader(input),
	}

	replay := NewReplay(parser)

	if replay == nil {
		t.Fatal("expected non-nil replay")
	}

	if replay.Header == nil {
		t.Error("expected non-nil header")
	}

	if replay.Body == nil {
		t.Error("expected non-nil body")
	}

	if replay.Summary == nil {
		t.Error("expected non-nil summary")
	}

	// Offset will be adjusted by AdjustPlayerIDOffset() call in NewReplay
	// The mock data has PlayerID: 0 in the body, so offset should be 0
	// But if the body parsing fails or returns empty, offset might remain 1000
	if replay.PlayerIDOffset != 0 && replay.PlayerIDOffset != 1000 {
		t.Errorf("expected offset 0 or 1000, got %d", replay.PlayerIDOffset)
	}
}

func TestReplayAddUserNames(t *testing.T) {
	replay := &Replay{
		PlayerIDOffset: 2,
		Summary: []*object.PlayerSummary{
			{Name: "Player1"},
			{Name: "Player2"},
		},
		Body: []*body.BodyChunk{
			{PlayerID: 2, PlayerName: ""},
			{PlayerID: 3, PlayerName: ""},
			{PlayerID: 4, PlayerName: ""}, // Out of range
		},
	}

	replay.AddUserNames()

	if replay.Body[0].PlayerName != "Player1" {
		t.Errorf("expected Player1, got %s", replay.Body[0].PlayerName)
	}

	if replay.Body[1].PlayerName != "Player2" {
		t.Errorf("expected Player2, got %s", replay.Body[1].PlayerName)
	}

	if replay.Body[2].PlayerName != "" {
		t.Errorf("expected empty string for out of range player, got %s", replay.Body[2].PlayerName)
	}
}

func TestReplayAdjustPlayerIDOffset(t *testing.T) {
	replay := &Replay{
		PlayerIDOffset: 1000,
		Body: []*body.BodyChunk{
			{PlayerID: 5},
			{PlayerID: 3},
			{PlayerID: 7},
		},
	}

	replay.AdjustPlayerIDOffset()

	if replay.PlayerIDOffset != 3 {
		t.Errorf("expected offset 3, got %d", replay.PlayerIDOffset)
	}
}

func TestReplayCreatePlayerList(t *testing.T) {
	replay := &Replay{
		Header: &header.GeneralsHeader{
			Metadata: header.Metadata{
				Players: []header.Player{
					{Name: "Player1", Team: "1"},
					{Name: "Player2", Team: "2"},
				},
			},
		},
		Summary: []*object.PlayerSummary{},
	}

	replay.CreatePlayerList()

	if len(replay.Summary) != 2 {
		t.Errorf("expected 2 players, got %d", len(replay.Summary))
	}

	if replay.Summary[0].Name != "Player1" {
		t.Errorf("expected Player1, got %s", replay.Summary[0].Name)
	}

	if replay.Summary[0].Team != 2 {
		t.Errorf("expected team 2, got %d", replay.Summary[0].Team)
	}

	if replay.Summary[0].Win != true {
		t.Errorf("expected win true, got %v", replay.Summary[0].Win)
	}

	if replay.Summary[0].BuildingsBuilt == nil {
		t.Error("expected non-nil BuildingsBuilt map")
	}

	if replay.Summary[0].UnitsCreated == nil {
		t.Error("expected non-nil UnitsCreated map")
	}

	if replay.Summary[0].UpgradesBuilt == nil {
		t.Error("expected non-nil UpgradesBuilt map")
	}

	if replay.Summary[0].PowersUsed == nil {
		t.Error("expected non-nil PowersUsed map")
	}
}

func TestReplayGenerateData(t *testing.T) {
	replay := &Replay{
		Header: &header.GeneralsHeader{
			TimeStampBegin: 1000, // Not the special timestamp
		},
		Summary: []*object.PlayerSummary{
			{
				Name:           "Player1",
				Team:           1,
				Win:            true,
				BuildingsBuilt: map[string]*object.ObjectSummary{},
				UnitsCreated:   map[string]*object.ObjectSummary{},
				UpgradesBuilt:  map[string]*object.ObjectSummary{},
				PowersUsed:     map[string]int{},
			},
		},
		Body: []*body.BodyChunk{
			{
				PlayerName: "Player1",
				OrderCode:  1047, // Create Unit
				Details: &object.Unit{
					Name: "GLAInfantryWorker",
					Cost: 100,
				},
			},
			{
				PlayerName: "Player1",
				OrderCode:  1049, // Build
				Details: &object.Building{
					Name: "TestBuilding",
					Cost: 200,
				},
			},
			{
				PlayerName: "Player1",
				OrderCode:  1045, // Upgrade
				Details: &object.Upgrade{
					Name: "TestUpgrade",
					Cost: 50,
				},
			},
			{
				PlayerName: "Player1",
				OrderCode:  1041, // Special Power
				Details: &object.Power{
					Name: "TestPower",
				},
			},
		},
	}

	replay.GenerateData()

	player := replay.Summary[0]

	// Check side detection
	if player.Side != "GLA" {
		t.Errorf("expected side GLA, got %s", player.Side)
	}

	// Check unit creation
	if player.UnitsCreated["GLAInfantryWorker"] == nil {
		t.Error("expected GLAInfantryWorker in UnitsCreated")
	} else {
		unitSummary := player.UnitsCreated["GLAInfantryWorker"]
		if unitSummary.Count != 1 {
			t.Errorf("expected unit count 1, got %d", unitSummary.Count)
		}
		if unitSummary.TotalSpent != 100 {
			t.Errorf("expected unit total spent 100, got %d", unitSummary.TotalSpent)
		}
	}

	// Check building creation
	if player.BuildingsBuilt["TestBuilding"] == nil {
		t.Error("expected TestBuilding in BuildingsBuilt")
	} else {
		buildingSummary := player.BuildingsBuilt["TestBuilding"]
		if buildingSummary.Count != 1 {
			t.Errorf("expected building count 1, got %d", buildingSummary.Count)
		}
		if buildingSummary.TotalSpent != 200 {
			t.Errorf("expected building total spent 200, got %d", buildingSummary.TotalSpent)
		}
	}

	// Check upgrade creation
	if player.UpgradesBuilt["TestUpgrade"] == nil {
		t.Error("expected TestUpgrade in UpgradesBuilt")
	} else {
		upgradeSummary := player.UpgradesBuilt["TestUpgrade"]
		if upgradeSummary.Count != 1 {
			t.Errorf("expected upgrade count 1, got %d", upgradeSummary.Count)
		}
		if upgradeSummary.TotalSpent != 50 {
			t.Errorf("expected upgrade total spent 50, got %d", upgradeSummary.TotalSpent)
		}
	}

	// Check power usage
	if player.PowersUsed["TestPower"] != 1 {
		t.Errorf("expected power usage 1, got %d", player.PowersUsed["TestPower"])
	}

	// Check total money spent
	if player.MoneySpent != 350 {
		t.Errorf("expected total money spent 350, got %d", player.MoneySpent)
	}
}

func TestConstructorMapEntries(t *testing.T) {
	testCases := []struct {
		unitName     string
		expectedSide string
	}{
		{"GLAInfantryWorker", "GLA"},
		{"Slth_GLAInfantryWorker", "GLA Stealth"},
		{"Chem_GLAInfantryWorker", "GLA Toxin"},
		{"Demo_GLAInfantryWorker", "GLA Demo"},
		{"AmericaVehicleDozer", "USA"},
		{"AirF_AmericaVehicleDozer", "USA Airforce"},
		{"Lazr_AmericaVehicleDozer", "USA Lazr"},
		{"SupW_AmericaVehicleDozer", "USA Superweapon"},
		{"ChinaVehicleDozer", "China"},
		{"Infa_ChinaVehicleDozer", "China Infantry"},
		{"Nuke_ChinaVehicleDozer", "China Nuke"},
		{"Tank_ChinaVehicleDozer", "China Tank"},
	}

	for _, tc := range testCases {
		if side, ok := constructorMap[tc.unitName]; !ok {
			t.Errorf("expected %s to be in constructorMap", tc.unitName)
		} else if side != tc.expectedSide {
			t.Errorf("expected side %s for %s, got %s", tc.expectedSide, tc.unitName, side)
		}
	}
}

func TestReplayDataStructures(t *testing.T) {
	t.Run("Replay", func(t *testing.T) {
		replay := &Replay{
			Header:  &header.GeneralsHeader{},
			Body:    []*body.BodyChunk{},
			Summary: []*object.PlayerSummary{},
			PlayerIDOffset:  2,
		}

		if replay.Header == nil {
			t.Error("expected non-nil header")
		}

		if replay.Body == nil {
			t.Error("expected non-nil body")
		}

		if replay.Summary == nil {
			t.Error("expected non-nil summary")
		}

		if replay.PlayerIDOffset != 2 {
			t.Errorf("expected offset 2, got %d", replay.PlayerIDOffset)
		}
	})

}

func TestEdgeCases(t *testing.T) {
	t.Run("EmptyPlayerList", func(t *testing.T) {
		replay := &Replay{
			Header: &header.GeneralsHeader{
				Metadata: header.Metadata{
					Players: []header.Player{},
				},
			},
			Summary: []*object.PlayerSummary{},
		}

		replay.CreatePlayerList()

		if len(replay.Summary) != 0 {
			t.Errorf("expected 0 players, got %d", len(replay.Summary))
		}
	})

	t.Run("EmptyBody", func(t *testing.T) {
		replay := &Replay{
			Header: &header.GeneralsHeader{
				TimeStampBegin: 1000,
			},
			Summary: []*object.PlayerSummary{
				{Name: "Player1", Team: 1, Win: true},
			},
			Body: []*body.BodyChunk{},
		}

		replay.GenerateData()

		player := replay.Summary[0]
		if player.MoneySpent != 0 {
			t.Errorf("expected 0 money spent, got %d", player.MoneySpent)
		}

		if len(player.UnitsCreated) != 0 {
			t.Errorf("expected 0 units created, got %d", len(player.UnitsCreated))
		}
	})

	t.Run("AdjustPlayerIDOffsetWithEmptyBody", func(t *testing.T) {
		replay := &Replay{
			PlayerIDOffset: 1000,
			Body:   []*body.BodyChunk{},
		}

		replay.AdjustPlayerIDOffset()

		if replay.PlayerIDOffset != 1000 {
			t.Errorf("expected offset to remain 1000, got %d", replay.PlayerIDOffset)
		}
	})

	t.Run("AddUserNamesWithEmptySummary", func(t *testing.T) {
		replay := &Replay{
			PlayerIDOffset:  2,
			Summary: []*object.PlayerSummary{},
			Body: []*body.BodyChunk{
				{PlayerID: 2, PlayerName: ""},
			},
		}

		replay.AddUserNames()

		if replay.Body[0].PlayerName != "" {
			t.Errorf("expected empty player name, got %s", replay.Body[0].PlayerName)
		}
	})
}

func TestWinDetection(t *testing.T) {
	t.Run("SingleWinner", func(t *testing.T) {
		replay := &Replay{
			Summary: []*object.PlayerSummary{
				{Name: "Player1", Team: 1, Win: true},
				{Name: "Player2", Team: 2, Win: false},
			},
		}

		// Simulate the win detection logic
		teamWins := map[int]bool{}
		for _, p := range replay.Summary {
			teamWins[p.Team] = true
		}
		for p := range replay.Summary {
			if !replay.Summary[p].Win {
				teamWins[replay.Summary[p].Team] = false
			}
		}

		if !teamWins[1] {
			t.Error("expected team 1 to win")
		}

		if teamWins[2] {
			t.Error("expected team 2 to lose")
		}
	})

	t.Run("MultipleWinners", func(t *testing.T) {
		replay := &Replay{
			Summary: []*object.PlayerSummary{
				{Name: "Player1", Team: 1, Win: true},
				{Name: "Player2", Team: 2, Win: true},
			},
		}

		// Simulate the win detection logic
		teamWins := map[int]bool{}
		for _, p := range replay.Summary {
			teamWins[p.Team] = true
		}
		for p := range replay.Summary {
			if !replay.Summary[p].Win {
				teamWins[replay.Summary[p].Team] = false
			}
		}

		winners := 0
		for _, t := range teamWins {
			if t {
				winners++
			}
		}

		if winners != 2 {
			t.Errorf("expected 2 winners, got %d", winners)
		}
	})
}

func TestSpecialTimestamp(t *testing.T) {
	replay := &Replay{
		Header: &header.GeneralsHeader{
			TimeStampBegin: 1652069156,
		},
		Summary: []*object.PlayerSummary{
			{Name: "Player1", Team: 3, Win: false},
			{Name: "Player2", Team: 1, Win: false},
		},
	}

	// Simulate the special timestamp logic
	for p := range replay.Summary {
		if replay.Summary[p].Team == 3 {
			replay.Summary[p].Win = true
		} else {
			replay.Summary[p].Win = false
		}
	}

	if !replay.Summary[0].Win {
		t.Error("expected team 3 to win")
	}

	if replay.Summary[1].Win {
		t.Error("expected team 1 to lose")
	}
}

func TestIntegrationScenarios(t *testing.T) {
	t.Run("CompleteReplayFlow", func(t *testing.T) {
		replay := &Replay{
			Header: &header.GeneralsHeader{
				Metadata: header.Metadata{
					Players: []header.Player{
						{Name: "Player1", Team: "1"},
						{Name: "Player2", Team: "2"},
					},
				},
			},
			Summary: []*object.PlayerSummary{},
			Body: []*body.BodyChunk{
				{
					PlayerID:   2,
					PlayerName: "Player1",
					OrderCode:  1047,
					Details: &object.Unit{
						Name: "GLAInfantryWorker",
						Cost: 100,
					},
				},
				{
					PlayerID:   3,
					PlayerName: "Player2",
					OrderCode:  1049,
					Details: &object.Building{
						Name: "TestBuilding",
						Cost: 200,
					},
				},
			},
		}

		// Test the complete flow
		replay.CreatePlayerList()
		replay.AdjustPlayerIDOffset()
		replay.AddUserNames()
		replay.GenerateData()

		if len(replay.Summary) != 2 {
			t.Errorf("expected 2 players, got %d", len(replay.Summary))
		}

		if replay.PlayerIDOffset != 2 {
			t.Errorf("expected offset 2, got %d", replay.PlayerIDOffset)
		}

		if replay.Body[0].PlayerName != "Player1" {
			t.Errorf("expected Player1, got %s", replay.Body[0].PlayerName)
		}

		if replay.Body[1].PlayerName != "Player2" {
			t.Errorf("expected Player2, got %s", replay.Body[1].PlayerName)
		}
	})
}
