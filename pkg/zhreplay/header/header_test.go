package header

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/bill-rich/cncstats/pkg/bitparse"
)

func TestMetadataParse(t *testing.T) {
	input := "US=1;M=07maps/tournament island;MC=12BE477C;MS=130668;SD=6449734;C=100;SR=0;SC=10000;O=N;S=HModus,17F04000,8088,FT,7,-1,-1,0,1:HYe_Ole_Seans,48595000,8088,FT,0,-1,-1,2,1:HOneThree111,49DDD000,8088,FT,6,-1,-1,2,1:Hjbb,18099000,8088,FT,3,-1,-1,0,1:X:X:X:X:;"
	mdOut := parseMetadata(input)
	mdExpected := Metadata{
		MapFile:         "07maps/tournament island",
		MapCRC:          "12BE477C",
		MapSize:         "130668",
		Seed:            "6449734",
		C:               "100",
		SR:              "0",
		StartingCredits: "10000",
		O:               "N",
		Players: []Player{
			{
				Type:             "H",
				Name:             "Modus",
				IP:               "17F04000",
				Port:             "8088",
				FT:               "FT",
				Color:            "7",
				Faction:          "-1",
				StartingPosition: "-1",
				Team:             "0",
				Unknown:          "1",
			},
			{
				Type:             "H",
				Name:             "Ye_Ole_Seans",
				IP:               "48595000",
				Port:             "8088",
				FT:               "FT",
				Color:            "0",
				Faction:          "-1",
				StartingPosition: "-1",
				Team:             "2",
				Unknown:          "1",
			},
			{
				Type:             "H",
				Name:             "OneThree111",
				IP:               "49DDD000",
				Port:             "8088",
				FT:               "FT",
				Color:            "6",
				Faction:          "-1",
				StartingPosition: "-1",
				Team:             "2",
				Unknown:          "1",
			},
			{
				Type:             "H",
				Name:             "jbb",
				IP:               "18099000",
				Port:             "8088",
				FT:               "FT",
				Color:            "3",
				Faction:          "-1",
				StartingPosition: "-1",
				Team:             "0",
				Unknown:          "1",
			},
		},
	}
	if !reflect.DeepEqual(mdOut, mdExpected) {
		t.Errorf("unexpected metadata parsing.\n got:%+v\n expected:%+v\n", mdOut, mdExpected)
	}
}

func TestNewHeader(t *testing.T) {
	// Create mock data for a complete header
	// This is a simplified version - in reality this would be much more complex
	input := []byte{
		// GameType (6 bytes)
		'G', 'E', 'N', 'E', 'R', 'A',
		// TimeStampBegin (4 bytes)
		100, 0, 0, 0,
		// TimeStampEnd (4 bytes)
		200, 0, 0, 0,
		// NumTimeStamps (2 bytes)
		5, 0,
		// Filler (12 bytes)
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
		// FileName (UTF16 null-terminated)
		'T', 0, 'e', 0, 's', 0, 't', 0, 0, 0,
		// Year (2 bytes) - 2023 in little-endian
		231, 7,
		// Month (2 bytes)
		12, 0,
		// DOW (2 bytes)
		1, 0,
		// Day (2 bytes)
		25, 0,
		// Hour (2 bytes)
		14, 0,
		// Minute (2 bytes)
		30, 0,
		// Second (2 bytes)
		45, 0,
		// Millisecond (2 bytes) - 500 in little-endian
		244, 1,
		// Version (UTF16 null-terminated)
		'1', 0, '.', 0, '0', 0, 0, 0,
		// BuildDate (UTF16 null-terminated)
		'2', 0, '0', 0, '2', 0, '3', 0, 0, 0,
		// VersionMinor (2 bytes)
		0, 0,
		// VersionMajor (2 bytes)
		1, 0,
		// Hash (8 bytes)
		1, 2, 3, 4, 5, 6, 7, 8,
		// Metadata (UTF8 null-terminated)
		'M', '=', 't', 'e', 's', 't', ';', 0,
		// ReplayOwnerSlot (2 bytes)
		0x30, 0x00,
		// Unknown1 (4 bytes)
		1, 2, 3, 4,
		// Unknown2 (4 bytes)
		5, 6, 7, 8,
		// Unknown3 (4 bytes)
		9, 10, 11, 12,
		// GameSpeed (4 bytes)
		1, 0, 0, 0,
	}

	parser := &bitparse.BitParser{
		Source: bytes.NewReader(input),
	}

	header := NewHeader(parser)

	if header.GameType != "GENERA" {
		t.Errorf("expected GameType 'GENERA', got '%s'", header.GameType)
	}

	if header.TimeStampBegin != 100 {
		t.Errorf("expected TimeStampBegin 100, got %d", header.TimeStampBegin)
	}

	if header.TimeStampEnd != 200 {
		t.Errorf("expected TimeStampEnd 200, got %d", header.TimeStampEnd)
	}

	if header.NumTimeStamps != 5 {
		t.Errorf("expected NumTimeStamps 5, got %d", header.NumTimeStamps)
	}

	if header.Year != 2023 {
		t.Errorf("expected Year 2023, got %d", header.Year)
	}

	if header.Month != 12 {
		t.Errorf("expected Month 12, got %d", header.Month)
	}

	if header.GameSpeed != 1 {
		t.Errorf("expected GameSpeed 1, got %d", header.GameSpeed)
	}
}

