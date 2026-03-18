package header

import (
	"strconv"
	"strings"

	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/iniparse"
	log "github.com/sirupsen/logrus"
)

// Constants for parsing and validation
const (
	PlayerFieldsCount    = 9
	PlayerTypeIndex      = 0
	PlayerNameStartIndex = 1
	MinYear              = 1990
	MaxYear              = 2030
	MaxPlayers           = 8
)

type Metadata struct {
	MapFile         string   `json:"mapFile"`         // M
	MapCRC          string   `json:"mapCRC"`          // MC
	MapSize         string   `json:"mapSize"`         // MS
	Seed            string   `json:"seed"`            // Seed
	C               string   `json:"c"`               // C
	SR              string   `json:"sr"`              // SR
	StartingCredits string   `json:"startingCredits"` // SC
	O               string   `json:"o"`               // O
	Players         []Player `json:"players"`         // S
}

type Player struct {
	Type             string `json:"type"`
	Name             string `json:"name"`
	IP               string `json:"ip"`
	Port             string `json:"port"`
	FT               string `json:"ft"`
	Color            string `json:"color"`
	Faction          string `json:"faction"`
	StartingPosition string `json:"startingPosition"`
	Team             string `json:"team"`
	Unknown          string `json:"unknown"`
}

// GetColorName converts the Player.Color string (which should be an int) to the actual color name
// using the provided ColorStore. Returns the color name or the original string if conversion fails.
func (p *Player) GetColorName(colorStore *iniparse.ColorStore) string {
	if colorStore == nil {
		return p.Color
	}

	// Convert the color string to an integer
	colorID, err := strconv.Atoi(p.Color)
	if err != nil {
		log.Debugf("failed to convert color string '%s' to int: %v", p.Color, err)
		return p.Color
	}

	// Get the color name from the ColorStore
	colorName, err := colorStore.GetColorName(colorID)
	if err != nil {
		log.Debugf("failed to get color name for ID %d: %v", colorID, err)
		return p.Color
	}

	return colorName
}

// GeneralsHeader represents the header structure of a Command & Conquer Generals replay file.
// Field order and sizes match Recorder.cpp startRecording() / readReplayHeader().
type GeneralsHeader struct {
	GameType         string   `json:"gameType"`         // 6 bytes: "GENREP"
	TimeStampBegin   int      `json:"timeStampBegin"`   // int32 (replay_time_t)
	TimeStampEnd     int      `json:"timeStampEnd"`     // int32 (replay_time_t)
	FrameCount       int      `json:"frameCount"`       // uint32
	Desync           bool     `json:"desync"`           // 1 byte Bool
	QuitEarly        bool     `json:"quitEarly"`        // 1 byte Bool
	PlayerDiscons    [8]bool  `json:"playerDiscons"`    // 8 × 1 byte Bool (MAX_SLOTS)
	ReplayName       string   `json:"replayName"`       // UTF-16 null-terminated
	Year             int      `json:"year"`             // SYSTEMTIME fields (8 × uint16)
	Month            int      `json:"month"`
	DOW              int      `json:"dow"`
	Day              int      `json:"day"`
	Hour             int      `json:"hour"`
	Minute           int      `json:"minute"`
	Second           int      `json:"second"`
	Millisecond      int      `json:"millisecond"`
	Version          string   `json:"version"`          // UTF-16 null-terminated
	BuildDate        string   `json:"buildDate"`        // UTF-16 null-terminated (versionTimeString)
	VersionNumber    int      `json:"versionNumber"`    // uint32
	ExeCRC           int      `json:"exeCRC"`           // uint32
	IniCRC           int      `json:"iniCRC"`           // uint32
	Metadata         Metadata `json:"metadata"`         // ASCII null-terminated (gameOptions)
	LocalPlayerIndex int      `json:"localPlayerIndex"` // ASCII null-terminated int
	Difficulty       int      `json:"difficulty"`       // int32 (GameDifficulty enum)
	OriginalGameMode int      `json:"originalGameMode"` // int32
	RankPoints       int      `json:"rankPoints"`       // int32
	MaxFPS           int      `json:"maxFPS"`           // int32
}

// Helper functions for reading values with fallback error handling

func readStringWithFallback(bp *bitparse.BitParser, reader func() (string, error), defaultValue string, fieldName string) string {
	value, err := reader()
	if err != nil {
		log.WithError(err).Errorf("failed to read %s", fieldName)
		return defaultValue
	}
	return value
}

func readIntWithFallback(bp *bitparse.BitParser, reader func() (int, error), defaultValue int, fieldName string) int {
	value, err := reader()
	if err != nil {
		log.WithError(err).Errorf("failed to read %s", fieldName)
		return defaultValue
	}
	return value
}

