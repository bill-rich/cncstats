package header

import (
	"fmt"
	"strings"

	"github.com/bill-rich/cncstats/pkg/bitparse"
	log "github.com/sirupsen/logrus"
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

type GeneralsHeader struct {
	GameType        string
	TimeStampBegin  int
	TimeStampEnd    int
	NumTimeStamps   int
	Filler          interface{}
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
	Hash            []byte
	Metadata        Metadata
	ReplayOwnerSlot interface{}
	Unknown1        interface{}
	Unknown2        interface{}
	Unknown3        interface{}
	GameSpeed       int
}

func NewHeader(bp *bitparse.BitParser) *GeneralsHeader {
	// Read all fields with error handling
	gameType, err := bp.ReadString(6)
	if err != nil {
		log.WithError(err).Error("failed to read GameType")
		gameType = ""
	}

	timeStampBegin, err := bp.ReadUInt32()
	if err != nil {
		log.WithError(err).Error("failed to read TimeStampBegin")
		timeStampBegin = 0
	}

	timeStampEnd, err := bp.ReadUInt32()
	if err != nil {
		log.WithError(err).Error("failed to read TimeStampEnd")
		timeStampEnd = 0
	}

	numTimeStamps, err := bp.ReadUInt16()
	if err != nil {
		log.WithError(err).Error("failed to read NumTimeStamps")
		numTimeStamps = 0
	}

	fillerBytes, err := bp.ReadBytes(12)
	if err != nil {
		log.WithError(err).Error("failed to read Filler")
		fillerBytes = make([]byte, 12)
	}

	fileName, err := bp.ReadNullTermString("utf16")
	if err != nil {
		log.WithError(err).Error("failed to read FileName")
		fileName = ""
	}

	year, err := bp.ReadUInt16()
	if err != nil {
		log.WithError(err).Error("failed to read Year")
		year = 0
	}

	month, err := bp.ReadUInt16()
	if err != nil {
		log.WithError(err).Error("failed to read Month")
		month = 0
	}

	dow, err := bp.ReadUInt16()
	if err != nil {
		log.WithError(err).Error("failed to read DOW")
		dow = 0
	}

	day, err := bp.ReadUInt16()
	if err != nil {
		log.WithError(err).Error("failed to read Day")
		day = 0
	}

	hour, err := bp.ReadUInt16()
	if err != nil {
		log.WithError(err).Error("failed to read Hour")
		hour = 0
	}

	minute, err := bp.ReadUInt16()
	if err != nil {
		log.WithError(err).Error("failed to read Minute")
		minute = 0
	}

	second, err := bp.ReadUInt16()
	if err != nil {
		log.WithError(err).Error("failed to read Second")
		second = 0
	}

	millisecond, err := bp.ReadUInt16()
	if err != nil {
		log.WithError(err).Error("failed to read Millisecond")
		millisecond = 0
	}

	version, err := bp.ReadNullTermString("utf16")
	if err != nil {
		log.WithError(err).Error("failed to read Version")
		version = ""
	}

	buildDate, err := bp.ReadNullTermString("utf16")
	if err != nil {
		log.WithError(err).Error("failed to read BuildDate")
		buildDate = ""
	}

	versionMinor, err := bp.ReadUInt16()
	if err != nil {
		log.WithError(err).Error("failed to read VersionMinor")
		versionMinor = 0
	}

	versionMajor, err := bp.ReadUInt16()
	if err != nil {
		log.WithError(err).Error("failed to read VersionMajor")
		versionMajor = 0
	}

	hash, err := bp.ReadBytes(8)
	if err != nil {
		log.WithError(err).Error("failed to read Hash")
		hash = make([]byte, 8)
	}

	metadataStr, err := bp.ReadNullTermString("utf8")
	if err != nil {
		log.WithError(err).Error("failed to read Metadata")
		metadataStr = ""
	}

	replayOwnerSlotBytes, err := bp.ReadBytes(2)
	if err != nil {
		log.WithError(err).Error("failed to read ReplayOwnerSlot")
		replayOwnerSlotBytes = make([]byte, 2)
	}

	unknown1Bytes, err := bp.ReadBytes(4)
	if err != nil {
		log.WithError(err).Error("failed to read Unknown1")
		unknown1Bytes = make([]byte, 4)
	}

	unknown2Bytes, err := bp.ReadBytes(4)
	if err != nil {
		log.WithError(err).Error("failed to read Unknown2")
		unknown2Bytes = make([]byte, 4)
	}

	unknown3Bytes, err := bp.ReadBytes(4)
	if err != nil {
		log.WithError(err).Error("failed to read Unknown3")
		unknown3Bytes = make([]byte, 4)
	}

	gameSpeed, err := bp.ReadUInt32()
	if err != nil {
		log.WithError(err).Error("failed to read GameSpeed")
		gameSpeed = 0
	}

	return &GeneralsHeader{
		GameType:        gameType,
		TimeStampBegin:  timeStampBegin,
		TimeStampEnd:    timeStampEnd,
		NumTimeStamps:   numTimeStamps,
		Filler:          fmt.Sprintf("%x", fillerBytes),
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
		ReplayOwnerSlot: fmt.Sprintf("%x", replayOwnerSlotBytes), // 3000 = slot 0, 3100 = slot 1, etc
		Unknown1:        fmt.Sprintf("%x", unknown1Bytes),
		Unknown2:        fmt.Sprintf("%x", unknown2Bytes), // Changes when playing solo or maybe against computers
		Unknown3:        fmt.Sprintf("%x", unknown3Bytes),
		GameSpeed:       gameSpeed,
	}
}

func parseMetadata(raw string) Metadata {
	metadata := &Metadata{}
	fields := strings.Split(raw, ";")
	for _, field := range fields {
		fieldSplit := strings.Split(field, "=")
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
		}
	}
	return *metadata
}

func parsePlayers(raw string) []Player {
	players := []Player{}
	playersRaw := strings.Split(raw, ":")
	for _, playerRaw := range playersRaw {
		fields := strings.Split(playerRaw, ",")
		if len(fields) != 9 {
			continue
		}
		playerType := []byte(fields[0])[0]
		player := Player{
			Type:             string(playerType),
			Name:             string([]byte(fields[0])[1:]),
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