func TestParseMetadata(t *testing.T) {
	t.Run("CompleteMetadata", func(t *testing.T) {
		input := "M=testmap;MC=12345678;MS=100000;SD=9876543;C=50;SR=1;SC=5000;O=Y;S=HPlayer1,1.2.3.4,8080,FT,1,0,0,0,1"
		metadata := parseMetadata(input)

		if metadata.MapFile != "testmap" {
			t.Errorf("expected MapFile 'testmap', got '%s'", metadata.MapFile)
		}

		if metadata.MapCRC != "12345678" {
			t.Errorf("expected MapCRC '12345678', got '%s'", metadata.MapCRC)
		}

		if metadata.MapSize != "100000" {
			t.Errorf("expected MapSize '100000', got '%s'", metadata.MapSize)
		}

		if metadata.Seed != "9876543" {
			t.Errorf("expected Seed '9876543', got '%s'", metadata.Seed)
		}

		if metadata.C != "50" {
			t.Errorf("expected C '50', got '%s'", metadata.C)
		}

		if metadata.SR != "1" {
			t.Errorf("expected SR '1', got '%s'", metadata.SR)
		}

		if metadata.StartingCredits != "5000" {
			t.Errorf("expected StartingCredits '5000', got '%s'", metadata.StartingCredits)
		}

		if metadata.O != "Y" {
			t.Errorf("expected O 'Y', got '%s'", metadata.O)
		}

		if len(metadata.Players) != 1 {
			t.Errorf("expected 1 player, got %d", len(metadata.Players))
		}

		if metadata.Players[0].Name != "Player1" {
			t.Errorf("expected player name 'Player1', got '%s'", metadata.Players[0].Name)
		}
	})

	t.Run("EmptyInput", func(t *testing.T) {
		metadata := parseMetadata("")

		if metadata.MapFile != "" {
			t.Errorf("expected empty MapFile, got '%s'", metadata.MapFile)
		}

		if len(metadata.Players) != 0 {
			t.Errorf("expected 0 players, got %d", len(metadata.Players))
		}
	})

	t.Run("InvalidFieldFormat", func(t *testing.T) {
		input := "M=testmap;INVALID_FIELD;MC=12345678"
		metadata := parseMetadata(input)

		if metadata.MapFile != "testmap" {
			t.Errorf("expected MapFile 'testmap', got '%s'", metadata.MapFile)
		}

		if metadata.MapCRC != "12345678" {
			t.Errorf("expected MapCRC '12345678', got '%s'", metadata.MapCRC)
		}
	})

	t.Run("PartialMetadata", func(t *testing.T) {
		input := "M=testmap;MC=12345678"
		metadata := parseMetadata(input)

		if metadata.MapFile != "testmap" {
			t.Errorf("expected MapFile 'testmap', got '%s'", metadata.MapFile)
		}

		if metadata.MapCRC != "12345678" {
			t.Errorf("expected MapCRC '12345678', got '%s'", metadata.MapCRC)
		}

		if metadata.MapSize != "" {
			t.Errorf("expected empty MapSize, got '%s'", metadata.MapSize)
		}
	})
}

