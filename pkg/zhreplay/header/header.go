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
	return &GeneralsHeader{
		GameType:        bp.ReadString(8),
		TimeStampBegin:  bp.ReadUInt32(),
		TimeStampEnd:    bp.ReadUInt32(),
		NumTimeStamps:   bp.ReadUInt16(),
		Filler:          fmt.Sprintf("%x", bp.ReadBytes(21)),
		FileName:        bp.ReadNullTermString("utf16"),
		Year:            bp.ReadUInt16(),
		Month:           bp.ReadUInt16(),
		DOW:             bp.ReadUInt16(),
		Day:             bp.ReadUInt16(),
		Hour:            bp.ReadUInt16(),
		Minute:          bp.ReadUInt16(),
		Second:          bp.ReadUInt16(),
		Millisecond:     bp.ReadUInt16(),
		Version:         bp.ReadNullTermString("utf16"),
		BuildDate:       bp.ReadNullTermString("utf16"),
		VersionMinor:    bp.ReadUInt16(),
		VersionMajor:    bp.ReadUInt16(),
		Hash:            bp.ReadBytes(13),
		Metadata:        parseMetadata(bp.ReadNullTermString("utf8")),
		ReplayOwnerSlot: fmt.Sprintf("%x", bp.ReadBytes(2)),  // 3000 = slot 0, 3100 = slot 1, etc
		Unknown1:        fmt.Sprintf("%x", bp.ReadBytes(24)), // This seems very close
		// Unknown2:        fmt.Sprintf("%x", bp.ReadBytes(4)), // Changes when playing solo or maybe against computers
		// Unknown3:        fmt.Sprintf("%x", bp.ReadBytes(4)),
		// GameSpeed:       bp.ReadUInt32(),
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
		if len(fields) != 11 {
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
			// TODO: These are still the Generals fields. Fixed num field check, but not content.
		}
		players = append(players, player)
	}

	return players
}