func readBytesWithFallback(bp *bitparse.BitParser, reader func() ([]byte, error), defaultValue []byte, fieldName string) []byte {
	value, err := reader()
	if err != nil {
		log.WithError(err).Errorf("failed to read %s", fieldName)
		return defaultValue
	}
	return value
}

// NewHeader parses a Command & Conquer Generals replay file header from the provided BitParser.
// Layout matches Recorder.cpp startRecording() / readReplayHeader().
func NewHeader(bp *bitparse.BitParser) *GeneralsHeader {
	gameType := readStringWithFallback(bp, func() (string, error) { return bp.ReadString(6) }, "", "GameType")
	timeStampBegin := readIntWithFallback(bp, bp.ReadUInt32, 0, "TimeStampBegin")
	timeStampEnd := readIntWithFallback(bp, bp.ReadUInt32, 0, "TimeStampEnd")
	frameCount := readIntWithFallback(bp, bp.ReadUInt32, 0, "FrameCount")

	desyncBytes := readBytesWithFallback(bp, func() ([]byte, error) { return bp.ReadBytes(1) }, make([]byte, 1), "Desync")
	quitEarlyBytes := readBytesWithFallback(bp, func() ([]byte, error) { return bp.ReadBytes(1) }, make([]byte, 1), "QuitEarly")
	playerDisconsBytes := readBytesWithFallback(bp, func() ([]byte, error) { return bp.ReadBytes(8) }, make([]byte, 8), "PlayerDiscons")
	var playerDiscons [8]bool
	for i := 0; i < 8; i++ {
		playerDiscons[i] = playerDisconsBytes[i] != 0
	}

	replayName := readStringWithFallback(bp, func() (string, error) { return bp.ReadNullTermString("utf16") }, "", "ReplayName")
	year := readIntWithFallback(bp, bp.ReadUInt16, 0, "Year")
	month := readIntWithFallback(bp, bp.ReadUInt16, 0, "Month")
	dow := readIntWithFallback(bp, bp.ReadUInt16, 0, "DOW")
	day := readIntWithFallback(bp, bp.ReadUInt16, 0, "Day")
	hour := readIntWithFallback(bp, bp.ReadUInt16, 0, "Hour")
	minute := readIntWithFallback(bp, bp.ReadUInt16, 0, "Minute")
	second := readIntWithFallback(bp, bp.ReadUInt16, 0, "Second")
	millisecond := readIntWithFallback(bp, bp.ReadUInt16, 0, "Millisecond")

	version := readStringWithFallback(bp, func() (string, error) { return bp.ReadNullTermString("utf16") }, "", "Version")
	buildDate := readStringWithFallback(bp, func() (string, error) { return bp.ReadNullTermString("utf16") }, "", "BuildDate")
	versionNumber := readIntWithFallback(bp, bp.ReadUInt32, 0, "VersionNumber")
	exeCRC := readIntWithFallback(bp, bp.ReadUInt32, 0, "ExeCRC")
	iniCRC := readIntWithFallback(bp, bp.ReadUInt32, 0, "IniCRC")

	metadataStr := readStringWithFallback(bp, func() (string, error) { return bp.ReadNullTermString("utf8") }, "", "Metadata")

	localPlayerIndexStr := readStringWithFallback(bp, func() (string, error) { return bp.ReadNullTermString("utf8") }, "0", "LocalPlayerIndex")
	localPlayerIndex, err := strconv.Atoi(localPlayerIndexStr)
	if err != nil {
		log.WithError(err).Warn("failed to parse LocalPlayerIndex, defaulting to 0")
		localPlayerIndex = 0
	}

	difficulty := readIntWithFallback(bp, bp.ReadUInt32, 0, "Difficulty")
	originalGameMode := readIntWithFallback(bp, bp.ReadUInt32, 0, "OriginalGameMode")
	rankPoints := readIntWithFallback(bp, bp.ReadUInt32, 0, "RankPoints")
	maxFPS := readIntWithFallback(bp, bp.ReadUInt32, 0, "MaxFPS")

	if year < MinYear || year > MaxYear {
		log.Warnf("unusual year value: %d (expected %d-%d)", year, MinYear, MaxYear)
	}

	return &GeneralsHeader{
		GameType:         gameType,
		TimeStampBegin:   timeStampBegin,
		TimeStampEnd:     timeStampEnd,
		FrameCount:       frameCount,
		Desync:           desyncBytes[0] != 0,
		QuitEarly:        quitEarlyBytes[0] != 0,
		PlayerDiscons:    playerDiscons,
		ReplayName:       replayName,
		Year:             year,
		Month:            month,
		DOW:              dow,
		Day:              day,
		Hour:             hour,
		Minute:           minute,
		Second:           second,
		Millisecond:      millisecond,
		Version:          version,
		BuildDate:        buildDate,
		VersionNumber:    versionNumber,
		ExeCRC:           exeCRC,
		IniCRC:           iniCRC,
		Metadata:         parseMetadata(metadataStr, bp.ColorStore),
		LocalPlayerIndex: localPlayerIndex,
		Difficulty:       difficulty,
		OriginalGameMode: originalGameMode,
		RankPoints:       rankPoints,
		MaxFPS:           maxFPS,
	}
}