func TestParsePlayers(t *testing.T) {
	t.Run("SinglePlayer", func(t *testing.T) {
		input := "HPlayer1,1.2.3.4,8080,FT,1,0,0,0,1"
		players := parsePlayers(input)

		if len(players) != 1 {
			t.Errorf("expected 1 player, got %d", len(players))
		}

		player := players[0]
		if player.Type != "H" {
			t.Errorf("expected Type 'H', got '%s'", player.Type)
		}

		if player.Name != "Player1" {
			t.Errorf("expected Name 'Player1', got '%s'", player.Name)
		}

		if player.IP != "1.2.3.4" {
			t.Errorf("expected IP '1.2.3.4', got '%s'", player.IP)
		}

		if player.Port != "8080" {
			t.Errorf("expected Port '8080', got '%s'", player.Port)
		}

		if player.FT != "FT" {
			t.Errorf("expected FT 'FT', got '%s'", player.FT)
		}

		if player.Color != "1" {
			t.Errorf("expected Color '1', got '%s'", player.Color)
		}

		if player.Faction != "0" {
			t.Errorf("expected Faction '0', got '%s'", player.Faction)
		}

		if player.StartingPosition != "0" {
			t.Errorf("expected StartingPosition '0', got '%s'", player.StartingPosition)
		}

		if player.Team != "0" {
			t.Errorf("expected Team '0', got '%s'", player.Team)
		}

		if player.Unknown != "1" {
			t.Errorf("expected Unknown '1', got '%s'", player.Unknown)
		}
	})

	t.Run("MultiplePlayers", func(t *testing.T) {
		input := "HPlayer1,1.2.3.4,8080,FT,1,0,0,0,1:HPlayer2,5.6.7.8,8081,FT,2,1,1,1,2"
		players := parsePlayers(input)

		if len(players) != 2 {
			t.Errorf("expected 2 players, got %d", len(players))
		}

		if players[0].Name != "Player1" {
			t.Errorf("expected first player name 'Player1', got '%s'", players[0].Name)
		}

		if players[1].Name != "Player2" {
			t.Errorf("expected second player name 'Player2', got '%s'", players[1].Name)
		}
	})

	t.Run("EmptyInput", func(t *testing.T) {
		players := parsePlayers("")

		if len(players) != 0 {
			t.Errorf("expected 0 players, got %d", len(players))
		}
	})

	t.Run("InvalidPlayerFormat", func(t *testing.T) {
		input := "HPlayer1,1.2.3.4,8080,FT,1,0,0,0:HPlayer2,5.6.7.8,8081,FT,2,1,1,1,2"
		players := parsePlayers(input)

		// Should skip invalid players and only include valid ones
		if len(players) != 1 {
			t.Errorf("expected 1 player (invalid one skipped), got %d", len(players))
			return
		}

		if players[0].Name != "Player2" {
			t.Errorf("expected player name 'Player2', got '%s'", players[0].Name)
		}
	})

	t.Run("PlayerWithSpecialCharacters", func(t *testing.T) {
		input := "HPlayer_With_Underscores,1.2.3.4,8080,FT,1,0,0,0,1"
		players := parsePlayers(input)

		if len(players) != 1 {
			t.Errorf("expected 1 player, got %d", len(players))
		}

		if players[0].Name != "Player_With_Underscores" {
			t.Errorf("expected player name 'Player_With_Underscores', got '%s'", players[0].Name)
		}
	})
}

