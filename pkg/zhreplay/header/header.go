package header

import (
	"strings"

	"github.com/bill-rich/cncstats/pkg/bitparse"
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
	MapFile         string   // M
	MapCRC          string   // MC
	MapSize         string   // MS
	Seed            string   // Seed
	C               string   // C
	SR              string   // SR
	StartingCredits string   // SC
	O               string   // O
	Players         []Player // S
}

type Player struct {
	Type             string
	Name             string
	IP               string
	Port             string
	FT               string
	Color            string
	Faction          string
	StartingPosition string
	Team             string
	Unknown          string
}

// GeneralsHeader represents the header structure of a Command & Conquer Generals replay file.
// It contains metadata about the game session, including timestamps, version information,
// and player details.
type GeneralsHeader struct {
	GameType        string
	TimeStampBegin  int
	TimeStampEnd    int
	NumTimeStamps   int
	Filler          [12]byte
	FileName        string
	Year            int
	Month           int
	DOW             int
	Day             int
	Hour            int
	Minute          int
	Second          int
	Millisecond     int
	Version         string
	BuildDate       string
	VersionMinor    int
	VersionMajor    int
	Hash            [8]byte
	Metadata        Metadata
	ReplayOwnerSlot [2]byte
	Unknown1        [4]byte
	Unknown2        [4]byte
	Unknown3        [4]byte
	GameSpeed       int
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
// It handles errors gracefully by providing fallback values and logging issues.
func NewHeader(bp *bitparse.BitParser) *GeneralsHeader {
	// Read all fields with error handling using the helper functions
	gameType := readStringWithFallback(bp, func() (string, error) { return bp.ReadString(6) }, "", "GameType")
	timeStampBegin := readIntWithFallback(bp, bp.ReadUInt32, 0, "TimeStampBegin")
	timeStampEnd := readIntWithFallback(bp, bp.ReadUInt32, 0, "TimeStampEnd")
	numTimeStamps := readIntWithFallback(bp, bp.ReadUInt16, 0, "NumTimeStamps")

	fillerBytes := readBytesWithFallback(bp, func() ([]byte, error) { return bp.ReadBytes(12) }, make([]byte, 12), "Filler")
	var filler [12]byte
	copy(filler[:], fillerBytes)

	fileName := readStringWithFallback(bp, func() (string, error) { return bp.ReadNullTermString("utf16") }, "", "FileName")
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
	versionMinor := readIntWithFallback(bp, bp.ReadUInt16, 0, "VersionMinor")
	versionMajor := readIntWithFallback(bp, bp.ReadUInt16, 0, "VersionMajor")

	hashBytes := readBytesWithFallback(bp, func() ([]byte, error) { return bp.ReadBytes(8) }, make([]byte, 8), "Hash")
	var hash [8]byte
	copy(hash[:], hashBytes)

	metadataStr := readStringWithFallback(bp, func() (string, error) { return bp.ReadNullTermString("utf8") }, "", "Metadata")

	replayOwnerSlotBytes := readBytesWithFallback(bp, func() ([]byte, error) { return bp.ReadBytes(2) }, make([]byte, 2), "ReplayOwnerSlot")
	var replayOwnerSlot [2]byte
	copy(replayOwnerSlot[:], replayOwnerSlotBytes)

	unknown1Bytes := readBytesWithFallback(bp, func() ([]byte, error) { return bp.ReadBytes(4) }, make([]byte, 4), "Unknown1")
	var unknown1 [4]byte
	copy(unknown1[:], unknown1Bytes)

	unknown2Bytes := readBytesWithFallback(bp, func() ([]byte, error) { return bp.ReadBytes(4) }, make([]byte, 4), "Unknown2")
	var unknown2 [4]byte
	copy(unknown2[:], unknown2Bytes)

	unknown3Bytes := readBytesWithFallback(bp, func() ([]byte, error) { return bp.ReadBytes(4) }, make([]byte, 4), "Unknown3")
	var unknown3 [4]byte
	copy(unknown3[:], unknown3Bytes)

	gameSpeed := readIntWithFallback(bp, bp.ReadUInt32, 0, "GameSpeed")

	// Validate parsed values
	if year < MinYear || year > MaxYear {
		log.Warnf("unusual year value: %d (expected %d-%d)", year, MinYear, MaxYear)
	}

	return &GeneralsHeader{
		GameType:        gameType,
		TimeStampBegin:  timeStampBegin,
		TimeStampEnd:    timeStampEnd,
		NumTimeStamps:   numTimeStamps,
		Filler:          filler,
		FileName:        fileName,
		Year:            year,
		Month:           month,
		DOW:             dow,
		Day:             day,
		Hour:            hour,
		Minute:          minute,
		Second:          second,
		Millisecond:     millisecond,
		Version:         version,
		BuildDate:       buildDate,
		VersionMinor:    versionMinor,
		VersionMajor:    versionMajor,
		Hash:            hash,
		Metadata:        parseMetadata(metadataStr),
		ReplayOwnerSlot: replayOwnerSlot, // 3000 = slot 0, 3100 = slot 1, etc
		Unknown1:        unknown1,
		Unknown2:        unknown2, // Changes when playing solo or maybe against computers
		Unknown3:        unknown3,
		GameSpeed:       gameSpeed,
	}
}

// parseMetadata parses the metadata string from the replay file header.
// The metadata contains game configuration information including map details,
// player information, and game settings.
func parseMetadata(raw string) Metadata {
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
			metadata.Players = parsePlayers(value)
		default:
			log.Debugf("unknown metadata key: %s", key)
		}
	}
	return *metadata
}

// parsePlayers parses the player information from the metadata string.
// Each player entry contains type, name, network information, and game settings.
func parsePlayers(raw string) []Player {
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
		if len(fields) != PlayerFieldsCount {
			log.Debugf("invalid player format: expected %d fields, got %d", PlayerFieldsCount, len(fields))
			continue
		}

		// Validate that the first field has at least one character for type
		if len(fields[PlayerTypeIndex]) == 0 {
			log.Debugf("empty player type field")
			continue
		}

		playerType := fields[PlayerTypeIndex][0:1]
		playerName := ""
		if len(fields[PlayerTypeIndex]) > PlayerNameStartIndex {
			playerName = fields[PlayerTypeIndex][PlayerNameStartIndex:]
		}

		player := Player{
			Type:             playerType,
			Name:             playerName,
			IP:               fields[1],
			Port:             fields[2],
			FT:               fields[3],
			Color:            fields[4],
			Faction:          fields[5],
			StartingPosition: fields[6],
			Team:             fields[7],
			Unknown:          fields[8],
		}
		players = append(players, player)
	}

	return players
}