// parseMetadata parses the metadata string from the replay file header.
// The metadata contains game configuration information including map details,
// player information, and game settings.
func parseMetadata(raw string, colorStore *iniparse.ColorStore) Metadata {
	metadata := &Metadata{}
	if raw == "" {
		return *metadata
	}

	// Use SplitN for better performance with large strings
	fields := strings.Split(raw, ";")
	for _, field := range fields {
		if field == "" {
			continue
		}

		// Use SplitN to limit splits to 2 parts for better performance
		fieldSplit := strings.SplitN(field, "=", 2)
		if len(fieldSplit) != 2 {
			log.Debugf("error splitting metadata field: %s", field)
			continue
		}

		key := fieldSplit[0]
		value := fieldSplit[1]

		switch key {
		case "M":
			metadata.MapFile = value
		case "MC":
			metadata.MapCRC = value
		case "MS":
			metadata.MapSize = value
		case "SD":
			metadata.Seed = value
		case "C":
			metadata.C = value
		case "SR":
			metadata.SR = value
		case "SC":
			metadata.StartingCredits = value
		case "O":
			metadata.O = value
		case "S":
			metadata.Players = parsePlayers(value, colorStore)
		default:
			log.Debugf("unknown metadata key: %s", key)
		}
	}
	return *metadata
}

// convertColorString converts a color string (int as string) to a color name using ColorStore
func convertColorString(colorStr string, colorStore *iniparse.ColorStore) string {
	if colorStore == nil {
		return colorStr
	}

	// Convert the color string to an integer
	colorID, err := strconv.Atoi(colorStr)
	if err != nil {
		log.Debugf("failed to convert color string '%s' to int: %v", colorStr, err)
		return colorStr
	}

	// Get the color name from the ColorStore
	colorName, err := colorStore.GetColorName(colorID)
	if err != nil {
		log.Debugf("failed to get color name for ID %d: %v", colorID, err)
		return colorStr
	}

	return colorName
}

// parsePlayers parses the player information from the metadata string.
// Each player entry contains type, name, network information, and game settings.
func parsePlayers(raw string, colorStore *iniparse.ColorStore) []Player {
	if raw == "" {
		return []Player{}
	}

	playersRaw := strings.Split(raw, ":")
	// Pre-allocate slice with expected capacity for better performance
	players := make([]Player, 0, len(playersRaw))

	for _, playerRaw := range playersRaw {
		if playerRaw == "" {
			continue
		}

		fields := strings.Split(playerRaw, ",")

		// Validate that the first field has at least one character for type
		if len(fields[PlayerTypeIndex]) == 0 {
			log.Debugf("empty player type field")
			continue
		}

		playerType := fields[PlayerTypeIndex][0:1]

		var player Player

		if playerType == "H" {
			// Human player - use original format
			if len(fields) != PlayerFieldsCount {
				log.Debugf("invalid human player format: expected %d fields, got %d", PlayerFieldsCount, len(fields))
				continue
			}

			playerName := ""
			if len(fields[PlayerTypeIndex]) > PlayerNameStartIndex {
				playerName = fields[PlayerTypeIndex][PlayerNameStartIndex:]
			}

			player = Player{
				Type:             playerType,
				Name:             playerName,
				IP:               fields[1],
				Port:             fields[2],
				FT:               fields[3],
				Color:            convertColorString(fields[4], colorStore),
				Faction:          fields[5],
				StartingPosition: fields[6],
				Team:             fields[7],
				Unknown:          fields[8],
			}
		} else if playerType == "C" {
			// Computer player - use new format: "C<difficulty>,<color>,<faction>,<startPos>,<teamNumber>"
			if len(fields) != 5 {
				log.Debugf("invalid computer player format: expected 5 fields, got %d", len(fields))
				continue
			}

			// Extract difficulty from the first field (after "C")
			difficulty := ""
			if len(fields[PlayerTypeIndex]) > 1 {
				difficulty = fields[PlayerTypeIndex][1:]
			}

			player = Player{
				Type:             playerType,
				Name:             "",         // Computer players don't have names
				IP:               "",         // Computer players don't have IPs
				Port:             "",         // Computer players don't have ports
				FT:               difficulty, // Store difficulty in FT field
				Color:            convertColorString(fields[1], colorStore),
				Faction:          fields[2],
				StartingPosition: fields[3],
				Team:             fields[4],
				Unknown:          "", // Computer players don't have unknown field
			}
		} else {
			// Unknown player type, skip
			log.Debugf("unknown player type: %s", playerType)
			continue
		}

		players = append(players, player)
	}

	return players
}
