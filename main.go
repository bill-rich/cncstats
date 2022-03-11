package main

import (
	"encoding/json"
	"fmt"
	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
)

type Replay struct {
	Header *header.GeneralsHeader
	Body   []*body.BodyChunk
}

func main() {

	if len(os.Getenv("TRACE")) > 0 {
		log.SetLevel(log.TraceLevel)
	}

	if len(os.Getenv("SERVER")) == 0 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			log.WithError(err).Fatal("could not open file")
		}

		bp := &bitparse.BitParser{
			Source: file,
		}
		um, err := json.Marshal(ReadReplay(bp))
		if err != nil {
			panic(err)
		}
		fmt.Println(um)
		return
	}

	router := gin.Default()
	router.POST("/replay", saveFileHandler)
	router.Run()
}

func ReadReplay(bp *bitparse.BitParser) Replay {
	replay := Replay{
		header.ParseHeader(bp),
		body.ParseBody(bp),
	}
	return replay
}

func saveFileHandler(c *gin.Context) {
	file, err := c.FormFile("file")

	// The file cannot be received.
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "No file is received",
		})
		return
	}

	// The file is received, so let's save it
	if err := c.SaveUploadedFile(file, "/tmp/"+file.Filename); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to save the file",
			"error":   err,
		})
		return
	}

	fileIn, err := os.Open("/tmp/" + file.Filename)
	if err != nil {
		log.WithError(err).Fatal("could not open file")
	}

	bp := &bitparse.BitParser{
		Source: fileIn,
	}

	// File saved successfully. Return proper result
	c.JSON(http.StatusOK, ReadReplay(bp))
}