func TestDataStructures(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		metadata := Metadata{
			MapFile:         "testmap",
			MapCRC:          "12345678",
			MapSize:         "100000",
			Seed:            "9876543",
			C:               "50",
			SR:              "1",
			StartingCredits: "5000",
			O:               "Y",
			Players: []Player{
				{Name: "Player1"},
			},
		}

		if metadata.MapFile != "testmap" {
			t.Errorf("expected MapFile 'testmap', got '%s'", metadata.MapFile)
		}

		if len(metadata.Players) != 1 {
			t.Errorf("expected 1 player, got %d", len(metadata.Players))
		}
	})

	t.Run("Player", func(t *testing.T) {
		player := Player{
			Type:             "H",
			Name:             "TestPlayer",
			IP:               "1.2.3.4",
			Port:             "8080",
			FT:               "FT",
			Color:            "1",
			Faction:          "0",
			StartingPosition: "0",
			Team:             "0",
			Unknown:          "1",
		}

		if player.Name != "TestPlayer" {
			t.Errorf("expected Name 'TestPlayer', got '%s'", player.Name)
		}

		if player.Type != "H" {
			t.Errorf("expected Type 'H', got '%s'", player.Type)
		}
	})

	t.Run("GeneralsHeader", func(t *testing.T) {
		header := GeneralsHeader{
			GameType:       "GENERA",
			TimeStampBegin: 100,
			TimeStampEnd:   200,
			NumTimeStamps:  5,
			FileName:       "test.rep",
			Year:           2023,
			Month:          12,
			Day:            25,
			Hour:           14,
			Minute:         30,
			Second:         45,
			Millisecond:    500,
			Version:        "1.0",
			BuildDate:      "2023-12-25",
			VersionMinor:   0,
			VersionMajor:   1,
			Hash:           []byte{1, 2, 3, 4, 5, 6, 7, 8},
			GameSpeed:      1,
		}

		if header.GameType != "GENERA" {
			t.Errorf("expected GameType 'GENERA', got '%s'", header.GameType)
		}

		if header.TimeStampBegin != 100 {
			t.Errorf("expected TimeStampBegin 100, got %d", header.TimeStampBegin)
		}

		if header.Year != 2023 {
			t.Errorf("expected Year 2023, got %d", header.Year)
		}
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("ParseMetadataWithEmptyFields", func(t *testing.T) {
		input := "M=;MC=;MS=;SD=;C=;SR=;SC=;O=;S="
		metadata := parseMetadata(input)

		// All fields should be empty strings
		if metadata.MapFile != "" {
			t.Errorf("expected empty MapFile, got '%s'", metadata.MapFile)
		}

		if metadata.MapCRC != "" {
			t.Errorf("expected empty MapCRC, got '%s'", metadata.MapCRC)
		}
	})

	t.Run("ParsePlayersWithEmptyFields", func(t *testing.T) {
		input := "H,,,,,,,,"
		players := parsePlayers(input)

		if len(players) != 1 {
			t.Errorf("expected 1 player, got %d", len(players))
		}

		player := players[0]
		if player.Type != "H" {
			t.Errorf("expected Type 'H', got '%s'", player.Type)
		}

		if player.Name != "" {
			t.Errorf("expected empty Name, got '%s'", player.Name)
		}
	})

	t.Run("ParseMetadataWithUnknownFields", func(t *testing.T) {
		input := "M=testmap;UNKNOWN=value;MC=12345678;ANOTHER=test"
		metadata := parseMetadata(input)

		// Should still parse known fields correctly
		if metadata.MapFile != "testmap" {
			t.Errorf("expected MapFile 'testmap', got '%s'", metadata.MapFile)
		}

		if metadata.MapCRC != "12345678" {
			t.Errorf("expected MapCRC '12345678', got '%s'", metadata.MapCRC)
		}
	})

	t.Run("ParsePlayersWithTooFewFields", func(t *testing.T) {
		input := "HPlayer1,1.2.3.4,8080,FT,1,0,0"
		players := parsePlayers(input)

		// Should skip invalid player
		if len(players) != 0 {
			t.Errorf("expected 0 players (invalid format), got %d", len(players))
		}
	})

	t.Run("ParsePlayersWithTooManyFields", func(t *testing.T) {
		input := "HPlayer1,1.2.3.4,8080,FT,1,0,0,0,1,extra,field"
		players := parsePlayers(input)

		// Should skip invalid player
		if len(players) != 0 {
			t.Errorf("expected 0 players (invalid format), got %d", len(players))
		}
	})
}
