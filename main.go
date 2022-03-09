package main

import (
	"encoding/binary"
	log "github.com/sirupsen/logrus"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type GeneralsHeader struct {
	GameType       *ByteString `byte:"size=6"`
	TimeStampBegin *ByteInt    `byte:"size=4"`
	TimeStampEnd   *ByteInt    `byte:"size=4"`
	Unknown        *ByteInt    `byte:"size=1"`
	FileName       *ByteString `byte:"size=2,nullterm"`
}

type ByteInterface interface {
	Write([]byte)
}

type ByteString string

func NewByteString(s string) *ByteString {
	bs := ByteString(s)
	return &bs
}

func (bs *ByteString) Write(b []byte) {
	*bs = ByteString(b)
}

type ByteInt int64

func (bs *ByteInt) Write(b []byte) {
	*bs = ByteInt(binary.BigEndian.Uint32(b))
}

func main() {
	file, err := os.Open(os.Args[1])
	if err != nil {
		log.WithError(err).Fatal("could not open file")
	}
	header := GeneralsHeader{
		GameType: NewByteString(""),
	}
	inst := reflect.ValueOf(&header).Elem()
	gtype := reflect.TypeOf(header)
	for i := 0; i < gtype.NumField(); i++ {
		tagKv := parseTag(gtype.Field(i).Tag.Get("byte"))
		size, err := strconv.Atoi(tagKv["size"])
		if err != nil {
			log.WithError(err).Error("could not parse fieldRaw size")
			continue
		}
		//fieldValue := []byte{}
		_, nullterm := tagKv["nullterm"]
		switch {
		case !nullterm:
			fieldRaw := make([]byte, size)
			sizeRead, err := file.Read(fieldRaw)
			if sizeRead != size {
				log.Errorf("unable to read fieldRaw. expected size: %d, got: %d", size, sizeRead)
			}

			if err != nil {
				log.WithError(err).Error("unable to read fieldRaw")
			}

			field := inst.Field(i).Interface().(ByteInterface)

			field.Write(fieldRaw)

		case nullterm:

		}
	}
	log.Printf("%+v", header)
}

func parseTag(rawTag string) map[string]string {
	out := map[string]string{}
	pairs := strings.Split(rawTag, ",")
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		switch len(kv) {
		case 1:
			out[kv[0]] = "true"
		case 2:
			out[kv[0]] = kv[1]
		default:
			log.WithField("pair", pair).Warn("unexpected tag pair format")
		}
	}
	return out
}
